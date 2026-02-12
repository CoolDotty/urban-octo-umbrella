package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEvaluateTunnelStateAuthRequiredExtractsCode(t *testing.T) {
	logOutput := "To sign in, use a web browser to open https://github.com/login/device and enter the code ABCD-EFGH"

	state, terminal := evaluateTunnelState(logOutput, false, nil, false)

	if !terminal {
		t.Fatal("expected auth-required state to be terminal")
	}
	if state.Status != tunnelStatusBlocked {
		t.Fatalf("expected blocked status, got %q", state.Status)
	}
	if state.Code != "ABCD-EFGH" {
		t.Fatalf("expected device code extraction, got %q", state.Code)
	}
	if state.Message != tunnelAuthRequiredMessage {
		t.Fatalf("expected auth-required message, got %q", state.Message)
	}
}

func TestEvaluateTunnelStateReadyWhenTokenPresent(t *testing.T) {
	state, terminal := evaluateTunnelState("", true, nil, true)

	if !terminal {
		t.Fatal("expected token-ready state to be terminal")
	}
	if state.Status != tunnelStatusReady {
		t.Fatalf("expected ready status, got %q", state.Status)
	}
	if state.Code != "" {
		t.Fatalf("expected code cleared on ready state, got %q", state.Code)
	}
}

func TestEvaluateTunnelStateFailureFallback(t *testing.T) {
	state, terminal := evaluateTunnelState("error creating tunnel session", false, nil, false)

	if !terminal {
		t.Fatal("expected failure state to be terminal")
	}
	if state.Status != tunnelStatusFailed {
		t.Fatalf("expected failed status, got %q", state.Status)
	}
	if state.Message == "" {
		t.Fatal("expected failure message")
	}
}

func TestEvaluateTunnelStateStartingWhenNoSignal(t *testing.T) {
	state, terminal := evaluateTunnelState("", false, nil, false)

	if terminal {
		t.Fatal("expected starting state to remain non-terminal")
	}
	if state.Status != tunnelStatusStarting {
		t.Fatalf("expected starting status, got %q", state.Status)
	}
}

func TestEvaluateTunnelStateReadyWhenOpenLinkAppearsAfterAuthPrompt(t *testing.T) {
	logOutput := strings.Join([]string{
		"To grant access to the server, please log into https://github.com/login/device and use code ABCD-EFGH",
		"Open this link in your browser https://vscode.dev/tunnel/cool_ishizaka",
	}, "\n")

	state, terminal := evaluateTunnelState(logOutput, false, nil, true)
	if !terminal {
		t.Fatal("expected terminal state")
	}
	if state.Status != tunnelStatusReady {
		t.Fatalf("expected ready status, got %q", state.Status)
	}
	if state.Code != "" {
		t.Fatalf("expected no code in ready status, got %q", state.Code)
	}
}

func TestEvaluateTunnelStateReadyLineWithoutProcessRemainsStarting(t *testing.T) {
	logOutput := "Open this link in your browser https://vscode.dev/tunnel/cool_ishizaka"
	state, terminal := evaluateTunnelState(logOutput, false, nil, false)
	if terminal {
		t.Fatal("expected non-terminal state when tunnel process is not running")
	}
	if state.Status != tunnelStatusStarting {
		t.Fatalf("expected starting status, got %q", state.Status)
	}
}

func TestEvaluateTunnelStateOpenLinkAfterAuthWithoutRunningRemainsStarting(t *testing.T) {
	logOutput := strings.Join([]string{
		"To grant access to the server, please log into https://github.com/login/device and use code ABCD-EFGH",
		"Open this link in your browser https://vscode.dev/tunnel/cool_ishizaka",
	}, "\n")

	state, terminal := evaluateTunnelState(logOutput, false, nil, false)
	if terminal {
		t.Fatal("expected non-terminal state")
	}
	if state.Status != tunnelStatusStarting {
		t.Fatalf("expected starting status, got %q", state.Status)
	}
	if state.Code != "" {
		t.Fatalf("expected no code in starting state, got %q", state.Code)
	}
}

