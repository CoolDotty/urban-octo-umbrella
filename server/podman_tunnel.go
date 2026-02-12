package main

import (
	"errors"
	"fmt"
	"io/fs"
	neturl "net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	tunnelStatusReady    = "ready"
	tunnelStatusStarting = "starting"
	tunnelStatusBlocked  = "blocked"
	tunnelStatusFailed   = "failed"

	tunnelAuthRequiredMessage = "Authentication required"
	tunnelAuthURL             = "https://github.com/login/device"
	tunnelLogPath             = "/tmp/pocketpod-vscode-tunnel.log"
	tunnelBootstrapLogPath    = "/tmp/pocketpod-vscode-bootstrap.log"

	tunnelMonitorMaxAttempts = 20
	tunnelMonitorInterval    = 3 * time.Second
)

var (
	deviceCodePattern     = regexp.MustCompile(`(?i)\b(?:enter\s+(?:the\s+)?)?(?:device\s*code|code)\b[^A-Z0-9-]*([A-Z0-9]{4}(?:-[A-Z0-9]{4})+)`)
	invalidTunnelName     = regexp.MustCompile(`[^a-zA-Z0-9_.-]`)
	authPromptLinePattern = regexp.MustCompile(`^To grant access to the server, please log into https://github\.com/login/device and use code [A-Za-z0-9-]+$`)
)

type podmanTunnelState struct {
	Status  string
	Code    string
	Message string
	Debug   *workspaceTunnelDebug
}

type workspaceTunnelSnapshot struct {
	Status  string                `json:"status"`
	Code    string                `json:"code,omitempty"`
	Message string                `json:"message,omitempty"`
	Debug   *workspaceTunnelDebug `json:"debug,omitempty"`
}

type workspaceTunnelDebug struct {
	Version       string `json:"version"`
	ExecUser      string `json:"execUser,omitempty"`
	InstallCmd    string `json:"installCmd,omitempty"`
	StartCmd      string `json:"startCmd,omitempty"`
	InstallOutput string `json:"installOutput,omitempty"`
	StartOutput   string `json:"startOutput,omitempty"`
}

func (s *podmanService) bootstrapTunnel(containerID string, workspaceName string) podmanTunnelState {
	execUser, err := resolveFirstNonRootUser(containerID)
	if err != nil {
		return podmanTunnelState{
			Status:  tunnelStatusFailed,
			Message: "No non-root user found in container.",
		}
	}

	installCommand := buildVSCodeInstallCommand()
	startCommand := buildTunnelStartCommand(buildTunnelName(workspaceName, containerID), execUser.Home)
	debug := &workspaceTunnelDebug{
		Version:    "tunnel-debug-v1",
		ExecUser:   execUser.Name,
		InstallCmd: installCommand,
		StartCmd:   startCommand,
	}

	if containerID == "" {
		return podmanTunnelState{
			Status:  tunnelStatusFailed,
			Message: "Missing container ID.",
			Debug:   debug,
		}
	}

	_, _ = runPodmanCommand("exec", containerID, "sh", "-lc", buildTunnelLogPrepareCommand(execUser.Name))

	installOutput, installErr := runPodmanCommand(
		"exec",
		containerID,
		"sh",
		"-lc",
		installCommand,
	)
	debug.InstallOutput = firstNonEmptyLine(string(installOutput))
	if installErr != nil {
		return podmanTunnelState{
			Status:  tunnelStatusFailed,
			Message: buildTunnelFailureMessage("Failed to install VS Code CLI", installOutput, installErr),
			Debug:   debug,
		}
	}

	startOutput, startErr := runPodmanCommand("exec", "-d", "--user", execUser.Name, containerID, "sh", "-lc", startCommand)
	debug.StartOutput = firstNonEmptyLine(string(startOutput))
	if startErr != nil {
		return podmanTunnelState{
			Status:  tunnelStatusFailed,
			Message: buildTunnelFailureMessage("Failed to start VS Code tunnel", startOutput, startErr),
			Debug:   debug,
		}
	}

	return podmanTunnelState{
		Status: tunnelStatusStarting,
		Debug:  debug,
	}
}

func buildTunnelLogPrepareCommand(execUser string) string {
	trimmedUser := strings.TrimSpace(execUser)
	if trimmedUser == "" {
		return "mkdir -p /tmp && : > " + tunnelLogPath + " && : > " + tunnelBootstrapLogPath
	}
	return fmt.Sprintf(
		"mkdir -p /tmp && : > %s && : > %s && chown %s %s",
		tunnelLogPath,
		tunnelBootstrapLogPath,
		shellSingleQuote(trimmedUser),
		tunnelLogPath,
	)
}

