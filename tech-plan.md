# VS Code Tunnel Monitoring Rewrite - Technical Plan

## Overview

Replace the current tunnel monitoring in `server/podman_tunnel.go` with a reliable, session-based approach that addresses the issues documented in `issues.md`.

---

## Key Changes

### 1. Session-Based Logging

Each tunnel invocation gets a unique session ID (UUID or timestamp-based).

```
Log file:   /tmp/pocketpod-tunnel-{sessionID}.log
PID file:   /tmp/pocketpod-tunnel-{sessionID}.pid
Label:      pocketpod.tunnel_session={sessionID}
```

When parsing logs, only consider lines matching the current session. Eliminates stale log issues on container restart or server restart reconciliation.

### 2. PID File for Process Health

Tunnel start command writes its PID:

```bash
code tunnel --name $NAME ... &
echo $! > /tmp/pocketpod-tunnel-{sessionID}.pid
wait
```

Monitor checks process health:

```bash
kill -0 $(cat /tmp/pocketpod-tunnel-{sessionID}.pid 2>/dev/null) 2>/dev/null && echo alive || echo dead
```

Benefits:

- O(1) check, no process list parsing
- Specific to the exact tunnel process
- Works regardless of available tools in container

### 3. Non-Terminal Auth State

`auth_required` is NOT a terminal state. Monitor continues watching for:

- Token file creation (user completed GitHub device auth)
- Process death (tunnel failed)

State machine:

```
initializing → installing → starting → auth_required ←→ ready
                                              ↓
                                           failed
```

### 4. Progress-Based Timeout

Track `lastProgress` timestamp. Update on any meaningful state change:

- CLI install completes
- Tunnel process starts
- Auth prompt detected
- Token file appears
- Process dies

Fail only if NO progress for N seconds (default: 120s), not a fixed attempt count.

### 5. Dedicated Monitor Goroutines

```go
type tunnelMonitor struct {
    containerID  string
    sessionID    string
    state        string
    lastProgress time.Time
    stopCh       chan struct{}
}

type podmanService struct {
    monitors map[string]*tunnelMonitor
    mu       sync.RWMutex
    // existing fields...
}
```

Each container gets a long-running monitor goroutine that:

- Polls at fixed interval (e.g., 3s)
- Updates state in `podmanService.tunnelStateByContainerID`
- Exits when container is deleted or `stopCh` is closed

### 6. Server Restart Reconciliation

On server startup, scan existing containers with `pocketpod.tunnel_session` label:

- Read PID file to check if tunnel is alive
- Read log file to detect auth state
- Resume monitoring if tunnel is still running

---

## File Changes

### `server/podman_tunnel.go`

Rewrite entirely with new structures and logic.

#### New Types

```go
type tunnelMonitor struct {
    containerID  string
    sessionID    string
    state        string
    lastProgress time.Time
    stopCh       chan struct{}
}

type tunnelHealth struct {
    processAlive bool
    tokenPresent bool
    authRequired bool
    deviceCode   string
}
```

#### New Constants

```go
const (
    labelTunnelSession = "pocketpod.tunnel_session"
    tunnelPIDFile      = "/tmp/pocketpod-tunnel-%s.pid"
    tunnelLogFile      = "/tmp/pocketpod-tunnel-%s.log"
    tunnelProgressTimeout = 120 * time.Second
    tunnelPollInterval    = 3 * time.Second
)
```

#### New Functions

| Function                                                        | Purpose                          |
| --------------------------------------------------------------- | -------------------------------- |
| `generateSessionID() string`                                    | Create unique session identifier |
| `startTunnelMonitor(containerID, sessionID string)`             | Spawn monitor goroutine          |
| `checkTunnelHealth(containerID, sessionID string) tunnelHealth` | PID + token + log checks         |
| `readSessionLog(containerID, sessionID string) (string, error)` | Read session-scoped log          |
| `reconcileTunnelSessions()`                                     | Called on server startup         |
| `stopTunnelMonitor(containerID string)`                         | Signal monitor to stop           |

#### Modified Functions

| Function                    | Changes                                            |
| --------------------------- | -------------------------------------------------- |
| `bootstrapTunnel()`         | Generate session ID, use new paths, store label    |
| `buildTunnelStartCommand()` | Include PID file writing, session ID in log prefix |
| `monitorTunnelState()`      | Rewrite with non-terminal auth, progress timeout   |

### `server/podman_workspace.go`

- Update `createWorkspace()` to:
  - Generate and pass session ID to `bootstrapTunnel()`
  - Apply `pocketpod.tunnel_session` label to container
- Ensure VS Code volume path is passed to monitor for token checks

### `server/podman.go`

