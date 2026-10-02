package agit

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Satari4545/ai-git/internal/testutil"
)

// writeCodexStub installs a fake `codex` executable on PATH and returns the
// stub directory. The CLI binary is the only thing stubbed in the bridge
// tests: the real Codex CLI needs authentication and makes network calls,
// so tests replace the binary itself while dispatch, Feed argument
// building, Provide output parsing, and provider normalization all run for
// real.
//
// Invoked arguments are recorded at <dir>/args. When the invocation carries
// --output-last-message <path> (the Provide path), provideJSON is written to
// that path, mimicking `codex exec resume --output-last-message`.
func writeCodexStub(t *testing.T, provideJSON string) string {
	t.Helper()

	dir := t.TempDir()
	argsPath := filepath.Join(dir, "args")
	script := "#!/bin/sh\n" +
		"echo \"$@\" > \"" + argsPath + "\"\n" +
		"out=\"\"\n" +
		"prev=\"\"\n" +
		"for a in \"$@\"; do\n" +
		"  if [ \"$prev\" = \"--output-last-message\" ]; then out=\"$a\"; fi\n" +
		"  prev=\"$a\"\n" +
		"done\n" +
		"if [ -n \"$out\" ]; then\n" +
		"  printf '%s' '" + provideJSON + "' > \"$out\"\n" +
		"fi\n"
	if err := os.WriteFile(filepath.Join(dir, "codex"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return dir
}

// readCodexStubArgs returns the arguments the fake `codex` binary was
// invoked with, failing the test if it was never invoked.
func readCodexStubArgs(t *testing.T, stubDir string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(stubDir, "args"))
	if err != nil {
		t.Fatalf("codex stub was not invoked: %v", err)
	}
	return string(raw)
}

// captureStdout runs fn with os.Stdout redirected to a pipe and returns
// what fn printed. It lets tests assert on the real provider output
// without replacing any component.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

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

func TestBridgeRegistryRoundTrip(t *testing.T) {
	b, err := GetBridge("codex")
	if err != nil {
		t.Fatalf("GetBridge(codex) failed: %v", err)
	}
	if b.Name() != "codex" {
		t.Errorf("Name = %q, want codex", b.Name())
	}

	if _, err := GetBridge("does-not-exist"); err == nil {
		t.Error("expected error for unknown bridge, got nil")
	}
}

func TestBridgeNamesSorted(t *testing.T) {
	names := BridgeNames()
	found := map[string]bool{}
	for _, n := range names {
		found[n] = true
	}
	for _, want := range []string{"codex", "claude-code"} {
		if !found[want] {
			t.Errorf("BridgeNames missing %q: %v", want, names)
		}
	}
	for i := 1; i < len(names); i++ {
		if names[i] < names[i-1] {
			t.Errorf("BridgeNames not sorted: %v", names)
			break
		}
	}
}

func TestCodexBridgeRegistered(t *testing.T) {
	b, err := GetBridge("codex")
	if err != nil {
		t.Fatalf("codex bridge not registered: %v", err)
	}
	if b.DisplayName() == "" {
		t.Error("codex bridge has empty display name")
	}
}

func TestBridgeDispatchInstall(t *testing.T) {
	dir := testutil.InitRepo(t)

	if err := Bridge([]string{"codex", "install"}); err != nil {
		t.Fatalf("Bridge install failed: %v", err)
	}
	if got := testutil.RunGit(t, dir, "config", "--get", "agit.contextCommand"); strings.TrimSpace(got) != "agit bridge codex provider" {
		t.Errorf("agit.contextCommand = %q", strings.TrimSpace(got))
	}
	if got := testutil.RunGit(t, dir, "config", "--get", "agit.bridge"); strings.TrimSpace(got) != "codex" {
		t.Errorf("agit.bridge = %q", strings.TrimSpace(got))
	}
}

func TestBridgeDispatchUnknownAgent(t *testing.T) {
	err := Bridge([]string{"no-such-agent", "install"})
	if err == nil || !strings.Contains(err.Error(), "no-such-agent") {
		t.Errorf("expected error naming the agent, got: %v", err)
	}
}

func TestBridgeDispatchNoArgsPrintsUsage(t *testing.T) {
	if err := Bridge(nil); err != nil {
		t.Errorf("expected nil error for bare 'bridge', got: %v", err)
	}
}

func TestBridgeDispatchFeed(t *testing.T) {
	dir := testutil.InitRepo(t)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_HOME", "")
	stubDir := writeCodexStub(t, "")

	head := testutil.CommitFile(t, dir, "feat.go", "package x", "feature work")
	addAgitNote(t, dir, head, testCheckoutNote)
	if _, err := RestoreHEADContext(); err != nil {
		t.Fatalf("RestoreHEADContext failed: %v", err)
	}

	if err := Bridge([]string{"codex", "feed"}); err != nil {
		t.Fatalf("Bridge feed failed: %v", err)
	}

	args := readCodexStubArgs(t, stubDir)
	for _, want := range []string{"exec", "resume", "--last"} {
		if !strings.Contains(args, want) {
			t.Errorf("codex stub args missing %q: %s", want, args)
		}
	}
	if !strings.Contains(args, head) {
		t.Errorf("feed prompt missing restored commit %s:\n%s", head, args)
	}
	if !strings.Contains(args, "work on feature branch") {
		t.Errorf("feed prompt missing restored intent:\n%s", args)
	}
}

func TestBridgeDispatchFeedWithoutContext(t *testing.T) {
	testutil.InitRepo(t)
	writeCodexStub(t, "")

	err := Bridge([]string{"codex", "feed"})
	if err == nil || !strings.Contains(err.Error(), "no checkout context") {
		t.Errorf("expected no-context error, got: %v", err)
	}
}

func TestBridgeDispatchProviderNormalizes(t *testing.T) {
	testutil.InitRepo(t)
	stubDir := writeCodexStub(t, `{"intent":"stubbed intent"}`)

	out := captureStdout(t, func() {
		if err := Bridge([]string{"codex", "provider"}); err != nil {
			t.Errorf("Bridge provider failed: %v", err)
		}
	})

	if !strings.Contains(out, `"provider":"codex"`) {
		t.Errorf("provider default not applied: %s", out)
	}
	if !strings.Contains(out, `"intentSummary":"stubbed intent"`) {
		t.Errorf("provider output missing stubbed intent: %s", out)
	}

	args := readCodexStubArgs(t, stubDir)
	for _, want := range []string{"exec", "resume", "--last", "--output-last-message"} {
		if !strings.Contains(args, want) {
			t.Errorf("codex stub args missing %q: %s", want, args)
		}
	}
}

func TestNormalizeProviderContext(t *testing.T) {
	out, err := normalizeProviderContext([]byte(`{"intent":"ship it"}`), "zz-fake")
	if err != nil {
		t.Fatalf("normalize failed: %v", err)
	}
	if !strings.Contains(string(out), `"provider":"zz-fake"`) {
		t.Errorf("provider default not applied: %s", out)
	}

	out, err = normalizeProviderContext([]byte(`{"provider":"custom","intent":"x"}`), "zz-fake")
	if err != nil {
		t.Fatalf("normalize failed: %v", err)
	}
	if !strings.Contains(string(out), `"provider":"custom"`) {
		t.Errorf("explicit provider was overwritten: %s", out)
	}

	if _, err := normalizeProviderContext([]byte(`{not json`), "zz-fake"); err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestActiveBridgeDefaultsToCodex(t *testing.T) {
	testutil.InitRepo(t)

	b, err := ActiveBridge()
	if err != nil {
		t.Fatalf("ActiveBridge failed: %v", err)
	}
	if b.Name() != "codex" {
		t.Errorf("default bridge = %q, want codex", b.Name())
	}
}

func TestActiveBridgeReadsConfig(t *testing.T) {
	dir := testutil.InitRepo(t)

	testutil.RunGit(t, dir, "config", "agit.bridge", "claude-code")
	b, err := ActiveBridge()
	if err != nil {
		t.Fatalf("ActiveBridge failed: %v", err)
	}
	if b.Name() != "claude-code" {
		t.Errorf("bridge = %q, want claude-code", b.Name())
	}

	testutil.RunGit(t, dir, "config", "agit.bridge", "no-such-agent")
	if _, err := ActiveBridge(); err == nil {
		t.Error("expected error for unknown configured bridge, got nil")
	}
}

func TestFeedCheckoutContextOrWarnUsesActiveBridge(t *testing.T) {
	dir := testutil.InitRepo(t)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_HOME", "")
	stubDir := writeCodexStub(t, "")
	testutil.RunGit(t, dir, "config", "agit.bridge", "codex")

	head := testutil.CommitFile(t, dir, "feat.go", "package x", "feature work")
	addAgitNote(t, dir, head, testCheckoutNote)
	if _, err := RestoreHEADContext(); err != nil {
		t.Fatalf("RestoreHEADContext failed: %v", err)
	}

	feedCheckoutContextOrWarn()

	args := readCodexStubArgs(t, stubDir)
	if !strings.Contains(args, head) {
		t.Errorf("active bridge feed missing restored commit %s:\n%s", head, args)
	}
}

func TestBridgeCodexSessionBackwardCompat(t *testing.T) {
	dir := testutil.InitRepo(t)

	if err := Bridge([]string{"codex", "session", "set", "sess-123"}); err != nil {
		t.Fatalf("session set failed: %v", err)
	}
	got := testutil.RunGit(t, dir, "config", "--get", "agit.codexSession")
	if strings.TrimSpace(got) != "sess-123" {
		t.Errorf("agit.codexSession = %q, want sess-123", got)
	}

	if err := Bridge([]string{"codex", "session", "get"}); err != nil {
		t.Fatalf("session get failed: %v", err)
	}
	if err := Bridge([]string{"codex", "session", "unset"}); err != nil {
		t.Fatalf("session unset failed: %v", err)
	}
}

func TestLoadCheckoutContext(t *testing.T) {
	dir := testutil.InitRepo(t)

	if _, err := LoadCheckoutContext(); err == nil {
		t.Error("expected error when no context restored, got nil")
	}

	head := testutil.CommitFile(t, dir, "feat.go", "package x", "feature work")
	addAgitNote(t, dir, head, testCheckoutNote)
	if _, err := RestoreHEADContext(); err != nil {
		t.Fatalf("RestoreHEADContext failed: %v", err)
	}

	ctx, err := LoadCheckoutContext()
	if err != nil {
		t.Fatalf("LoadCheckoutContext failed: %v", err)
	}
	if ctx.Commit != head {
		t.Errorf("Commit = %q, want %q", ctx.Commit, head)
	}
	if ctx.SchemaVersion != "agit.checkout-context.v0" {
		t.Errorf("SchemaVersion = %q", ctx.SchemaVersion)
	}
}