type tunnelExecUser struct {
	Name string
	Home string
}

func resolveFirstNonRootUser(containerID string) (tunnelExecUser, error) {
	output, err := runPodmanCommand("exec", containerID, "sh", "-lc", "cat /etc/passwd 2>/dev/null || true")
	if err != nil {
		return tunnelExecUser{}, err
	}
	name, home, ok := selectFirstNonRootUser(string(output))
	if !ok {
		return tunnelExecUser{}, errors.New("no non-root user")
	}
	return tunnelExecUser{Name: name, Home: home}, nil
}

func selectFirstNonRootUser(passwdContents string) (string, string, bool) {
	fallbackName := ""
	fallbackHome := ""

	for _, line := range strings.Split(passwdContents, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		parts := strings.Split(trimmed, ":")
		if len(parts) < 7 {
			continue
		}

		name := strings.TrimSpace(parts[0])
		if name == "" || name == "root" {
			continue
		}

		uid, err := strconv.Atoi(strings.TrimSpace(parts[2]))
		if err != nil {
			continue
		}
		home := strings.TrimSpace(parts[5])
		if home == "" {
			home = "/tmp"
		}

		if fallbackName == "" {
			fallbackName = name
			fallbackHome = home
		}

		if uid >= 1000 && strings.HasPrefix(home, "/home/") {
			return name, home, true
		}
	}

	if fallbackName == "" {
		return "", "", false
	}

	return fallbackName, fallbackHome, true
}

func buildVSCodeInstallCommand() string {
	return strings.Join([]string{
		"set -eu",
		fmt.Sprintf("exec >> %s 2>&1", tunnelBootstrapLogPath),
		"echo \"[bootstrap] install started $(date -Iseconds)\"",
		"if command -v code >/dev/null 2>&1; then",
		"  if code tunnel --help >/dev/null 2>&1; then",
		"    echo \"[bootstrap] code already installed and usable: $(command -v code)\"",
		"    exit 0",
		"  fi",
		"  echo \"[bootstrap] code exists but tunnel command is unavailable, reinstalling CLI\"",
		"fi",
		"echo \"[bootstrap] installing prerequisites via apt-get\"",
		"apt-get update >/dev/null",
		"DEBIAN_FRONTEND=noninteractive apt-get install -y ca-certificates curl tar >/dev/null",
		"apt-get clean >/dev/null",
		"arch=$(uname -m)",
		"case \"$arch\" in",
		"  x86_64|amd64) download_url=https://code.visualstudio.com/sha/download?build=stable\\&os=cli-alpine-x64 ;;",
		"  armv7l|armv6l|armhf) download_url=https://code.visualstudio.com/sha/download?build=stable\\&os=cli-linux-armhf ;;",
		"  aarch64|arm64) download_url=https://code.visualstudio.com/sha/download?build=stable\\&os=cli-linux-arm64 ;;",
		"  *) echo \"[bootstrap] unsupported architecture: $arch\"; exit 1 ;;",
		"esac",
		"echo \"[bootstrap] attempting download: $download_url\"",
		"curl -fsSL \"$download_url\" -o /tmp/vscode_cli.tar.gz",
		"tar -xzf /tmp/vscode_cli.tar.gz -C /usr/local/bin code",
		"chmod +x /usr/local/bin/code",
		"rm -f /tmp/vscode_cli.tar.gz",
		"echo \"[bootstrap] install completed $(date -Iseconds)\"",
	}, "\n")
}

func buildTunnelStartCommand(tunnelName string, homeDir string) string {
	safeName := shellSingleQuote(tunnelName)
	home := strings.TrimSpace(homeDir)
	if home == "" {
		home = "/tmp"
	}
	dataDir := strings.TrimRight(home, "/") + "/.vscode"
	safeHome := shellSingleQuote(home)
	safeDataDir := shellSingleQuote(dataDir)
	return strings.Join([]string{
		fmt.Sprintf("echo \"[tunnel] start requested $(date -Iseconds), name=%s\" >> %s", safeName, tunnelLogPath),
		fmt.Sprintf("echo \"[tunnel] starting as user: $(id -un)\" >> %s", tunnelLogPath),
		fmt.Sprintf("echo \"[tunnel] code path: $(command -v code || echo missing)\" >> %s", tunnelLogPath),
		fmt.Sprintf("mkdir -p %s", safeDataDir),
		fmt.Sprintf("HOME=%s VSCODE_CLI_DATA_DIR=%s code tunnel --accept-server-license-terms --name %s >> %s 2>&1", safeHome, safeDataDir, safeName, tunnelLogPath),
		fmt.Sprintf("rc=$?; echo \"[tunnel] process exited with code $rc at $(date -Iseconds)\" >> %s; exit $rc", tunnelLogPath),
	}, "; ")
}