- Add `monitors map[string]*tunnelMonitor` to `podmanService`
- Call `reconcileTunnelSessions()` during service initialization
- Ensure monitors are stopped on container delete

### `server/podman_actions.go`

- Update `deleteContainer()` to call `stopTunnelMonitor()` before clearing state

---

## Implementation Steps

### Step 1: Define new types and constants

Add to `podman_tunnel.go`:

- `tunnelMonitor` struct
- `tunnelHealth` struct
- New constants for paths, labels, timeouts

### Step 2: Implement session ID generation

```go
func generateSessionID() string {
    return fmt.Sprintf("%d-%s", time.Now().Unix(), strings.Split(uuid.New().String(), "-")[0])
}
```

### Step 3: Rewrite tunnel start command with PID file

Update `buildTunnelStartCommand()`:

```go
func buildTunnelStartCommand(sessionID, tunnelName, homeDir string) string {
    pidFile := fmt.Sprintf(tunnelPIDFile, sessionID)
    logFile := fmt.Sprintf(tunnelLogFile, sessionID)
    // ...
    // code tunnel ... &
    // echo $! > pidFile
    // wait
}
```

### Step 4: Implement health check function

```go
func checkTunnelHealth(containerID, sessionID string) tunnelHealth {
    // 1. Check PID file + kill -0
    // 2. Check token file exists
    // 3. Parse log for auth prompt + device code
}
```

### Step 5: Rewrite monitor goroutine

```go
func (s *podmanService) startTunnelMonitor(containerID, sessionID, hostVSCodeDir string) {
    m := &tunnelMonitor{
        containerID:  containerID,
        sessionID:    sessionID,
        state:        tunnelStatusStarting,
        lastProgress: time.Now(),
        stopCh:       make(chan struct{}),
    }

    s.mu.Lock()
    s.monitors[containerID] = m
    s.mu.Unlock()

    go m.run(s, hostVSCodeDir)
}

func (m *tunnelMonitor) run(s *podmanService, hostVSCodeDir string) {
    ticker := time.NewTicker(tunnelPollInterval)
    defer ticker.Stop()

    for {
        select {
        case <-m.stopCh:
            return
        case <-ticker.C:
            health := checkTunnelHealth(m.containerID, m.sessionID)
            newState := m.evaluateHealth(health, hostVSCodeDir)

            if newState != m.state {
                m.lastProgress = time.Now()
                m.state = newState
                s.setTunnelState(m.containerID, buildTunnelState(newState, health))
                s.schedulePoll(podmanPollDebounce)
            }

            if time.Since(m.lastProgress) > tunnelProgressTimeout {
                s.setTunnelState(m.containerID, podmanTunnelState{
                    Status:  tunnelStatusFailed,
                    Message: "Tunnel bootstrap timed out.",
                })
                s.schedulePoll(podmanPollDebounce)
                return
            }
        }
    }
}
```

### Step 6: Implement reconciliation on startup

```go
func (s *podmanService) reconcileTunnelSessions() {
    containers, _ := s.listContainers() // or equivalent
    for _, c := range containers {
        sessionID := c.Labels[labelTunnelSession]
        if sessionID == "" {
            continue
        }
        health := checkTunnelHealth(c.ID, sessionID)
        if health.processAlive {
            s.startTunnelMonitor(c.ID, sessionID, deriveHostVSCodeDir(c))
        }
    }
}
```

### Step 7: Update workspace creation

In `createWorkspace()`:

```go
sessionID := generateSessionID()
// add label to container args
args = append(args, "--label", fmt.Sprintf("%s=%s", labelTunnelSession, sessionID))
// ...
tunnelState := s.bootstrapTunnel(containerID, name, sessionID)
s.startTunnelMonitor(containerID, sessionID, volumeHostPath)
```

### Step 8: Cleanup on container delete

In `deleteContainer()`:

```go
s.stopTunnelMonitor(containerID)
s.clearTunnelState(containerID)
```

### Step 9: Remove old monitoring code

Delete or replace:

- Old `monitorTunnelState()` implementation
- Old `isTunnelProcessRunning()` with pgrep/ps parsing
- Old log parsing without session scoping

---

## Testing Checklist

1. **Fresh container creation**
   - Tunnel starts with new session ID
   - PID file created
   - Monitor tracks state correctly

2. **Auth required flow**
   - Device code detected from log
   - State set to `auth_required`
   - Monitor continues running
   - State transitions to `ready` when user completes auth

3. **Container restart**
   - Old session logs ignored
   - New session created
   - No stale state

4. **Server restart**
   - Existing containers reconciled
   - Monitors resume for running tunnels
   - Correct state restored

5. **Container deletion**
   - Monitor stopped
   - State cleaned up

6. **Timeout handling**
   - Fails after 120s with no progress
   - Does NOT fail during slow CLI download
