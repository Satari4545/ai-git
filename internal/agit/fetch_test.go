package agit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Satari4545/ai-git/internal/testutil"
)

func setupBareRemote(t *testing.T) string {
	t.Helper()
	bare := t.TempDir()
	testutil.RunGit(t, bare, "init", "--bare", "-q")
	testutil.RunGit(t, bare, "symbolic-ref", "HEAD", "refs/heads/main")
	return bare
}

func commitWithNote(t *testing.T, dir, msg, note string) {
	t.Helper()
	testutil.RunGit(t, dir, "commit", "--allow-empty", "-q", "-m", msg)
	testutil.RunGit(t, dir, "notes", "--ref=agit", "add", "-m", note, "HEAD")
}

func pushBranchAndNotes(t *testing.T, dir, bare string) {
	t.Helper()
	testutil.RunGit(t, dir, "push", "-q", bare, "HEAD:refs/heads/main", NotesRef+":"+NotesRef)
}

func chdir(t *testing.T, dir string) {
	t.Helper()
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
}

func noteForHEAD(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "notes", "--ref=agit", "show", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(out)
}

func TestRemoteFromArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"no args defaults to origin", nil, "origin"},
		{"bare remote", []string{"origin"}, "origin"},
		{"flags before remote", []string{"--prune", "upstream"}, "upstream"},
		{"remote and branch", []string{"origin", "main"}, "origin"},
		{"value flag skipped", []string{"--depth", "1", "origin"}, "origin"},
		{"flag-only args default to origin", []string{"--tags"}, "origin"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := remoteFromArgs(tt.args); got != tt.want {
				t.Errorf("remoteFromArgs(%v) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}

func TestFetchSyncsNotesFromRemote(t *testing.T) {
	bare := setupBareRemote(t)
	work := testutil.InitRepo(t)

	commitWithNote(t, work, "first", `{"contextID":"ctx-fetch-1"}`)
	pushBranchAndNotes(t, work, bare)

	parent := t.TempDir()
	testutil.RunGit(t, parent, "clone", "-q", bare, "clone")
	clone := filepath.Join(parent, "clone")

	if got := noteForHEAD(t, clone); got != "" {
		t.Fatalf("plain clone unexpectedly carried agit notes: %q", got)
	}

	chdir(t, clone)
	if err := Fetch(nil); err != nil {
		t.Fatalf("Fetch(nil) = %v", err)
	}

	if got := noteForHEAD(t, clone); !strings.Contains(got, "ctx-fetch-1") {
		t.Errorf("after Fetch, note on HEAD = %q, want it to contain ctx-fetch-1", got)
	}
}

func TestFetchToleratesRemoteWithoutNotes(t *testing.T) {
	bare := setupBareRemote(t)
	work := testutil.InitRepo(t)

	testutil.RunGit(t, work, "commit", "--allow-empty", "-q", "-m", "no notes here")
	testutil.RunGit(t, work, "push", "-q", bare, "HEAD:refs/heads/main")

	parent := t.TempDir()
	testutil.RunGit(t, parent, "clone", "-q", bare, "clone")
	clone := filepath.Join(parent, "clone")

	chdir(t, clone)
	if err := Fetch(nil); err != nil {
		t.Fatalf("Fetch(nil) on note-less remote = %v, want nil", err)
	}
}

func TestPullSyncsNotesFromRemote(t *testing.T) {
	bare := setupBareRemote(t)
	work := testutil.InitRepo(t)

	commitWithNote(t, work, "first", `{"contextID":"ctx-pull-1"}`)
	pushBranchAndNotes(t, work, bare)

	parent := t.TempDir()
	testutil.RunGit(t, parent, "clone", "-q", bare, "clone")
	clone := filepath.Join(parent, "clone")

	commitWithNote(t, work, "second", `{"contextID":"ctx-pull-2"}`)
	pushBranchAndNotes(t, work, bare)

	chdir(t, clone)
	if err := Pull(nil); err != nil {
		t.Fatalf("Pull(nil) = %v", err)
	}

	if got := noteForHEAD(t, clone); !strings.Contains(got, "ctx-pull-2") {
		t.Errorf("after Pull, note on HEAD = %q, want it to contain ctx-pull-2", got)
	}
}