func buildTunnelName(workspaceName string, containerID string) string {
	name := strings.TrimSpace(workspaceName)
	if name == "" {
		name = strings.TrimSpace(containerID)
	}
	if name == "" {
		name = "workspace"
	}
	name = invalidTunnelName.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-.")
	if name == "" {
		return "workspace"
	}
	if len(name) > 128 {
		return name[:128]
	}
	return name
}

func shellSingleQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func buildTunnelFailureMessage(prefix string, output []byte, err error) string {
	message := strings.TrimSpace(string(output))
	if message == "" && err != nil {
		message = strings.TrimSpace(err.Error())
	}
	if message == "" {
		return prefix + "."
	}
	if len(message) > 240 {
		message = message[:240] + "..."
	}
	return fmt.Sprintf("%s: %s", prefix, message)
}

func firstNonEmptyLine(value string) string {
	for _, line := range strings.Split(value, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if len(trimmed) > 240 {
			return trimmed[:240] + "..."
		}
		return trimmed
	}
	return ""
}

func (s *podmanService) monitorTunnelState(containerID string, hostVSCodeDir string) {
	for attempt := 0; attempt < tunnelMonitorMaxAttempts; attempt++ {
		tokenPresent := hasVSCodeToken(hostVSCodeDir)
		logOutput, logErr := readTunnelLog(containerID)
		tunnelRunning, _ := isTunnelProcessRunning(containerID)
		state, terminal := evaluateTunnelState(logOutput, tokenPresent, logErr, tunnelRunning)

		if state.Status != "" && s.setTunnelState(containerID, state) {
			s.schedulePoll(podmanPollDebounce)
		}

		if terminal {
			return
		}

		time.Sleep(tunnelMonitorInterval)
	}

	if s.setTunnelState(containerID, podmanTunnelState{Status: tunnelStatusFailed, Message: "Tunnel bootstrap timed out."}) {
		s.schedulePoll(podmanPollDebounce)
	}
}

func evaluateTunnelState(logOutput string, tokenPresent bool, logErr error, tunnelRunning bool) (podmanTunnelState, bool) {
	if isLatestTunnelLogLineAuthPrompt(logOutput) {
		code := extractDeviceCode(latestNonEmptyLine(logOutput))
		return podmanTunnelState{
			Status:  tunnelStatusBlocked,
			Code:    code,
			Message: tunnelAuthRequiredMessage,
		}, true
	}

	// Runtime process check is the strongest readiness signal once the latest
	// line is no longer an auth prompt.
	if tunnelRunning {
		return podmanTunnelState{Status: tunnelStatusReady}, true
	}

	if state, ok := deriveTunnelStateFromLog(logOutput); ok {
		if state.Status == tunnelStatusBlocked || state.Status == tunnelStatusFailed {
			return state, true
		}
	}

	if tokenPresent && containsTunnelReady(logOutput) {
		return podmanTunnelState{Status: tunnelStatusReady}, true
	}

	if errors.Is(logErr, errPodmanUnavailable) {
		return podmanTunnelState{Status: tunnelStatusFailed, Message: "Podman unavailable while checking tunnel."}, true
	}

	if logErr != nil && isPodmanContainerNotFound([]byte(logErr.Error())) {
		return podmanTunnelState{Status: tunnelStatusFailed, Message: "Container not found while checking tunnel."}, true
	}

	return podmanTunnelState{Status: tunnelStatusStarting}, false
}

func isLatestTunnelLogLineAuthPrompt(logOutput string) bool {
	line := latestNonEmptyLine(logOutput)
	if line == "" {
		return false
	}
	return authPromptLinePattern.MatchString(line)
}

func latestNonEmptyLine(value string) string {
	lines := strings.Split(value, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			return line
		}
	}
	return ""
}

func deriveTunnelStateFromLog(logOutput string) (podmanTunnelState, bool) {
	line := latestNonEmptyLine(logOutput)
	return deriveTunnelStateFromLine(line)
}

func deriveTunnelStateFromLine(line string) (podmanTunnelState, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return podmanTunnelState{}, false
	}

	if code := extractDeviceCode(trimmed); code != "" {
		return podmanTunnelState{
			Status:  tunnelStatusBlocked,
			Code:    code,
			Message: tunnelAuthRequiredMessage,
		}, true
	}

	if containsTunnelAuthRequired(trimmed) {
		return podmanTunnelState{
			Status:  tunnelStatusBlocked,
			Message: tunnelAuthRequiredMessage,
		}, true
	}

	if containsTunnelFailure(trimmed) {
		return podmanTunnelState{
			Status:  tunnelStatusFailed,
			Message: "Tunnel failed to start.",
		}, true
	}

	if containsTunnelReady(trimmed) {
		return podmanTunnelState{Status: tunnelStatusReady}, true
	}

	return podmanTunnelState{}, false
}

