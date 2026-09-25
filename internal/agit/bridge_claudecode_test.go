package agit

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Satari4545/ai-git/internal/testutil"
)

func TestClaudeCodeBridgeRegistered(t *testing.T) {
	b, err := GetBridge("claude-code")
	if err != nil {
		t.Fatalf("claude-code bridge not registered: %v", err)
	}
	if b.Name() != "claude-code" {
		t.Errorf("Name = %q, want claude-code", b.Name())
	}
	if b.DisplayName() != "Claude Code" {
		t.Errorf("DisplayName = %q, want Claude Code", b.DisplayName())
	}

	found := false
	for _, name := range BridgeNames() {
		if name == "claude-code" {
			found = true
		}
	}
	if !found {
		t.Errorf("BridgeNames missing claude-code: %v", BridgeNames())
	}
}

// writeClaudeStub installs a fake `claude` executable on PATH that ignores
// its arguments and prints output verbatim.
func writeClaudeStub(t *testing.T, output string) (argsPath string) {
	t.Helper()

	dir := t.TempDir()
	argsPath = filepath.Join(dir, "args")
	script := "#!/bin/sh\necho \"$@\" > " + argsPath + "\nprintf '%s' '" + output + "'\n"
	if err := os.WriteFile(filepath.Join(dir, "claude"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return argsPath
}

func TestClaudeCodeInstall(t *testing.T) {
	dir := testutil.InitRepo(t)
	home := t.TempDir()
	t.Setenv("HOME", home)

	b, err := GetBridge("claude-code")
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Install(); err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	if got := testutil.RunGit(t, dir, "config", "--get", "agit.contextCommand"); strings.TrimSpace(got) != "agit bridge claude-code provider" {
		t.Errorf("agit.contextCommand = %q", strings.TrimSpace(got))
	}
	if got := testutil.RunGit(t, dir, "config", "--get", "agit.bridge"); strings.TrimSpace(got) != "claude-code" {
		t.Errorf("agit.bridge = %q", strings.TrimSpace(got))
	}

	settingsPath := filepath.Join(home, ".claude", "settings.json")
	raw, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("settings.json not written: %v", err)
	}
	if strings.Count(string(raw), "agit bridge claude-code hook") != 1 {
		t.Errorf("expected exactly one hook registration, got:\n%s", raw)
	}
	var settings map[string]any
	if err := json.Unmarshal(raw, &settings); err != nil {
		t.Fatalf("settings.json is not valid JSON: %v", err)
	}

	// Install is idempotent: a second run must not duplicate the hook.
	if err := b.Install(); err != nil {
		t.Fatalf("second Install failed: %v", err)
	}
	raw, err = os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(raw), "agit bridge claude-code hook") != 1 {
		t.Errorf("Install is not idempotent:\n%s", raw)
	}
}

func TestClaudeCodeInstallPreservesExistingSettings(t *testing.T) {
	testutil.InitRepo(t)
	home := t.TempDir()
	t.Setenv("HOME", home)

	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	existing := `{"model": "opus", "hooks": {"SessionStart": []}}`
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	b, err := GetBridge("claude-code")
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Install(); err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var settings map[string]any
	if err := json.Unmarshal(raw, &settings); err != nil {
		t.Fatalf("settings.json is not valid JSON: %v", err)
	}
	if settings["model"] != "opus" {
		t.Errorf("existing settings key lost: %v", settings)
	}
	if strings.Count(string(raw), "agit bridge claude-code hook") != 1 {
		t.Errorf("hook not merged into existing hooks:\n%s", raw)
	}
}

func TestClaudeCodeInstallRefusesCorruptSettings(t *testing.T) {
	testutil.InitRepo(t)
	home := t.TempDir()
	t.Setenv("HOME", home)

	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	b, err := GetBridge("claude-code")
	if err != nil {
		t.Fatal(err)
	}
	err = b.Install()
	if err == nil || !strings.Contains(err.Error(), "settings.json") {
		t.Errorf("expected error naming settings.json, got: %v", err)
	}
}

func TestClaudeCodeProvide(t *testing.T) {
	testutil.InitRepo(t)
	argsPath := writeClaudeStub(t, `{"type":"result","result":"{\"intent\":\"wire the hook\"}"}`)

	b, err := GetBridge("claude-code")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := b.Provide()
	if err != nil {
		t.Fatalf("Provide failed: %v", err)
	}
	if !strings.Contains(string(raw), `"intent":"wire the hook"`) {
		t.Errorf("Provide did not extract the result JSON: %s", raw)
	}

	args, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(args), "-p") || !strings.Contains(string(args), "--output-format") {
		t.Errorf("claude was not invoked in print mode: %s", args)
	}
}

