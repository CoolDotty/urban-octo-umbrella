package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

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

func TestDeriveWorkspaceHomeFromPasswd(t *testing.T) {
	passwd := strings.Join([]string{
		"root:x:0:0:root:/root:/bin/bash",
		"ubuntu:x:1000:1000:ubuntu:/home/ubuntu:/bin/bash",
	}, "\n")

	target := deriveWorkspaceHomeFromPasswd(passwd)
	if target != "/home/ubuntu" {
		t.Fatalf("expected ubuntu home, got %q", target)
	}
}

func TestDeriveWorkspaceHomeFromPasswdFallsBackToRoot(t *testing.T) {
	passwd := "root:x:0:0:root:/root:/bin/bash"
	target := deriveWorkspaceHomeFromPasswd(passwd)
	if target != defaultWorkspaceHome {
		t.Fatalf("expected default home path, got %q", target)
	}
}

func TestFormatWorkspaceVSCodeMountArgUsesTarget(t *testing.T) {
	mountArg := formatWorkspaceVSCodeMountArg("C:/work/volumes/user/.vscode", "/home/ubuntu/.vscode")
	if !strings.Contains(mountArg, "dst=/home/ubuntu/.vscode") {
		t.Fatalf("expected target mount path, got %q", mountArg)
	}
}

func TestFormatWorkspaceMountArgUsesTarget(t *testing.T) {
	mountArg := formatWorkspaceMountArg("C:/work/volumes/user/workspaces", "/home/ubuntu/workspaces")
	if !strings.Contains(mountArg, "dst=/home/ubuntu/workspaces") {
		t.Fatalf("expected target mount path, got %q", mountArg)
	}
}

