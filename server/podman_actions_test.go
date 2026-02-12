package main

import "testing"

func TestIsPodmanContainerNotFound(t *testing.T) {
	cases := []string{
		"Error: no such container",
		"Error: no container with name or ID \"abc\" found",
		"Error: unable to find container \"abc\"",
	}

	for _, c := range cases {
		if !isPodmanContainerNotFound([]byte(c)) {
			t.Fatalf("expected not found match for %q", c)
		}
	}

	if isPodmanContainerNotFound([]byte("Error: image not found")) {
		t.Fatal("did not expect unrelated error to match")
	}
}

func TestIsPodmanContainerAlreadyStopped(t *testing.T) {
	cases := []string{
		"Error: container is not running",
		"Error: can only stop created or running containers: container state improper",
	}

	for _, c := range cases {
		if !isPodmanContainerAlreadyStopped([]byte(c)) {
			t.Fatalf("expected already stopped match for %q", c)
		}
	}

	if isPodmanContainerAlreadyStopped([]byte("Error: no such container")) {
		t.Fatal("did not expect not found message to match already stopped")
	}
}

func TestIsPodmanContainerAlreadyRunning(t *testing.T) {
	if !isPodmanContainerAlreadyRunning([]byte("Error: container is already running")) {
		t.Fatal("expected already running match")
	}
	if isPodmanContainerAlreadyRunning([]byte("Error: no such container")) {
		t.Fatal("did not expect not found message to match already running")
	}
}
