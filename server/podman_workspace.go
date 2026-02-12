package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"regexp"
	"sort"
	"strings"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
)

const (
	defaultWorkspaceImage = "mcr.microsoft.com/devcontainers/universal:latest"

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

	workspaceNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,127}$`)
	envKeyPattern        = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	scpLikeGitPattern    = regexp.MustCompile(`^git@[A-Za-z0-9._-]+:[^\s]+$`)
)

type createWorkspacePayload struct {
	RepoURL   string            `json:"repoUrl"`
	Name      string            `json:"name"`
	Ref       string            `json:"ref"`
	Env       map[string]string `json:"env"`
	AutoStart *bool             `json:"autoStart"`
}

type createWorkspaceResponse struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	RepoURL string `json:"repoUrl"`
	Ref     string `json:"ref,omitempty"`
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

		result, err := svc.createWorkspace(payload)
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
			case errors.Is(err, errWorkspaceStartFailed):
				return re.JSON(http.StatusInternalServerError, map[string]string{
					"message": workspaceStartFailedMessage,
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

func (s *podmanService) createWorkspace(payload createWorkspacePayload) (*createWorkspaceResponse, error) {
	if _, err := exec.LookPath("podman"); err != nil {
		return nil, errPodmanUnavailable
	}

	autoStart := payload.AutoStart == nil || *payload.AutoStart

	args := []string{"create", "--pull=missing"}
	if payload.Name != "" {
		args = append(args, "--name", payload.Name)
	}

	keys := make([]string, 0, len(payload.Env))
	for key := range payload.Env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		args = append(args, "-e", fmt.Sprintf("%s=%s", key, payload.Env[key]))
	}

	args = append(args, "--label", fmt.Sprintf("pocketpod.repo=%s", payload.RepoURL))
	if payload.Ref != "" {
		args = append(args, "--label", fmt.Sprintf("pocketpod.ref=%s", payload.Ref))
	}

	args = append(args, defaultWorkspaceImage)

	createCmd := exec.Command("podman", args...)
	createOutput, createErr := createCmd.CombinedOutput()
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

	if autoStart {
		startCmd := exec.Command("podman", "start", containerID)
		if _, err := startCmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("%w: %v", errWorkspaceStartFailed, err)
		}
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
		if autoStart {
			status = "Running"
		} else {
			status = "Created"
		}
	}

	s.schedulePoll(podmanPollDebounce)

	return &createWorkspaceResponse{
		Name:    name,
		Status:  status,
		RepoURL: payload.RepoURL,
		Ref:     payload.Ref,
	}, nil
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
