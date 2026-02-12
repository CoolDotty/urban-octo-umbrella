package main

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

var errPodmanContainerNotFound = errors.New("podman container not found")

func (s *podmanService) stopContainer(containerID string) error {
	output, err := runPodmanCommand("stop", containerID)
	if err != nil {
		switch {
		case isPodmanContainerNotFound(output):
			return errPodmanContainerNotFound
		case isPodmanContainerAlreadyStopped(output):
			s.schedulePoll(podmanPollDebounce)
			return nil
		default:
			return fmt.Errorf("stop container: %w", err)
		}
	}

	s.schedulePoll(podmanPollDebounce)
	return nil
}

func (s *podmanService) startContainer(containerID string) error {
	output, err := runPodmanCommand("start", containerID)
	if err != nil {
		switch {
		case isPodmanContainerNotFound(output):
			return errPodmanContainerNotFound
		case isPodmanContainerAlreadyRunning(output):
			s.schedulePoll(podmanPollDebounce)
			return nil
		default:
			return fmt.Errorf("start container: %w", err)
		}
	}

	s.schedulePoll(podmanPollDebounce)
	return nil
}

func (s *podmanService) deleteContainer(containerID string) error {
	output, err := runPodmanCommand("rm", "-f", containerID)
	if err != nil {
		if isPodmanContainerNotFound(output) {
			return errPodmanContainerNotFound
		}
		return fmt.Errorf("delete container: %w", err)
	}

	s.schedulePoll(podmanPollDebounce)
	return nil
}

func runPodmanCommand(args ...string) ([]byte, error) {
	if _, err := exec.LookPath("podman"); err != nil {
		return nil, errPodmanUnavailable
	}

	cmd := exec.Command("podman", args...)
	return cmd.CombinedOutput()
}

func isPodmanContainerNotFound(output []byte) bool {
	text := strings.ToLower(string(output))

	return strings.Contains(text, "no such container") ||
		strings.Contains(text, "no container with name or id") ||
		strings.Contains(text, "unable to find")
}

func isPodmanContainerAlreadyStopped(output []byte) bool {
	text := strings.ToLower(string(output))

	return strings.Contains(text, "is not running") ||
		strings.Contains(text, "container state improper")
}

func isPodmanContainerAlreadyRunning(output []byte) bool {
	text := strings.ToLower(string(output))

	return strings.Contains(text, "already running")
}
