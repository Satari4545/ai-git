package agit

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
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
	if !ShouldFeedCheckoutContext("enabled") {
		t.Error("override enabled: want true")
	}
	if ShouldFeedCheckoutContext("disabled") {
		t.Error("override disabled: want false")
	}
}

func TestShouldFeedCheckoutContextConfigModes(t *testing.T) {
	dir := testutil.InitRepo(t)

	testutil.RunGit(t, dir, "config", "agit.checkoutFeed", "true")
	if !ShouldFeedCheckoutContext("") {
		t.Error("config true: want true")
	}

	testutil.RunGit(t, dir, "config", "agit.checkoutFeed", "false")
	if ShouldFeedCheckoutContext("") {
		t.Error("config false: want false")
	}

	testutil.RunGit(t, dir, "config", "--unset", "agit.checkoutFeed")
	if !ShouldFeedCheckoutContext("") {
		t.Error("unset config: want true, auto is the default")
	}
}

func TestShouldFeedCheckoutContextPromptSkipsWithoutTTY(t *testing.T) {
	dir := testutil.InitRepo(t)
	testutil.RunGit(t, dir, "config", "agit.checkoutFeed", "prompt")

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	oldStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()

	if ShouldFeedCheckoutContext("") {
		t.Error("prompt mode without a TTY must skip the feed instead of hanging")
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
		{"unexpected-value", "enabled"},
	}

	for _, tt := range tests {
		testutil.RunGit(t, dir, "config", "agit.checkoutFeed", tt.value)
		if got := CheckoutFeedMode(); got != tt.want {
			t.Errorf("CheckoutFeedMode(%q) = %q, want %q", tt.value, got, tt.want)
		}
	}

	testutil.RunGit(t, dir, "config", "--unset", "agit.checkoutFeed")
	if got := CheckoutFeedMode(); got != "enabled" {
		t.Errorf("CheckoutFeedMode(unset) = %q, want enabled", got)
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

	args := readCodexStubArgs(t, stubDir)
	if !strings.Contains(args, head) {
		t.Errorf("codex feed missing restored commit %s:\n%s", head, args)
	}
}

func TestCheckoutNoFeedKeepsState(t *testing.T) {
	dir := testutil.InitRepo(t)
	t.Setenv("AGIT_MCP_SILENT", "1")

	head := testutil.CommitFile(t, dir, "feat.go", "package x", "feature work")
	addAgitNote(t, dir, head, testCheckoutNote)

	testutil.RunGit(t, dir, "checkout", "-qb", "other")
	testutil.CommitFile(t, dir, "other.go", "package x", "other work")

	if err := Checkout([]string{"--no-feed-context", "-q", "-"}); err != nil {
		t.Fatalf("Checkout failed: %v", err)
	}

	raw, ok, err := LoadCheckoutContextRaw()
	if err != nil || !ok {
		t.Fatalf("declining the feed must keep the restored context, ok=%v err=%v", ok, err)
	}
	var envelope RestoredCheckoutContext
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("checkout.json invalid: %v", err)
	}
	if envelope.Commit != head {
		t.Errorf("restored commit = %q, want %q", envelope.Commit, head)
	}
}

