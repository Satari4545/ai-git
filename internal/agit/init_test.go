package agit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Satari4545/ai-git/internal/testutil"
)

func chdirTo(t *testing.T, dir string) {
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

func fetchRefspecs(t *testing.T, dir string) []string {
	t.Helper()
	out := testutil.RunGit(t, dir, "config", "--get-all", "remote.origin.fetch")
	var specs []string
	for _, line := range strings.Split(out, "\n") {
		if s := strings.TrimSpace(line); s != "" {
			specs = append(specs, s)
		}
	}
	return specs
}

func TestInitInitializesGitRepo(t *testing.T) {
	dir := t.TempDir()
	chdirTo(t, dir)

	if err := Init(); err != nil {
		t.Fatalf("Init() in non-repo = %v, want nil", err)
	}
	if out := testutil.RunGit(t, dir, "rev-parse", "--is-inside-work-tree"); strings.TrimSpace(out) != "true" {
		t.Errorf("after Init, not inside a work tree")
	}
}

func TestInitAddsNotesRefspecWhenOriginExists(t *testing.T) {
	dir := testutil.InitRepo(t)
	bare := t.TempDir()
	testutil.RunGit(t, bare, "init", "--bare", "-q")
	testutil.RunGit(t, dir, "remote", "add", "origin", bare)

	if err := Init(); err != nil {
		t.Fatalf("Init() = %v, want nil", err)
	}

	found := false
	for _, s := range fetchRefspecs(t, dir) {
		if s == notesFetchRefspec {
			found = true
		}
	}
	if !found {
		t.Errorf("notes refspec %q not found in remote.origin.fetch", notesFetchRefspec)
	}
}

func TestInitIsIdempotent(t *testing.T) {
	dir := testutil.InitRepo(t)
	bare := t.TempDir()
	testutil.RunGit(t, bare, "init", "--bare", "-q")
	testutil.RunGit(t, dir, "remote", "add", "origin", bare)

	for i := 0; i < 2; i++ {
		if err := Init(); err != nil {
			t.Fatalf("Init() run %d = %v, want nil", i+1, err)
		}
	}

	count := 0
	for _, s := range fetchRefspecs(t, dir) {
		if s == notesFetchRefspec {
			count++
		}
	}
	if count != 1 {
		t.Errorf("notes refspec appears %d times, want exactly 1", count)
	}
}

func TestInitWithoutOriginSkipsRefspec(t *testing.T) {
	dir := testutil.InitRepo(t)

	if err := Init(); err != nil {
		t.Fatalf("Init() without origin = %v, want nil", err)
	}

	if _, err := os.Stat(filepath.Join(dir, ".agit")); !os.IsNotExist(err) {
		t.Errorf(".agit dir should not be created eagerly by init")
	}
}

func TestInitConfiguresAllRemotes(t *testing.T) {
	dir := testutil.InitRepo(t)
	bare1 := t.TempDir()
	bare2 := t.TempDir()
	testutil.RunGit(t, bare1, "init", "--bare", "-q")
	testutil.RunGit(t, bare2, "init", "--bare", "-q")
	testutil.RunGit(t, dir, "remote", "add", "origin", bare1)
	testutil.RunGit(t, dir, "remote", "add", "upstream", bare2)

	if err := Init(); err != nil {
		t.Fatalf("Init() = %v, want nil", err)
	}

	for _, name := range []string{"origin", "upstream"} {
		out := testutil.RunGit(t, dir, "config", "--get-all", "remote."+name+".fetch")
		if !strings.Contains(out, "refs/notes/agit") {
			t.Errorf("remote %q fetch refspec missing notes ref", name)
		}
	}
}
