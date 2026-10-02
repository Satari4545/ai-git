package agit

import (
	"strings"
	"testing"

	"github.com/Satari4545/ai-git/internal/testutil"
)

func TestGeminiBridgeRegistered(t *testing.T) {
	b, err := GetBridge("gemini")
	if err != nil {
		t.Fatalf("gemini bridge not registered: %v", err)
	}
	if b.Name() != "gemini" {
		t.Errorf("Name = %q, want gemini", b.Name())
	}
	if b.DisplayName() != "Gemini CLI" {
		t.Errorf("DisplayName = %q, want Gemini CLI", b.DisplayName())
	}

	found := false
	for _, name := range BridgeNames() {
		if name == "gemini" {
			found = true
		}
	}
	if !found {
		t.Errorf("BridgeNames missing gemini: %v", BridgeNames())
	}
}

func TestGeminiInstall(t *testing.T) {
	dir := testutil.InitRepo(t)
	t.Setenv("HOME", t.TempDir())

	b, err := GetBridge("gemini")
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Install(); err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	if got := testutil.RunGit(t, dir, "config", "--get", "agit.contextCommand"); strings.TrimSpace(got) != "agit bridge gemini provider" {
		t.Errorf("agit.contextCommand = %q", strings.TrimSpace(got))
	}
	if got := testutil.RunGit(t, dir, "config", "--get", "agit.bridge"); strings.TrimSpace(got) != "gemini" {
		t.Errorf("agit.bridge = %q", strings.TrimSpace(got))
	}

	if err := b.Install(); err != nil {
		t.Fatalf("second Install failed: %v", err)
	}
}

func TestGeminiSessionSetGetUnset(t *testing.T) {
	dir := testutil.InitRepo(t)

	if err := Bridge([]string{"gemini", "session", "set", "sess-uuid-1"}); err != nil {
		t.Fatalf("session set failed: %v", err)
	}
	if got := testutil.RunGit(t, dir, "config", "--get", "agit.geminiSession"); strings.TrimSpace(got) != "sess-uuid-1" {
		t.Errorf("agit.geminiSession = %q, want sess-uuid-1", got)
	}

	if err := Bridge([]string{"gemini", "session", "get"}); err != nil {
		t.Fatalf("session get failed: %v", err)
	}
	if err := Bridge([]string{"gemini", "session", "unset"}); err != nil {
		t.Fatalf("session unset failed: %v", err)
	}

	if err := Bridge([]string{"gemini", "session"}); err != nil {
		t.Fatalf("session without subcommand failed: %v", err)
	}
}

func TestGeminiProvideWithoutCLI(t *testing.T) {
	testutil.InitRepo(t)
	t.Setenv("PATH", pathWithGitOnly(t))

	b, err := GetBridge("gemini")
	if err != nil {
		t.Fatal(err)
	}
	_, err = b.Provide()
	if err == nil || !strings.Contains(err.Error(), "Gemini CLI context bridge failed") {
		t.Errorf("expected Gemini CLI context bridge failure, got: %v", err)
	}
}

func TestGeminiFeedWithoutCLI(t *testing.T) {
	dir := testutil.InitRepo(t)
	t.Setenv("PATH", pathWithGitOnly(t))

	head := testutil.CommitFile(t, dir, "feat.go", "package x", "feature work")
	addAgitNote(t, dir, head, testCheckoutNote)
	if _, err := RestoreHEADContext(); err != nil {
		t.Fatalf("RestoreHEADContext failed: %v", err)
	}

	b, err := GetBridge("gemini")
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := LoadCheckoutContext()
	if err != nil {
		t.Fatal(err)
	}
	err = b.Feed(ctx)
	if err == nil || !strings.Contains(err.Error(), "Gemini CLI checkout feed failed") {
		t.Errorf("expected Gemini CLI checkout feed failure, got: %v", err)
	}
}

func TestActiveBridgeGemini(t *testing.T) {
	dir := testutil.InitRepo(t)
	testutil.RunGit(t, dir, "config", "agit.bridge", "gemini")

	b, err := ActiveBridge()
	if err != nil {
		t.Fatalf("ActiveBridge failed: %v", err)
	}
	if b.Name() != "gemini" {
		t.Errorf("bridge = %q, want gemini", b.Name())
	}
}