func TestEvaluateTunnelStateBlockedWhenLatestLineIsAuthPrompt(t *testing.T) {
	logOutput := strings.Join([]string{
		"Open this link in your browser https://vscode.dev/tunnel/cool_ishizaka",
		"To grant access to the server, please log into https://github.com/login/device and use code ABCD-EFGH",
	}, "\n")

	state, terminal := evaluateTunnelState(logOutput, false, nil, true)
	if !terminal {
		t.Fatal("expected terminal state")
	}
	if state.Status != tunnelStatusBlocked {
		t.Fatalf("expected blocked status, got %q", state.Status)
	}
	if state.Code != "ABCD-EFGH" {
		t.Fatalf("expected extracted code, got %q", state.Code)
	}
}

func TestEnrichContainersWithTunnelState(t *testing.T) {
	containers := []podmanContainer{{ID: "abc", Name: "one"}, {ID: "def", Name: "two"}}
	states := map[string]podmanTunnelState{
		"def": {Status: tunnelStatusBlocked, Code: "ZZZZ-9999", Message: tunnelAuthRequiredMessage},
	}

	enrichContainersWithTunnelState(containers, states)

	if containers[0].TunnelStatus != "" {
		t.Fatalf("expected first container to remain without tunnel state, got %q", containers[0].TunnelStatus)
	}
	if containers[1].TunnelStatus != tunnelStatusBlocked {
		t.Fatalf("expected blocked tunnel state, got %q", containers[1].TunnelStatus)
	}
	if containers[1].TunnelCode != "ZZZZ-9999" {
		t.Fatalf("expected tunnel code merge, got %q", containers[1].TunnelCode)
	}
	if containers[1].TunnelURL != "" {
		t.Fatalf("expected blocked tunnel to omit connect URL, got %q", containers[1].TunnelURL)
	}
}

func TestEnrichContainersWithTunnelStateAddsConnectURLWhenNotBlocked(t *testing.T) {
	containers := []podmanContainer{{
		ID:   "abc",
		Name: "my-workspace",
		Labels: map[string]string{
			labelWorkspaceHome: "/home/ubuntu",
			labelWorkspaceDir:  "tiny-stats",
		},
	}}
	states := map[string]podmanTunnelState{
		"abc": {Status: tunnelStatusReady},
	}

	enrichContainersWithTunnelState(containers, states)

	if containers[0].TunnelURL != "https://vscode.dev/tunnel/my-workspace/home/ubuntu/workspaces/tiny-stats" {
		t.Fatalf("expected tunnel URL, got %q", containers[0].TunnelURL)
	}
}

func TestEnrichContainersWithTunnelStateUsesBaseURLWhenWorkspaceLabelsMissing(t *testing.T) {
	containers := []podmanContainer{{ID: "abc", Name: "my-workspace"}}
	states := map[string]podmanTunnelState{
		"abc": {Status: tunnelStatusReady},
	}

	enrichContainersWithTunnelState(containers, states)

	if containers[0].TunnelURL != "https://vscode.dev/tunnel/my-workspace" {
		t.Fatalf("expected base tunnel URL, got %q", containers[0].TunnelURL)
	}
}

func TestEnrichContainersWithTunnelStateOmitsConnectURLWhileStarting(t *testing.T) {
	containers := []podmanContainer{{ID: "abc", Name: "my-workspace"}}
	states := map[string]podmanTunnelState{
		"abc": {Status: tunnelStatusStarting},
	}

	enrichContainersWithTunnelState(containers, states)

	if containers[0].TunnelURL != "" {
		t.Fatalf("expected no tunnel URL while starting, got %q", containers[0].TunnelURL)
	}
}

