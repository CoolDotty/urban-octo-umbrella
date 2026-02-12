package main

import "testing"

func TestValidateCreateWorkspacePayloadMinimalValid(t *testing.T) {
	payload := createWorkspacePayload{
		RepoURL: "https://github.com/org/repo.git",
	}

	if err := validateCreateWorkspacePayload(&payload); err != nil {
		t.Fatalf("expected valid payload, got %v", err)
	}
}

func TestValidateCreateWorkspacePayloadMissingRepo(t *testing.T) {
	payload := createWorkspacePayload{}

	if err := validateCreateWorkspacePayload(&payload); err == nil {
		t.Fatal("expected missing repoUrl to fail")
	}
}

func TestValidateCreateWorkspacePayloadInvalidName(t *testing.T) {
	payload := createWorkspacePayload{
		RepoURL: "https://github.com/org/repo.git",
		Name:    "bad/name",
	}

	if err := validateCreateWorkspacePayload(&payload); err == nil {
		t.Fatal("expected invalid name to fail")
	}
}

func TestValidateCreateWorkspacePayloadInvalidEnvKey(t *testing.T) {
	payload := createWorkspacePayload{
		RepoURL: "https://github.com/org/repo.git",
		Env: map[string]string{
			"NOT-VALID": "x",
		},
	}

	if err := validateCreateWorkspacePayload(&payload); err == nil {
		t.Fatal("expected invalid env key to fail")
	}
}

func TestIsValidGitRepoURL(t *testing.T) {
	valid := []string{
		"https://github.com/org/repo.git",
		"http://github.com/org/repo",
		"ssh://git@github.com/org/repo.git",
		"git://github.com/org/repo.git",
		"git@github.com:org/repo.git",
	}
	for _, candidate := range valid {
		if !isValidGitRepoURL(candidate) {
			t.Fatalf("expected valid repo URL: %s", candidate)
		}
	}

	invalid := []string{
		"",
		"not-a-url",
		"https://",
		"git@github.com",
		"https://github.com",
		"https://github.com/ space/repo",
	}
	for _, candidate := range invalid {
		if isValidGitRepoURL(candidate) {
			t.Fatalf("expected invalid repo URL: %s", candidate)
		}
	}
}

func TestIsPodmanNameConflict(t *testing.T) {
	if !isPodmanNameConflict([]byte("Error: the container name is already in use")) {
		t.Fatal("expected name conflict match")
	}
	if isPodmanNameConflict([]byte("Error: image not found")) {
		t.Fatal("did not expect unrelated error to match conflict")
	}
}
