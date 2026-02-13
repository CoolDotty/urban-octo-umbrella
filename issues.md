# VS Code Tunnel Monitoring Issues

## 1. Terminal state on auth prompt stops monitoring prematurely

Location: `server/podman_tunnel.go:326-333`

When auth is required, the monitor returns `terminal=true` and exits. If the user completes GitHub device auth afterward, there's no mechanism to resume monitoring - the tunnel stays stuck in `blocked`.

## 2. Race between process start and log write

The tunnel starts detached (`-d`), then monitoring begins immediately. On first iteration:

- `isTunnelProcessRunning()` might return false (process not fully started)
- `readTunnelLog()` might return empty (log not written yet)
- Result: stays in `starting` but could miss the auth prompt window

## 3. Stale log content on container restart

The log file is created with `: > tunnelLogPath`, but if a container is restarted without recreation:

- Old log lines persist
- `isLatestTunnelLogLineAuthPrompt()` could detect a stale auth prompt from a previous tunnel instance
- The process check might pass while log state is wrong

## 4. `tunnelRunning` check is unreliable

Location: `server/podman_tunnel.go:431-451`

`isTunnelProcessRunning()` relies on multiple fallbacks (pgrep, ps, /proc). Containers may lack these tools, and the grep patterns:

```
grep -E '[c]ode( |$).*tunnel|[c]ode tunnel'
```

could match unrelated processes or miss the actual tunnel if arg ordering differs.

## 5. Token file check race

`hasVSCodeToken()` checks the bind-mounted host directory, but:

- File writes inside container may not be immediately visible on host (filesystem caching)
- The token could appear after monitoring has already timed out

## 6. Monitor timeout is arbitrary

20 attempts × 3s = 60s max. A slow container or network could exceed this during CLI download/install phase, failing with "Tunnel bootstrap timed out" even though it's still progressing.
