package agit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Satari4545/ai-git/internal/testutil"
)

const testCheckoutNote = `{
  "contextID": "ctx_sha256:checkout123",
  "logicState": {
    "schemaVersion": "agit.logic-delta.v0",
    "agentThinking": {"intentSummary": "work on feature branch"},
    "codeReality": {"activeFiles": ["feat.go"]}
  }
}`

func TestStripCheckoutAgitArgs(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantGit      []string
		wantOverride string
	}{
		{"feed override", []string{"--feed-context", "main"}, []string{"main"}, "enabled"},
		{"no-feed override", []string{"--no-feed-context", "-q", "main"}, []string{"-q", "main"}, "disabled"},
		{"passthrough untouched", []string{"-b", "new-branch"}, []string{"-b", "new-branch"}, ""},
		{"empty", nil, []string{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gitArgs, override := stripCheckoutAgitArgs(tt.args)
			if !reflect.DeepEqual(gitArgs, tt.wantGit) {
				t.Errorf("gitArgs = %v, want %v", gitArgs, tt.wantGit)
			}
			if override != tt.wantOverride {
				t.Errorf("override = %q, want %q", override, tt.wantOverride)
			}
		})
	}
}

func TestShouldFeedCheckoutContextOverrides(t *testing.T) {
	if ok, err := ShouldFeedCheckoutContext("enabled"); err != nil || !ok {
		t.Errorf("override enabled: ok=%v err=%v, want true", ok, err)
	}
	if ok, err := ShouldFeedCheckoutContext("disabled"); err != nil || ok {
		t.Errorf("override disabled: ok=%v err=%v, want false", ok, err)
	}
}

func TestShouldFeedCheckoutContextConfigModes(t *testing.T) {
	dir := testutil.InitRepo(t)

	testutil.RunGit(t, dir, "config", "agit.checkoutFeed", "true")
	if ok, err := ShouldFeedCheckoutContext(""); err != nil || !ok {
		t.Errorf("config true: ok=%v err=%v, want true", ok, err)
	}

	testutil.RunGit(t, dir, "config", "agit.checkoutFeed", "false")
	if ok, err := ShouldFeedCheckoutContext(""); err != nil || ok {
		t.Errorf("config false: ok=%v err=%v, want false", ok, err)
	}
}

func TestCheckoutFeedModeMappings(t *testing.T) {
	dir := testutil.InitRepo(t)

	tests := []struct {
		value string
		want  string
	}{
		{"true", "enabled"},
		{"enable", "enabled"},
		{"enabled", "enabled"},
		{"false", "disabled"},
		{"disable", "disabled"},
		{"disabled", "disabled"},
		{"prompt", "prompt"},
		{"unexpected-value", "prompt"},
	}

	for _, tt := range tests {
		testutil.RunGit(t, dir, "config", "agit.checkoutFeed", tt.value)
		if got := CheckoutFeedMode(); got != tt.want {
			t.Errorf("CheckoutFeedMode(%q) = %q, want %q", tt.value, got, tt.want)
		}
	}

	testutil.RunGit(t, dir, "config", "--unset", "agit.checkoutFeed")
	if got := CheckoutFeedMode(); got != "prompt" {
		t.Errorf("CheckoutFeedMode(unset) = %q, want prompt", got)
	}
}

func TestRestoreHEADContextWithNote(t *testing.T) {
	dir := testutil.InitRepo(t)

	head := testutil.CommitFile(t, dir, "feat.go", "package x", "feature work")
	addAgitNote(t, dir, head, testCheckoutNote)

	restored, err := RestoreHEADContext()
	if err != nil {
		t.Fatalf("RestoreHEADContext failed: %v", err)
	}
	if !restored {
		t.Fatal("expected restored=true when HEAD has a note")
	}

	raw, err := os.ReadFile(filepath.Join(".agit", "state", "checkout.json"))
	if err != nil {
		t.Fatalf("checkout.json was not written: %v", err)
	}

	var envelope RestoredCheckoutContext
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("checkout.json is not valid JSON: %v", err)
	}
	if envelope.SchemaVersion != "agit.checkout-context.v0" {
		t.Errorf("SchemaVersion = %q", envelope.SchemaVersion)
	}
	if envelope.Commit != head {
		t.Errorf("Commit = %q, want HEAD %q", envelope.Commit, head)
	}
	if envelope.RestoredAt == "" {
		t.Error("RestoredAt was not stamped")
	}
	if !strings.Contains(string(envelope.Note), "ctx_sha256:checkout123") {
		t.Errorf("Note does not contain the original context ID: %s", envelope.Note)
	}
}

func TestRestoreHEADContextNoNote(t *testing.T) {
	dir := testutil.InitRepo(t)
	testutil.CommitFile(t, dir, "plain.go", "package x", "no note here")

	restored, err := RestoreHEADContext()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if restored {
		t.Error("expected restored=false when HEAD has no note")
	}
	if _, err := os.Stat(filepath.Join(".agit", "state", "checkout.json")); !os.IsNotExist(err) {
		t.Error("checkout.json should not exist when nothing was restored")
	}
}

