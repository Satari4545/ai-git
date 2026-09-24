// Package testutil provides shared helpers for tests that need throwaway git
// repositories. It lives in its own package so both internal/agit and
// internal/state tests can use the same helpers without duplicating them.
package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// RunGit runs git with args in dir, failing the test on error, and returns the
// combined output.
func RunGit(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return string(out)
}

// InitRepo creates a temp git repo with a test identity, chdirs into it
// (restoring the original working directory on test cleanup), and returns its
// path.
//
// Because it changes the process working directory, tests that use InitRepo
// must not use t.Parallel: they share the process cwd.
func InitRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	RunGit(t, dir, "init", "-q")
	RunGit(t, dir, "config", "user.email", "agit-test@example.com")
	RunGit(t, dir, "config", "user.name", "agit-test")

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldwd); err != nil {
			t.Fatal(err)
		}
	})

	return dir
}

// WriteFile writes content to path, creating parent directories as needed.
func WriteFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// StageFile writes content to dir/path and stages it with git add.
func StageFile(t *testing.T, dir, path, content string) {
	t.Helper()

	WriteFile(t, filepath.Join(dir, path), content)
	RunGit(t, dir, "add", path)
}

// CommitFile writes content to dir/path, commits it, and returns the new
// commit SHA.
func CommitFile(t *testing.T, dir, path, content, message string) string {
	t.Helper()

	StageFile(t, dir, path, content)
	RunGit(t, dir, "commit", "-qm", message)
	return strings.TrimSpace(RunGit(t, dir, "rev-parse", "HEAD"))
}