func discoverTunnelStateFromContainer(containerID string) (podmanTunnelState, bool) {
	logOutput, err := readTunnelLog(containerID)
	if err != nil {
		return podmanTunnelState{}, false
	}
	tunnelRunning, _ := isTunnelProcessRunning(containerID)
	state, terminal := evaluateTunnelState(logOutput, false, nil, tunnelRunning)
	return state, terminal
}

func isTunnelProcessRunning(containerID string) (bool, error) {
	output, err := runPodmanCommand(
		"exec",
		containerID,
		"sh",
		"-lc",
		"if command -v pgrep >/dev/null 2>&1 && pgrep -fa 'code.*tunnel|code tunnel' >/dev/null 2>&1; then "+
			"echo running; "+
			"elif command -v ps >/dev/null 2>&1 && ps -eo args 2>/dev/null | grep -E '[c]ode( |$).*tunnel|[c]ode tunnel' >/dev/null 2>&1; then "+
			"echo running; "+
			"elif grep -saE 'code(.{0,32})tunnel' /proc/[0-9]*/cmdline >/dev/null 2>&1; then "+
			"echo running; "+
			"else "+
			"echo stopped; "+
			"fi",
	)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(output)) == "running", nil
}

func discoverTunnelStatesForContainers(containers []podmanContainer) map[string]podmanTunnelState {
	discovered := make(map[string]podmanTunnelState)
	for _, container := range containers {
		containerID := strings.TrimSpace(container.ID)
		if containerID == "" {
			continue
		}
		state, ok := discoverTunnelStateFromContainer(containerID)
		if !ok || strings.TrimSpace(state.Status) == "" {
			continue
		}
		discovered[containerID] = state
	}
	return discovered
}

func mergeTunnelStateMap(dst map[string]podmanTunnelState, src map[string]podmanTunnelState) {
	for containerID, state := range src {
		if strings.TrimSpace(containerID) == "" || strings.TrimSpace(state.Status) == "" {
			continue
		}
		dst[containerID] = state
	}
}

func containsTunnelAuthRequired(logOutput string) bool {
	lower := strings.ToLower(logOutput)
	return strings.Contains(lower, "github.com/login/device") ||
		strings.Contains(lower, "authentication") && strings.Contains(lower, "required") ||
		strings.Contains(lower, "device code")
}

func containsTunnelReady(logOutput string) bool {
	lower := strings.ToLower(logOutput)
	return strings.Contains(lower, "connected to tunnel")
}

func containsTunnelFailure(logOutput string) bool {
	lower := strings.ToLower(logOutput)
	return strings.Contains(lower, "command not found") ||
		strings.Contains(lower, "failed to") ||
		strings.Contains(lower, "error creating tunnel")
}

func extractDeviceCode(logOutput string) string {
	matches := deviceCodePattern.FindStringSubmatch(logOutput)
	if len(matches) < 2 {
		return ""
	}
	code := strings.ToUpper(strings.TrimSpace(matches[1]))
	if code == "" {
		return ""
	}
	return code
}

func readTunnelLog(containerID string) (string, error) {
	output, err := runPodmanCommand("exec", containerID, "sh", "-lc", fmt.Sprintf("cat %s 2>/dev/null || true", tunnelLogPath))
	if err != nil {
		return "", err
	}
	return string(output), nil
}

func hasVSCodeToken(hostVSCodeDir string) bool {
	if strings.TrimSpace(hostVSCodeDir) == "" {
		return false
	}

	candidates := []string{
		filepath.Join(hostVSCodeDir, "cli", "token.json"),
		filepath.Join(hostVSCodeDir, "cli", "github", "token.json"),
	}
	for _, candidate := range candidates {
		if fileExists(candidate) {
			return true
		}
	}

	baseDepth := pathDepth(hostVSCodeDir)
	found := false
	_ = filepath.WalkDir(hostVSCodeDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if found {
			return fs.SkipAll
		}
		if entry.IsDir() {
			if pathDepth(path)-baseDepth > 4 {
				return fs.SkipDir
			}
			return nil
		}
		if strings.EqualFold(entry.Name(), "token.json") {
			found = true
			return fs.SkipAll
		}
		return nil
	})

	return found
}

