package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
)

const (
	defaultWorkspaceImage   = "mcr.microsoft.com/devcontainers/universal"
	defaultWorkspaceCommand = "while true; do sleep 3600; done"
	defaultWorkspaceHome    = "/root"
	defaultVSCodeMountPath  = "/root/.vscode"
	labelWorkspaceRepo      = "pocketpod.repo"
	labelWorkspaceRef       = "pocketpod.ref"
	labelWorkspaceDir       = "pocketpod.workspace_dir"
	labelWorkspaceHome      = "pocketpod.workspace_home"

	workspaceCreateFailedMessage = "Failed to create workspace container."
	workspaceStartFailedMessage  = "Failed to start workspace container."

	maxRepoURLLength         = 2048
	maxWorkspaceNameLength   = 128
	maxWorkspaceRefLength    = 256
	maxWorkspaceEnvCount     = 64
	maxWorkspaceEnvKeyLength = 128
	maxWorkspaceEnvValLength = 4096
)

var (
	errWorkspaceNameConflict = errors.New("workspace name already exists")
	errWorkspaceDirConflict  = errors.New("workspace directory already exists")
	errWorkspaceGitMissing   = errors.New("git unavailable")
	errWorkspaceCloneFailed  = errors.New("workspace clone failed")
	errWorkspaceRefFailed    = errors.New("workspace ref checkout failed")

	workspaceNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,127}$`)
	envKeyPattern        = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	scpLikeGitPattern    = regexp.MustCompile(`^git@[A-Za-z0-9._-]+:[^\s]+$`)
	invalidDirNameChars  = regexp.MustCompile(`[^A-Za-z0-9_.-]`)
)

var runWorkspaceCommand = func(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	return cmd.CombinedOutput()
}

var workspaceLookPath = exec.LookPath

type createWorkspacePayload struct {
	RepoURL string            `json:"repoUrl"`
	Name    string            `json:"name"`
	Ref     string            `json:"ref"`
	Env     map[string]string `json:"env"`
}

type createWorkspaceResponse struct {
	Name    string                  `json:"name"`
	Status  string                  `json:"status"`
	RepoURL string                  `json:"repoUrl"`
	Ref     string                  `json:"ref,omitempty"`
	Tunnel  workspaceTunnelSnapshot `json:"tunnel"`
}

type podmanInspectSummary struct {
	Name  string `json:"Name"`
	State struct {
		Status  string `json:"Status"`
		Running bool   `json:"Running"`
	} `json:"State"`
}

func registerWorkspaceRoutes(rtr *router.Router[*core.RequestEvent], svc *podmanService) {
	rtr.POST("/podman/workspaces", func(re *core.RequestEvent) error {
		if re.Auth == nil {
			return re.JSON(http.StatusUnauthorized, map[string]string{
				"message": "Unauthorized.",
			})
		}

		var payload createWorkspacePayload
		if err := re.BindBody(&payload); err != nil {
			return re.JSON(http.StatusBadRequest, map[string]string{
				"message": "Invalid workspace payload.",
			})
		}

		if err := validateCreateWorkspacePayload(&payload); err != nil {
			return re.JSON(http.StatusBadRequest, map[string]string{
				"message": err.Error(),
			})
		}

		result, err := svc.createWorkspace(re.Auth.Id, payload)
		if err != nil {
			switch {
			case errors.Is(err, errPodmanUnavailable):
				return re.JSON(http.StatusServiceUnavailable, map[string]string{
					"message": podmanUnavailableMessage,
				})
			case errors.Is(err, errWorkspaceNameConflict):
				return re.JSON(http.StatusConflict, map[string]string{
					"message": "Workspace name already exists.",
				})
			case errors.Is(err, errWorkspaceDirConflict):
				return re.JSON(http.StatusConflict, map[string]string{
					"message": "Workspace directory already exists.",
				})
			case errors.Is(err, errWorkspaceStartFailed):
				return re.JSON(http.StatusInternalServerError, map[string]string{
					"message": workspaceStartFailedMessage,
				})
			case errors.Is(err, errWorkspaceGitMissing):
				return re.JSON(http.StatusInternalServerError, map[string]string{
					"message": "Git is unavailable on the server.",
				})
			case errors.Is(err, errWorkspaceCloneFailed), errors.Is(err, errWorkspaceRefFailed):
				return re.JSON(http.StatusInternalServerError, map[string]string{
					"message": "Failed to clone workspace repository.",
				})
			default:
				return re.JSON(http.StatusInternalServerError, map[string]string{
					"message": workspaceCreateFailedMessage,
				})
			}
		}

		return re.JSON(http.StatusCreated, result)
	})
}

var errWorkspaceStartFailed = errors.New("workspace start failed")

func (s *podmanService) createWorkspace(userID string, payload createWorkspacePayload) (*createWorkspaceResponse, error) {
	if _, err := exec.LookPath("podman"); err != nil {
		return nil, errPodmanUnavailable
	}

	workspaceHostPath, workspaceDirName, err := cloneWorkspaceRepository(userID, payload)
	if err != nil {
		return nil, err
	}

	volumeHostPath, err := ensureWorkspaceVSCodeVolumePath(userID)
	if err != nil {
		return nil, err
	}
	workspaceHomeTarget := defaultWorkspaceHome
	if resolvedPath, resolveErr := resolveWorkspaceHomeTarget(defaultWorkspaceImage); resolveErr == nil && strings.TrimSpace(resolvedPath) != "" {
		workspaceHomeTarget = resolvedPath
	}
	workspaceMountArg := formatWorkspaceMountArg(workspaceHostPath, strings.TrimRight(workspaceHomeTarget, "/")+"/workspaces")
	vscodeMountTarget := strings.TrimRight(workspaceHomeTarget, "/") + "/.vscode"
	vscodeMountArg := formatWorkspaceVSCodeMountArg(volumeHostPath, vscodeMountTarget)

	args := []string{"create", "--pull=missing"}
	if payload.Name != "" {
		args = append(args, "--name", payload.Name)
	}
	args = append(args, "--mount", workspaceMountArg)
	args = append(args, "--mount", vscodeMountArg)

	keys := make([]string, 0, len(payload.Env))
	for key := range payload.Env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		args = append(args, "-e", fmt.Sprintf("%s=%s", key, payload.Env[key]))
	}

	args = append(args, "--label", fmt.Sprintf("%s=%s", labelWorkspaceRepo, payload.RepoURL))
	args = append(args, "--label", fmt.Sprintf("%s=%s", labelWorkspaceDir, workspaceDirName))
	args = append(args, "--label", fmt.Sprintf("%s=%s", labelWorkspaceHome, workspaceHomeTarget))
	if payload.Ref != "" {
		args = append(args, "--label", fmt.Sprintf("%s=%s", labelWorkspaceRef, payload.Ref))
	}

	sessionID := generateSessionID()
	args = append(args, "--label", fmt.Sprintf("%s=%s", labelTunnelSession, sessionID))

	args = append(args, defaultWorkspaceImage, "sh", "-lc", defaultWorkspaceCommand)

	createOutput, createErr := runWorkspaceCommand("podman", args...)
	if createErr != nil {
		if isPodmanNameConflict(createOutput) {
			return nil, errWorkspaceNameConflict
		}
		return nil, createErr
	}

	containerID := strings.TrimSpace(string(createOutput))
	if containerID == "" {
		return nil, errors.New("empty container id")
	}

	if _, err := runWorkspaceCommand("podman", "start", containerID); err != nil {
		return nil, fmt.Errorf("%w: %v", errWorkspaceStartFailed, err)
	}

	name, status := inspectCreatedContainer(containerID)
	if name == "" {
		if payload.Name != "" {
			name = payload.Name
		} else {
			name = containerID
		}
	}
	if status == "" {
		status = "Running"
	}

	tunnelState := s.bootstrapTunnel(containerID, name, sessionID)
	if tunnelState.Status == "" {
		tunnelState.Status = tunnelStatusStarting
	}
	if s.setTunnelState(containerID, tunnelState) {
		s.schedulePoll(podmanPollDebounce)
	}
	if tunnelState.Status == tunnelStatusStarting {
		s.startTunnelMonitor(containerID, sessionID, volumeHostPath)
	}

	return &createWorkspaceResponse{
		Name:    name,
		Status:  status,
		RepoURL: payload.RepoURL,
		Ref:     payload.Ref,
		Tunnel:  workspaceTunnelSnapshot(tunnelState),
	}, nil
}

func ensureWorkspaceVSCodeVolumePath(userID string) (string, error) {
	trimmedUserID := strings.TrimSpace(userID)
	if trimmedUserID == "" {
		return "", errors.New("missing user id")
	}

	volumeHostPath := filepath.Join(".", "volumes", trimmedUserID, ".vscode")
	if err := os.MkdirAll(volumeHostPath, 0o755); err != nil {
		return "", err
	}
	absolutePath, err := filepath.Abs(volumeHostPath)
	if err != nil {
		return "", err
	}
	return absolutePath, nil
}

func ensureWorkspaceRepoBasePath(userID string) (string, error) {
	trimmedUserID := strings.TrimSpace(userID)
	if trimmedUserID == "" {
		return "", errors.New("missing user id")
	}

	workspaceHostPath := filepath.Join(".", "volumes", trimmedUserID, "workspaces")
	if err := os.MkdirAll(workspaceHostPath, 0o755); err != nil {
		return "", err
	}
	absolutePath, err := filepath.Abs(workspaceHostPath)
	if err != nil {
		return "", err
	}
	return absolutePath, nil
}

func cloneWorkspaceRepository(userID string, payload createWorkspacePayload) (string, string, error) {
	if _, err := workspaceLookPath("git"); err != nil {
		return "", "", errWorkspaceGitMissing
	}

	repoBasePath, err := ensureWorkspaceRepoBasePath(userID)
	if err != nil {
		return "", "", err
	}
	dirName, err := deriveWorkspaceDirName(payload)
	if err != nil {
		return "", "", err
	}
	repoPath := filepath.Join(repoBasePath, dirName)
	available, err := isWorkspaceRepoPathAvailable(repoPath)
	if err != nil {
		return "", "", err
	}
	if !available {
		return "", "", errWorkspaceDirConflict
	}

	if cloneOutput, cloneErr := runWorkspaceCommand("git", "clone", "--", payload.RepoURL, repoPath); cloneErr != nil {
		return "", "", fmt.Errorf("%w: %s", errWorkspaceCloneFailed, strings.TrimSpace(string(cloneOutput)))
	}
	if payload.Ref != "" {
		if checkoutOutput, checkoutErr := runWorkspaceCommand("git", "-C", repoPath, "checkout", "--detach", payload.Ref); checkoutErr != nil {
			return "", "", fmt.Errorf("%w: %s", errWorkspaceRefFailed, strings.TrimSpace(string(checkoutOutput)))
		}
	}

	return repoBasePath, dirName, nil
}

func deriveWorkspaceDirName(payload createWorkspacePayload) (string, error) {
	if payload.Name != "" {
		return payload.Name, nil
	}

	repoPath := extractRepoPath(payload.RepoURL)
	repoPath = strings.Trim(repoPath, "/")
	if repoPath == "" {
		return "", errors.New("unable to derive workspace directory")
	}

	base := strings.TrimSpace(pathBase(repoPath))
	base = strings.TrimSuffix(base, ".git")
	base = invalidDirNameChars.ReplaceAllString(base, "-")
	base = strings.Trim(base, "-._")
	if base == "" {
		return "", errors.New("unable to derive workspace directory")
	}
	if len(base) > maxWorkspaceNameLength {
		base = strings.TrimRight(base[:maxWorkspaceNameLength], "-._")
	}
	if base == "" || !workspaceNamePattern.MatchString(base) {
		return "", errors.New("unable to derive workspace directory")
	}
	return base, nil
}

func extractRepoPath(repoURL string) string {
	if scpLikeGitPattern.MatchString(repoURL) {
		if idx := strings.Index(repoURL, ":"); idx >= 0 && idx+1 < len(repoURL) {
			return repoURL[idx+1:]
		}
		return ""
	}

	parsed, err := url.Parse(repoURL)
	if err != nil {
		return ""
	}
	return parsed.Path
}

func pathBase(pathValue string) string {
	cleaned := strings.TrimSuffix(pathValue, "/")
	if cleaned == "" {
		return ""
	}
	parts := strings.Split(cleaned, "/")
	return parts[len(parts)-1]
}

func isWorkspaceRepoPathAvailable(repoPath string) (bool, error) {
	info, err := os.Stat(repoPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return true, nil
		}
		return false, err
	}
	if !info.IsDir() {
		return false, nil
	}

	entries, readErr := os.ReadDir(repoPath)
	if readErr != nil {
		return false, readErr
	}
	return len(entries) == 0, nil
}

func formatWorkspaceMountArg(volumeHostPath string, mountTarget string) string {
	normalizedHostPath := filepath.ToSlash(strings.TrimSpace(volumeHostPath))
	target := strings.TrimSpace(mountTarget)
	if target == "" {
		target = strings.TrimRight(defaultWorkspaceHome, "/") + "/workspaces"
	}
	return fmt.Sprintf("type=bind,src=%s,dst=%s", normalizedHostPath, target)
}

func formatWorkspaceVSCodeMountArg(volumeHostPath string, mountTarget string) string {
	target := strings.TrimSpace(mountTarget)
	if target == "" {
		target = defaultVSCodeMountPath
	}
	return formatWorkspaceMountArg(volumeHostPath, target)
}

func resolveWorkspaceHomeTarget(imageRef string) (string, error) {
	output, err := runPodmanCommand("run", "--rm", "--entrypoint", "sh", imageRef, "-lc", "cat /etc/passwd 2>/dev/null || true")
	if err != nil {
		return "", err
	}
	return deriveWorkspaceHomeFromPasswd(string(output)), nil
}

func deriveWorkspaceHomeFromPasswd(passwdContents string) string {
	_, home, ok := selectFirstNonRootUser(passwdContents)
	if !ok {
		return defaultWorkspaceHome
	}
	trimmedHome := strings.TrimSpace(home)
	if trimmedHome == "" {
		return defaultWorkspaceHome
	}
	return strings.TrimRight(trimmedHome, "/")
}

func validateCreateWorkspacePayload(payload *createWorkspacePayload) error {
	payload.RepoURL = strings.TrimSpace(payload.RepoURL)
	payload.Name = strings.TrimSpace(payload.Name)
	payload.Ref = strings.TrimSpace(payload.Ref)

	switch {
	case payload.RepoURL == "":
		return errors.New("repoUrl is required")
	case len(payload.RepoURL) > maxRepoURLLength:
		return errors.New("repoUrl is too long")
	case !isValidGitRepoURL(payload.RepoURL):
		return errors.New("repoUrl must be a valid git URL")
	case hasUnsafeControlChars(payload.RepoURL):
		return errors.New("repoUrl contains unsupported characters")
	}

	if payload.Name != "" {
		switch {
		case len(payload.Name) > maxWorkspaceNameLength:
			return errors.New("name is too long")
		case !workspaceNamePattern.MatchString(payload.Name):
			return errors.New("name must be Podman-compatible")
		case hasUnsafeControlChars(payload.Name):
			return errors.New("name contains unsupported characters")
		}
	}

	if payload.Ref != "" {
		switch {
		case len(payload.Ref) > maxWorkspaceRefLength:
			return errors.New("ref is too long")
		case hasUnsafeControlChars(payload.Ref):
			return errors.New("ref contains unsupported characters")
		}
	}

	if len(payload.Env) > maxWorkspaceEnvCount {
		return errors.New("env has too many entries")
	}

	for key, value := range payload.Env {
		trimmedKey := strings.TrimSpace(key)
		switch {
		case trimmedKey == "":
			return errors.New("env keys must be non-empty")
		case key != trimmedKey:
			return errors.New("env key is invalid")
		case len(key) > maxWorkspaceEnvKeyLength:
			return errors.New("env key is too long")
		case !envKeyPattern.MatchString(key):
			return errors.New("env key is invalid")
		case len(value) > maxWorkspaceEnvValLength:
			return errors.New("env value is too long")
		case hasUnsafeControlChars(value):
			return errors.New("env value contains unsupported characters")
		}
	}

	return nil
}

func hasUnsafeControlChars(value string) bool {
	for _, ch := range value {
		if ch < 32 || ch == 127 {
			return true
		}
	}
	return false
}

func isValidGitRepoURL(value string) bool {
	if value == "" {
		return false
	}
	if strings.ContainsAny(value, " \t\r\n") {
		return false
	}

	if scpLikeGitPattern.MatchString(value) {
		return true
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return false
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return false
	}
	switch strings.ToLower(parsed.Scheme) {
	case "https", "http", "ssh", "git":
		return parsed.Path != "" && parsed.Path != "/"
	default:
		return false
	}
}

func isPodmanNameConflict(output []byte) bool {
	text := strings.ToLower(string(output))
	if strings.Contains(text, "name is already in use") {
		return true
	}
	if strings.Contains(text, "container name") && strings.Contains(text, "already") {
		return true
	}
	return false
}

func inspectCreatedContainer(containerID string) (string, string) {
	cmd := exec.Command("podman", "inspect", "--format", "json", containerID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", ""
	}

	var parsed []podmanInspectSummary
	if err := json.Unmarshal(output, &parsed); err != nil || len(parsed) == 0 {
		return "", ""
	}

	name := strings.TrimSpace(strings.TrimPrefix(parsed[0].Name, "/"))
	status := strings.TrimSpace(parsed[0].State.Status)
	if parsed[0].State.Running {
		status = "Running"
	} else if status != "" {
		status = strings.ToUpper(status[:1]) + status[1:]
	}

	return name, status
}