func TestCheckoutFailedFeedWarnsWithoutFailing(t *testing.T) {
	dir := testutil.InitRepo(t)
	t.Setenv("AGIT_MCP_SILENT", "1")
	testutil.RunGit(t, dir, "config", "agit.bridge", "codex")
	writeFailingCodexStub(t)

	head := testutil.CommitFile(t, dir, "feat.go", "package x", "feature work")
	addAgitNote(t, dir, head, testCheckoutNote)

	testutil.RunGit(t, dir, "checkout", "-qb", "other")
	testutil.CommitFile(t, dir, "other.go", "package x", "other work")

	stderr := captureStderr(t, func() {
		if err := Checkout([]string{"--feed-context", "-q", "-"}); err != nil {
			t.Fatalf("a failed feed must not fail checkout, got: %v", err)
		}
	})

	if !strings.Contains(stderr, "warning") {
		t.Errorf("expected a stderr warning about the failed feed, got: %q", stderr)
	}
	if _, ok, err := LoadCheckoutContextRaw(); err != nil || !ok {
		t.Errorf("checkout.json must survive a failed feed, ok=%v err=%v", ok, err)
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

func TestCheckoutBinaryAutoFeedNeverPrompts(t *testing.T) {
	bin := buildAgitBinary(t)
	t.Setenv("AGIT_MCP_SILENT", "1")

	t.Run("failed feed warns, exits zero, keeps state", func(t *testing.T) {
		dir, head := initBinaryTestRepo(t)
		writeFailingCodexStub(t)

		stdout, stderr, err := runAgitBinary(t, bin, dir, "checkout", "-q", "-")
		if err != nil {
			t.Fatalf("agit checkout exited non-zero: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
		}
		if strings.Contains(stdout, "Feed restored AGIT context") {
			t.Errorf("checkout must not prompt in auto mode, stdout: %q", stdout)
		}
		if !strings.Contains(stderr, "warning") {
			t.Errorf("expected a stderr warning for the failed feed, stderr: %q", stderr)
		}
		assertBinaryCheckoutState(t, dir, head)
	})

	t.Run("no-feed-context keeps state without feeding", func(t *testing.T) {
		dir, head := initBinaryTestRepo(t)

		stdout, stderr, err := runAgitBinary(t, bin, dir, "checkout", "--no-feed-context", "-q", "-")
		if err != nil {
			t.Fatalf("agit checkout exited non-zero: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
		}
		if strings.Contains(stdout, "Feed restored AGIT context") {
			t.Errorf("checkout must not prompt, stdout: %q", stdout)
		}
		if strings.Contains(stderr, "warning") {
			t.Errorf("no feed was attempted, so no warning is expected, stderr: %q", stderr)
		}
		assertBinaryCheckoutState(t, dir, head)
	})
}

func buildAgitBinary(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate the test source file")
	}
	root := filepath.Join(filepath.Dir(file), "..", "..")
	out := filepath.Join(t.TempDir(), "agit")
	cmd := exec.Command("go", "build", "-o", out, "./cmd/agit")
	cmd.Dir = root
	if combined, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build ./cmd/agit failed: %v\n%s", err, combined)
	}
	return out
}

func initBinaryTestRepo(t *testing.T) (string, string) {
	t.Helper()

	dir := t.TempDir()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	runGit := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
		return string(out)
	}

	runGit("init", "-q")
	runGit("config", "user.email", "agit-test@example.com")
	runGit("config", "user.name", "agit-test")
	testutil.WriteFile(t, filepath.Join(dir, "feat.go"), "package x\n")
	runGit("add", "feat.go")
	runGit("commit", "-qm", "feature work")
	head := strings.TrimSpace(runGit("rev-parse", "HEAD"))
	noteFile := filepath.Join(dir, "note.json")
	testutil.WriteFile(t, noteFile, testCheckoutNote)
	runGit("notes", "--ref=agit", "add", "-F", noteFile, head)
	if err := os.Remove(noteFile); err != nil {
		t.Fatal(err)
	}
	runGit("checkout", "-qb", "other")
	testutil.WriteFile(t, filepath.Join(dir, "other.go"), "package x\n")
	runGit("add", "other.go")
	runGit("commit", "-qm", "other work")
	return dir, head
}

func runAgitBinary(t *testing.T, bin, dir string, args ...string) (string, string, error) {
	t.Helper()

	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer devNull.Close()
	cmd.Stdin = devNull
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	return stdout.String(), stderr.String(), err
}

func assertBinaryCheckoutState(t *testing.T, dir, head string) {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join(dir, ".agit", "state", "checkout.json"))
	if err != nil {
		t.Fatalf("checkout.json missing after checkout: %v", err)
	}
	var envelope RestoredCheckoutContext
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("checkout.json invalid: %v", err)
	}
	if envelope.Commit != head {
		t.Errorf("restored commit = %q, want %q", envelope.Commit, head)
	}
}

func writeFailingCodexStub(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	script := "#!/bin/sh\necho codex feed failed >&2\nexit 1\n"
	if err := os.WriteFile(filepath.Join(dir, "codex"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	defer func() { os.Stderr = old }()

	fn()

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}