func TestRestoreHEADContextInvalidNote(t *testing.T) {
	dir := testutil.InitRepo(t)

	head := testutil.CommitFile(t, dir, "bad.go", "package x", "bad note")
	addAgitNote(t, dir, head, "{not valid json")

	_, err := RestoreHEADContext()
	if err == nil || !strings.Contains(err.Error(), "invalid AGIT note") {
		t.Errorf("expected invalid-note error, got: %v", err)
	}
}

func TestCheckoutRestoresContextEndToEnd(t *testing.T) {
	dir := testutil.InitRepo(t)
	t.Setenv("AGIT_MCP_SILENT", "1")
	testutil.RunGit(t, dir, "config", "agit.bridge", "codex")
	stubDir := writeCodexStub(t, "")

	head := testutil.CommitFile(t, dir, "feat.go", "package x", "feature work")
	addAgitNote(t, dir, head, testCheckoutNote)

	testutil.RunGit(t, dir, "checkout", "-qb", "other")
	testutil.CommitFile(t, dir, "other.go", "package x", "other work")

	if err := Checkout([]string{"--feed-context", "-q", "-"}); err != nil {
		t.Fatalf("Checkout failed: %v", err)
	}

	raw, ok, err := LoadCheckoutContextRaw()
	if err != nil || !ok {
		t.Fatalf("expected checkout context after approved feed, ok=%v err=%v", ok, err)
	}
	var envelope RestoredCheckoutContext
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("checkout.json invalid: %v", err)
	}
	if envelope.Commit != head {
		t.Errorf("restored commit = %q, want %q", envelope.Commit, head)
	}

	// The real codex bridge must have been fed the restored context: the
	// stubbed `codex` binary records the exact invocation.
	args := readCodexStubArgs(t, stubDir)
	if !strings.Contains(args, head) {
		t.Errorf("codex feed missing restored commit %s:\n%s", head, args)
	}
}

func TestCheckoutNoFeedDiscardsState(t *testing.T) {
	dir := testutil.InitRepo(t)
	t.Setenv("AGIT_MCP_SILENT", "1")

	head := testutil.CommitFile(t, dir, "feat.go", "package x", "feature work")
	addAgitNote(t, dir, head, testCheckoutNote)

	testutil.RunGit(t, dir, "checkout", "-qb", "other")
	testutil.CommitFile(t, dir, "other.go", "package x", "other work")

	// Declining the feed must not leave feedable state behind for
	// agent hooks or manual feed verbs.
	if err := Checkout([]string{"--no-feed-context", "-q", "-"}); err != nil {
		t.Fatalf("Checkout failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(".agit", "state", "checkout.json")); !os.IsNotExist(err) {
		t.Error("checkout.json should not exist after a declined feed")
	}
}

func TestRestoreHEADContextClearsStaleState(t *testing.T) {
	dir := testutil.InitRepo(t)

	head := testutil.CommitFile(t, dir, "feat.go", "package x", "feature work")
	addAgitNote(t, dir, head, testCheckoutNote)
	if _, err := RestoreHEADContext(); err != nil {
		t.Fatalf("RestoreHEADContext failed: %v", err)
	}
	if _, ok, _ := LoadCheckoutContextRaw(); !ok {
		t.Fatal("expected checkout state after restore")
	}

	// Move to a commit with no note: the stale state must be cleared so
	// hooks and manual feeds never see the previous checkout.
	testutil.RunGit(t, dir, "checkout", "-qb", "other")
	testutil.CommitFile(t, dir, "other.go", "package x", "other work")
	restored, err := RestoreHEADContext()
	if err != nil {
		t.Fatalf("RestoreHEADContext failed: %v", err)
	}
	if restored {
		t.Error("expected restored=false on a note-less HEAD")
	}
	if _, err := os.Stat(filepath.Join(".agit", "state", "checkout.json")); !os.IsNotExist(err) {
		t.Error("stale checkout.json was not cleared")
	}
}

func TestCheckoutWithoutNoteWritesNothing(t *testing.T) {
	dir := testutil.InitRepo(t)

	testutil.CommitFile(t, dir, "a.go", "package x", "first")
	testutil.RunGit(t, dir, "checkout", "-qb", "second")
	testutil.CommitFile(t, dir, "b.go", "package x", "second")

	if err := Checkout([]string{"--no-feed-context", "-q", "second"}); err != nil {
		t.Fatalf("Checkout failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(".agit", "state", "checkout.json")); !os.IsNotExist(err) {
		t.Error("checkout.json should not exist when HEAD has no note")
	}
}

func TestLoadCheckoutContextRawMissing(t *testing.T) {
	testutil.InitRepo(t)

	raw, ok, err := LoadCheckoutContextRaw()
	if err != nil || ok || raw != nil {
		t.Errorf("expected (nil, false, nil), got (%v, %v, %v)", raw, ok, err)
	}
}