func pathDepth(path string) int {
	cleaned := filepath.Clean(path)
	if cleaned == "." || cleaned == string(filepath.Separator) {
		return 0
	}
	parts := strings.Split(cleaned, string(filepath.Separator))
	depth := 0
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			continue
		}
		depth++
	}
	return depth
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func (s *podmanService) setTunnelState(containerID string, state podmanTunnelState) bool {
	containerID = strings.TrimSpace(containerID)
	if containerID == "" || strings.TrimSpace(state.Status) == "" {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	current, ok := s.tunnelStateByContainerID[containerID]
	if ok && current == state {
		return false
	}
	s.tunnelStateByContainerID[containerID] = state
	return true
}

func (s *podmanService) clearTunnelState(containerID string) {
	containerID = strings.TrimSpace(containerID)
	if containerID == "" {
		return
	}
	s.mu.Lock()
	for key := range s.tunnelStateByContainerID {
		if isContainerIDMatch(key, containerID) {
			delete(s.tunnelStateByContainerID, key)
		}
	}
	s.mu.Unlock()
}

func enrichContainersWithTunnelState(containers []podmanContainer, tunnelStateByContainerID map[string]podmanTunnelState) {
	for i := range containers {
		state, ok := findTunnelStateForContainerID(strings.TrimSpace(containers[i].ID), tunnelStateByContainerID)
		if !ok {
			containers[i].TunnelStatus = ""
			containers[i].TunnelCode = ""
			containers[i].TunnelMessage = ""
			containers[i].TunnelURL = ""
			continue
		}
		containers[i].TunnelStatus = state.Status
		containers[i].TunnelCode = state.Code
		containers[i].TunnelMessage = state.Message
		containers[i].TunnelURL = buildTunnelConnectURLForContainer(containers[i].Name, containers[i].Labels, state.Status)
	}
}

func buildTunnelConnectURLForContainer(containerName string, labels map[string]string, tunnelStatus string) string {
	status := strings.TrimSpace(strings.ToLower(tunnelStatus))
	if status != tunnelStatusReady {
		return ""
	}
	name := buildTunnelName(containerName, "")
	if name == "" {
		return ""
	}
	baseURL := "https://vscode.dev/tunnel/" + name
	workspacePath := buildWorkspaceOpenPath(labels)
	if workspacePath == "" {
		return baseURL
	}
	return baseURL + workspacePath
}

func buildWorkspaceOpenPath(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	workspaceHome := strings.TrimSpace(labels[labelWorkspaceHome])
	workspaceDir := strings.TrimSpace(labels[labelWorkspaceDir])
	if workspaceHome == "" || workspaceDir == "" {
		return ""
	}

	fullPath := strings.TrimRight(workspaceHome, "/") + "/workspaces/" + workspaceDir
	fullPath = strings.TrimSpace(fullPath)
	if fullPath == "" {
		return ""
	}
	parts := strings.Split(fullPath, "/")
	escapedParts := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		escapedParts = append(escapedParts, neturl.PathEscape(part))
	}
	if len(escapedParts) == 0 {
		return ""
	}
	return "/" + strings.Join(escapedParts, "/")
}

func pruneTunnelStateMap(tunnelStateByContainerID map[string]podmanTunnelState, containers []podmanContainer) {
	if len(tunnelStateByContainerID) == 0 {
		return
	}
	knownIDs := make([]string, 0, len(containers))
	for _, container := range containers {
		id := strings.TrimSpace(container.ID)
		if id == "" {
			continue
		}
		knownIDs = append(knownIDs, id)
	}

	for containerID := range tunnelStateByContainerID {
		matched := false
		for _, knownID := range knownIDs {
			if isContainerIDMatch(containerID, knownID) {
				matched = true
				break
			}
		}
		if !matched {
			delete(tunnelStateByContainerID, containerID)
		}
	}
}

func findTunnelStateForContainerID(containerID string, tunnelStateByContainerID map[string]podmanTunnelState) (podmanTunnelState, bool) {
	if containerID == "" {
		return podmanTunnelState{}, false
	}
	if state, ok := tunnelStateByContainerID[containerID]; ok {
		return state, true
	}
	for id, state := range tunnelStateByContainerID {
		if isContainerIDMatch(id, containerID) {
			return state, true
		}
	}
	return podmanTunnelState{}, false
}

func isContainerIDMatch(left string, right string) bool {
	left = strings.ToLower(strings.TrimSpace(left))
	right = strings.ToLower(strings.TrimSpace(right))
	if left == "" || right == "" {
		return false
	}
	return left == right || strings.HasPrefix(left, right) || strings.HasPrefix(right, left)
}