func TestBuildTunnelStartCommand(t *testing.T) {
	cmd := buildTunnelStartCommand("my-workspace", "/home/dev")
	if !strings.Contains(cmd, "code tunnel --accept-server-license-terms --name 'my-workspace'") {
		t.Fatalf("unexpected command: %s", cmd)
	}
	if !strings.Contains(cmd, tunnelLogPath) {
		t.Fatalf("expected tunnel log path in command: %s", cmd)
	}
	if !strings.Contains(cmd, "HOME='/home/dev'") {
		t.Fatalf("expected HOME assignment in command: %s", cmd)
	}
}

func TestBuildVSCodeInstallCommandValidatesCodeTunnel(t *testing.T) {
	cmd := buildVSCodeInstallCommand()
	if !strings.Contains(cmd, "code tunnel --help") {
		t.Fatalf("expected tunnel validation check in install command: %s", cmd)
	}
}

func TestSelectFirstNonRootUserPrefersRegularHomeUser(t *testing.T) {
	passwd := strings.Join([]string{
		"root:x:0:0:root:/root:/bin/bash",
		"daemon:x:1:1:daemon:/usr/sbin:/usr/sbin/nologin",
		"ubuntu:x:1000:1000:ubuntu:/home/ubuntu:/bin/bash",
	}, "\n")

	name, home, ok := selectFirstNonRootUser(passwd)
	if !ok {
		t.Fatal("expected user to be selected")
	}
	if name != "ubuntu" {
		t.Fatalf("expected ubuntu user, got %q", name)
	}
	if home != "/home/ubuntu" {
		t.Fatalf("expected ubuntu home, got %q", home)
	}
}

func TestSelectFirstNonRootUserFallsBackToFirstNonRoot(t *testing.T) {
	passwd := strings.Join([]string{
		"root:x:0:0:root:/root:/bin/bash",
		"daemon:x:1:1:daemon:/usr/sbin:/usr/sbin/nologin",
		"nobody:x:65534:65534:nobody:/nonexistent:/usr/sbin/nologin",
	}, "\n")

	name, home, ok := selectFirstNonRootUser(passwd)
	if !ok {
		t.Fatal("expected fallback user to be selected")
	}
	if name != "daemon" {
		t.Fatalf("expected daemon fallback user, got %q", name)
	}
	if home != "/usr/sbin" {
		t.Fatalf("expected daemon home fallback, got %q", home)
	}
}

func TestSelectFirstNonRootUserReturnsFalseWhenOnlyRootExists(t *testing.T) {
	name, home, ok := selectFirstNonRootUser("root:x:0:0:root:/root:/bin/bash")
	if ok {
		t.Fatalf("expected no user selection, got %q %q", name, home)
	}
}

func TestBuildTunnelLogPrepareCommandIncludesChownForExecUser(t *testing.T) {
	cmd := buildTunnelLogPrepareCommand("ubuntu")
	if !strings.Contains(cmd, "chown 'ubuntu' "+tunnelLogPath) {
		t.Fatalf("expected chown to tunnel log path: %s", cmd)
	}
	if !strings.Contains(cmd, tunnelBootstrapLogPath) {
		t.Fatalf("expected bootstrap log path in command: %s", cmd)
	}
}

func TestCreateWorkspaceResponseTunnelJSON(t *testing.T) {
	payload := createWorkspaceResponse{
		Name:    "ws-one",
		Status:  "Running",
		RepoURL: "https://github.com/org/repo.git",
		Tunnel: workspaceTunnelSnapshot{
			Status: tunnelStatusStarting,
		},
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}

	decoded := map[string]any{}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	tunnelValue, ok := decoded["tunnel"].(map[string]any)
	if !ok {
		t.Fatal("expected tunnel object in create response")
	}
	if tunnelValue["status"] != tunnelStatusStarting {
		t.Fatalf("expected tunnel status %q, got %#v", tunnelStatusStarting, tunnelValue["status"])
	}
}
