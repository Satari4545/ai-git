package agit

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// initTestRepo creates an empty git repo in a temp dir and chdirs the test
// process into it. Cleanup restores the original working directory.
// Tests in this package must not use t.Parallel: they share the process cwd.
func initTestRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "config", "user.email", "agit-test@example.com")
	runGit(t, dir, "config", "user.name", "agit-test")

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

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return string(out)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// commitFile stages path with content and creates a commit, returning its SHA.
func commitFile(t *testing.T, dir, path, content, message string) string {
	t.Helper()

	writeFile(t, filepath.Join(dir, path), content)
	runGit(t, dir, "add", path)
	runGit(t, dir, "commit", "-qm", message)
	out := runGit(t, dir, "rev-parse", "HEAD")
	// Trim trailing newline.
	return out[:len(out)-1]
}

// addAgitNote attaches raw JSON as an AGIT note to the given commit,
// using the exact ref the product reads and writes.
func addAgitNote(t *testing.T, dir, commit, noteJSON string) {
	t.Helper()

	noteFile := filepath.Join(dir, ".agit-test-note.json")
	if err := os.WriteFile(noteFile, []byte(noteJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "notes", "--ref=agit", "add", "-f", "-F", noteFile, commit)
	if err := os.Remove(noteFile); err != nil {
		t.Fatal(err)
	}
}
