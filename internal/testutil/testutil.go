// Shared test helpers live here so packages do not duplicate them.
package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

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

// InitRepo chdirs the process: tests using it must not use t.Parallel.
func InitRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	// Isolate git configuration: tests must never inherit the caller's
	// global or system settings (commit signing, credential helpers,
	// agit.contextCommand, ...), which could otherwise leak into
	// temp-repo behavior and make tests environment-dependent.
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")

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

func WriteFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func StageFile(t *testing.T, dir, path, content string) {
	t.Helper()

	WriteFile(t, filepath.Join(dir, path), content)
	RunGit(t, dir, "add", path)
}

func CommitFile(t *testing.T, dir, path, content, message string) string {
	t.Helper()

	StageFile(t, dir, path, content)
	RunGit(t, dir, "commit", "-qm", message)
	return strings.TrimSpace(RunGit(t, dir, "rev-parse", "HEAD"))
}