func TestEnsureWorkspaceRepoBasePath(t *testing.T) {
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	tempRoot := t.TempDir()
	if err := os.Chdir(tempRoot); err != nil {
		t.Fatalf("chdir temp: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	path, err := ensureWorkspaceRepoBasePath("user-1")
	if err != nil {
		t.Fatalf("ensure path: %v", err)
	}
	if !filepath.IsAbs(path) {
		t.Fatalf("expected absolute path, got %q", path)
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Fatalf("stat created path: %v", statErr)
	}
	if !strings.HasSuffix(filepath.ToSlash(path), "/volumes/user-1/workspaces") {
		t.Fatalf("unexpected workspace path: %q", path)
	}
}

func TestDeriveWorkspaceDirName(t *testing.T) {
	tests := []struct {
		name    string
		payload createWorkspacePayload
		want    string
		wantErr bool
	}{
		{
			name: "uses explicit name",
			payload: createWorkspacePayload{
				Name:    "my-workspace",
				RepoURL: "https://github.com/org/repo.git",
			},
			want: "my-workspace",
		},
		{
			name: "derives from https repo",
			payload: createWorkspacePayload{
				RepoURL: "https://github.com/org/repo.git",
			},
			want: "repo",
		},
		{
			name: "derives from scp repo",
			payload: createWorkspacePayload{
				RepoURL: "git@github.com:org/repo-name.git",
			},
			want: "repo-name",
		},
		{
			name: "invalid when derivation is empty",
			payload: createWorkspacePayload{
				RepoURL: "https://github.com/org/.git",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		got, err := deriveWorkspaceDirName(tt.payload)
		if tt.wantErr {
			if err == nil {
				t.Fatalf("%s: expected error", tt.name)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", tt.name, err)
		}
		if got != tt.want {
			t.Fatalf("%s: expected %q, got %q", tt.name, tt.want, got)
		}
	}
}

func TestIsWorkspaceRepoPathAvailable(t *testing.T) {
	base := t.TempDir()
	missingPath := filepath.Join(base, "missing")

	available, err := isWorkspaceRepoPathAvailable(missingPath)
	if err != nil {
		t.Fatalf("missing path check: %v", err)
	}
	if !available {
		t.Fatal("expected missing path to be available")
	}

	nonEmptyPath := filepath.Join(base, "non-empty")
	if mkErr := os.MkdirAll(nonEmptyPath, 0o755); mkErr != nil {
		t.Fatalf("mkdir non-empty: %v", mkErr)
	}
	if writeErr := os.WriteFile(filepath.Join(nonEmptyPath, "README.md"), []byte("x"), 0o644); writeErr != nil {
		t.Fatalf("write non-empty file: %v", writeErr)
	}

	available, err = isWorkspaceRepoPathAvailable(nonEmptyPath)
	if err != nil {
		t.Fatalf("non-empty path check: %v", err)
	}
	if available {
		t.Fatal("expected non-empty path to be unavailable")
	}

	emptyPath := filepath.Join(base, "empty")
	if mkErr := os.MkdirAll(emptyPath, 0o755); mkErr != nil {
		t.Fatalf("mkdir empty: %v", mkErr)
	}

	available, err = isWorkspaceRepoPathAvailable(emptyPath)
	if err != nil {
		t.Fatalf("empty path check: %v", err)
	}
	if !available {
		t.Fatal("expected empty path to be available")
	}
}

func TestCloneWorkspaceRepository(t *testing.T) {
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	originalRun := runWorkspaceCommand
	originalLookPath := workspaceLookPath
	t.Cleanup(func() {
		runWorkspaceCommand = originalRun
		workspaceLookPath = originalLookPath
	})

	tests := []struct {
		name       string
		payload    createWorkspacePayload
		lookPath   func(string) (string, error)
		run        func(string, ...string) ([]byte, error)
		wantErrIs  error
		wantCalls  [][]string
		prepareDir bool
	}{
		{
			name: "fails when git missing",
			payload: createWorkspacePayload{
				RepoURL: "https://github.com/org/repo.git",
			},
			lookPath:  func(string) (string, error) { return "", errors.New("missing") },
			wantErrIs: errWorkspaceGitMissing,
		},
		{
			name: "fails when clone fails",
			payload: createWorkspacePayload{
				RepoURL: "https://github.com/org/repo.git",
			},
			lookPath: func(string) (string, error) { return "git", nil },
			run: func(name string, args ...string) ([]byte, error) {
				if name == "git" && len(args) >= 1 && args[0] == "clone" {
					return []byte("clone failed"), errors.New("clone error")
				}
				return nil, nil
			},
			wantErrIs: errWorkspaceCloneFailed,
		},
		{
			name: "fails when ref checkout fails",
			payload: createWorkspacePayload{
				RepoURL: "https://github.com/org/repo.git",
				Ref:     "main",
			},
			lookPath: func(string) (string, error) { return "git", nil },
			run: func(name string, args ...string) ([]byte, error) {
				if name == "git" && len(args) >= 3 && args[2] == "checkout" {
					return []byte("bad ref"), errors.New("checkout error")
				}
				return nil, nil
			},
			wantErrIs: errWorkspaceRefFailed,
		},
		{
			name: "fails when target dir exists and non-empty",
			payload: createWorkspacePayload{
				RepoURL: "https://github.com/org/repo.git",
				Name:    "already-exists",
			},
			lookPath:   func(string) (string, error) { return "git", nil },
			run:        func(string, ...string) ([]byte, error) { return nil, nil },
			wantErrIs:  errWorkspaceDirConflict,
			prepareDir: true,
		},
		{
			name: "success clone without ref",
			payload: createWorkspacePayload{
				RepoURL: "https://github.com/org/repo.git",
			},
			lookPath: func(string) (string, error) { return "git", nil },
			wantCalls: [][]string{
				{"git", "clone", "--"},
			},
		},
		{
			name: "success clone with ref",
			payload: createWorkspacePayload{
				RepoURL: "https://github.com/org/repo.git",
				Ref:     "main",
			},
			lookPath: func(string) (string, error) { return "git", nil },
			wantCalls: [][]string{
				{"git", "clone", "--"},
				{"git", "-C"},
			},
		},
	}

	for _, tt := range tests {
		tempRoot := t.TempDir()
		if err := os.Chdir(tempRoot); err != nil {
			t.Fatalf("%s: chdir temp: %v", tt.name, err)
		}

		calls := make([][]string, 0)
		workspaceLookPath = tt.lookPath
		runWorkspaceCommand = func(name string, args ...string) ([]byte, error) {
			call := append([]string{name}, args...)
			calls = append(calls, call)
			if tt.run != nil {
				return tt.run(name, args...)
			}
			return nil, nil
		}

		if tt.prepareDir {
			conflictPath := filepath.Join(tempRoot, "volumes", "user-1", "workspaces", "already-exists")
			if mkErr := os.MkdirAll(conflictPath, 0o755); mkErr != nil {
				t.Fatalf("%s: mkdir conflict: %v", tt.name, mkErr)
			}
			if writeErr := os.WriteFile(filepath.Join(conflictPath, "x.txt"), []byte("x"), 0o644); writeErr != nil {
				t.Fatalf("%s: write conflict file: %v", tt.name, writeErr)
			}
		}

		_, _, gotErr := cloneWorkspaceRepository("user-1", tt.payload)
		if err := os.Chdir(originalWD); err != nil {
			t.Fatalf("%s: chdir original: %v", tt.name, err)
		}
		if tt.wantErrIs != nil {
			if !errors.Is(gotErr, tt.wantErrIs) {
				t.Fatalf("%s: expected error %v, got %v", tt.name, tt.wantErrIs, gotErr)
			}
			continue
		}
		if gotErr != nil {
			t.Fatalf("%s: unexpected error: %v", tt.name, gotErr)
		}

		if len(tt.wantCalls) > 0 {
			for _, want := range tt.wantCalls {
				if !containsCallPrefix(calls, want) {
					t.Fatalf("%s: expected call prefix %v, got calls %v", tt.name, want, calls)
				}
			}
		}
	}
}

func containsCallPrefix(calls [][]string, prefix []string) bool {
	for _, call := range calls {
		if len(call) < len(prefix) {
			continue
		}
		if reflect.DeepEqual(call[:len(prefix)], prefix) {
			return true
		}
	}
	return false
}