func TestClaudeCodeProvideErrors(t *testing.T) {
	testutil.InitRepo(t)

	b, err := GetBridge("claude-code")
	if err != nil {
		t.Fatal(err)
	}

	writeClaudeStub(t, `not json at all`)
	if _, err := b.Provide(); err == nil {
		t.Error("expected error for non-JSON envelope, got nil")
	}

	writeClaudeStub(t, `{"type":"result","result":""}`)
	if _, err := b.Provide(); err == nil {
		t.Error("expected error for empty result, got nil")
	}

	writeClaudeStub(t, `{"type":"result","result":"just prose, no object"}`)
	if _, err := b.Provide(); err == nil {
		t.Error("expected error when result has no JSON object, got nil")
	}
}

func TestClaudeCodeFeedWritesContextFile(t *testing.T) {
	dir := testutil.InitRepo(t)

	head := testutil.CommitFile(t, dir, "feat.go", "package x", "feature work")
	addAgitNote(t, dir, head, testCheckoutNote)
	if _, err := RestoreHEADContext(); err != nil {
		t.Fatalf("RestoreHEADContext failed: %v", err)
	}

	b, err := GetBridge("claude-code")
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := LoadCheckoutContext()
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Feed(ctx); err != nil {
		t.Fatalf("Feed failed: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(".agit", "state", "claude-code-feed.md"))
	if err != nil {
		t.Fatalf("feed file not written: %v", err)
	}
	if !strings.Contains(string(raw), "work on feature branch") {
		t.Errorf("feed file missing intent:\n%s", raw)
	}
	if !strings.Contains(string(raw), head) {
		t.Errorf("feed file missing commit %s:\n%s", head, raw)
	}
}

func TestClaudeCodeHookRendersContext(t *testing.T) {
	dir := testutil.InitRepo(t)

	// No restored context: the hook must stay silent, not inject stale text.
	out, err := renderClaudeCodeHookContext()
	if err != nil {
		t.Fatalf("hook render failed: %v", err)
	}
	if out != "" {
		t.Errorf("expected empty hook output without context, got %q", out)
	}

	head := testutil.CommitFile(t, dir, "feat.go", "package x", "feature work")
	addAgitNote(t, dir, head, testCheckoutNote)
	if _, err := RestoreHEADContext(); err != nil {
		t.Fatalf("RestoreHEADContext failed: %v", err)
	}

	out, err = renderClaudeCodeHookContext()
	if err != nil {
		t.Fatalf("hook render failed: %v", err)
	}
	if !strings.Contains(out, "ai-git") {
		t.Errorf("hook output missing ai-git marker:\n%s", out)
	}
	if !strings.Contains(out, "work on feature branch") {
		t.Errorf("hook output missing intent:\n%s", out)
	}
}

func TestBridgeDispatchClaudeCodeHook(t *testing.T) {
	dir := testutil.InitRepo(t)

	// Silent when there is no context.
	if err := Bridge([]string{"claude-code", "hook"}); err != nil {
		t.Fatalf("hook dispatch failed: %v", err)
	}

	head := testutil.CommitFile(t, dir, "feat.go", "package x", "feature work")
	addAgitNote(t, dir, head, testCheckoutNote)
	if _, err := RestoreHEADContext(); err != nil {
		t.Fatalf("RestoreHEADContext failed: %v", err)
	}
	if err := Bridge([]string{"claude-code", "hook"}); err != nil {
		t.Fatalf("hook dispatch with context failed: %v", err)
	}
}

func TestActiveBridgeClaudeCode(t *testing.T) {
	dir := testutil.InitRepo(t)
	testutil.RunGit(t, dir, "config", "agit.bridge", "claude-code")

	b, err := ActiveBridge()
	if err != nil {
		t.Fatalf("ActiveBridge failed: %v", err)
	}
	if b.Name() != "claude-code" {
		t.Errorf("bridge = %q, want claude-code", b.Name())
	}
}

func TestClaudeCodeInstallNullSettings(t *testing.T) {
	testutil.InitRepo(t)
	home := t.TempDir()
	t.Setenv("HOME", home)

	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Valid JSON, but not an object: must be a clean error, not a panic.
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte("null"), 0o644); err != nil {
		t.Fatal(err)
	}

	b, err := GetBridge("claude-code")
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Install(); err == nil || !strings.Contains(err.Error(), "settings.json") {
		t.Errorf("expected error naming settings.json, got: %v", err)
	}
}

func TestClaudeCodeInstallAllOrNothing(t *testing.T) {
	testutil.InitRepo(t) // chdirs into the temp repo; git commands below run there
	home := t.TempDir()
	t.Setenv("HOME", home)

	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	b, err := GetBridge("claude-code")
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Install(); err == nil {
		t.Fatal("expected Install to fail on corrupt settings, got nil")
	}

	// A failed hook install must not leave git config half-installed.
	// `git config --get` exits 1 for unset keys, which the test helper
	// treats as fatal, so shell out directly here.
	for _, key := range []string{"agit.contextCommand", "agit.bridge"} {
		out, err := exec.Command("git", "config", "--get", key).Output()
		if err == nil {
			t.Errorf("%s should be unset after failed install, got %q", key, strings.TrimSpace(string(out)))
		} else if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() != 1 {
			t.Errorf("unexpected error reading %s: %v", key, err)
		}
	}
}
