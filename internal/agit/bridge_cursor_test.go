package agit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Satari4545/ai-git/internal/testutil"
)

func pathWithGitOnly(t *testing.T) string {
	t.Helper()

	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.Symlink(git, filepath.Join(dir, "git")); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestCursorBridgeRegistered(t *testing.T) {
	b, err := GetBridge("cursor")
	if err != nil {
		t.Fatalf("cursor bridge not registered: %v", err)
	}
	if b.Name() != "cursor" {
		t.Errorf("Name = %q, want cursor", b.Name())
	}
	if b.DisplayName() != "Cursor" {
		t.Errorf("DisplayName = %q, want Cursor", b.DisplayName())
	}

	found := false
	for _, name := range BridgeNames() {
		if name == "cursor" {
			found = true
		}
	}
	if !found {
		t.Errorf("BridgeNames missing cursor: %v", BridgeNames())
	}
}

func TestCursorInstall(t *testing.T) {
	dir := testutil.InitRepo(t)
	t.Setenv("HOME", t.TempDir())

	b, err := GetBridge("cursor")
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Install(); err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	if got := testutil.RunGit(t, dir, "config", "--get", "agit.contextCommand"); strings.TrimSpace(got) != "agit bridge cursor provider" {
		t.Errorf("agit.contextCommand = %q", strings.TrimSpace(got))
	}
	if got := testutil.RunGit(t, dir, "config", "--get", "agit.bridge"); strings.TrimSpace(got) != "cursor" {
		t.Errorf("agit.bridge = %q", strings.TrimSpace(got))
	}

	if err := b.Install(); err != nil {
		t.Fatalf("second Install failed: %v", err)
	}
}

func TestCursorSessionSetGetUnset(t *testing.T) {
	dir := testutil.InitRepo(t)

	if err := Bridge([]string{"cursor", "session", "set", "chat-abc"}); err != nil {
		t.Fatalf("session set failed: %v", err)
	}
	if got := testutil.RunGit(t, dir, "config", "--get", "agit.cursorSession"); strings.TrimSpace(got) != "chat-abc" {
		t.Errorf("agit.cursorSession = %q, want chat-abc", got)
	}

	if err := Bridge([]string{"cursor", "session", "get"}); err != nil {
		t.Fatalf("session get failed: %v", err)
	}
	if err := Bridge([]string{"cursor", "session", "unset"}); err != nil {
		t.Fatalf("session unset failed: %v", err)
	}

	if err := Bridge([]string{"cursor", "session"}); err != nil {
		t.Fatalf("session without subcommand failed: %v", err)
	}
}

func TestCursorProvideWithoutCLI(t *testing.T) {
	testutil.InitRepo(t)
	t.Setenv("PATH", pathWithGitOnly(t))

	b, err := GetBridge("cursor")
	if err != nil {
		t.Fatal(err)
	}
	_, err = b.Provide()
	if err == nil || !strings.Contains(err.Error(), "Cursor context bridge failed") {
		t.Errorf("expected Cursor context bridge failure, got: %v", err)
	}
}

func TestCursorFeedWithoutCLI(t *testing.T) {
	dir := testutil.InitRepo(t)
	t.Setenv("PATH", pathWithGitOnly(t))

	head := testutil.CommitFile(t, dir, "feat.go", "package x", "feature work")
	addAgitNote(t, dir, head, testCheckoutNote)
	if _, err := RestoreHEADContext(); err != nil {
		t.Fatalf("RestoreHEADContext failed: %v", err)
	}

	b, err := GetBridge("cursor")
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := LoadCheckoutContext()
	if err != nil {
		t.Fatal(err)
	}
	err = b.Feed(ctx)
	if err == nil || !strings.Contains(err.Error(), "Cursor checkout feed failed") {
		t.Errorf("expected Cursor checkout feed failure, got: %v", err)
	}
}

func TestActiveBridgeCursor(t *testing.T) {
	dir := testutil.InitRepo(t)
	testutil.RunGit(t, dir, "config", "agit.bridge", "cursor")

	b, err := ActiveBridge()
	if err != nil {
		t.Fatalf("ActiveBridge failed: %v", err)
	}
	if b.Name() != "cursor" {
		t.Errorf("bridge = %q, want cursor", b.Name())
	}
}
