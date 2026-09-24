package agit

import (
	"strings"
	"testing"

	"github.com/Satari4545/ai-git/internal/testutil"
)

// fakeBridge is a test double for the Bridge interface.
type fakeBridge struct {
	name        string
	displayName string
	installErr  error
	feedErr     error
	provideJSON string
	provideErr  error

	installed bool
	fed       *RestoredCheckoutContext
	provided  bool
}

func (f *fakeBridge) Name() string        { return f.name }
func (f *fakeBridge) DisplayName() string { return f.displayName }
func (f *fakeBridge) Install() error {
	f.installed = true
	return f.installErr
}
func (f *fakeBridge) Feed(ctx *RestoredCheckoutContext) error {
	f.fed = ctx
	return f.feedErr
}
func (f *fakeBridge) Provide() ([]byte, error) {
	f.provided = true
	if f.provideErr != nil {
		return nil, f.provideErr
	}
	return []byte(f.provideJSON), nil
}

func registerFakeBridge(t *testing.T, name string) *fakeBridge {
	t.Helper()
	fb := &fakeBridge{name: name, displayName: "Fake " + name}
	RegisterBridge(fb)
	return fb
}

func TestBridgeRegistryRoundTrip(t *testing.T) {
	fb := registerFakeBridge(t, "zz-fake-registry")

	got, err := GetBridge("zz-fake-registry")
	if err != nil {
		t.Fatalf("GetBridge failed: %v", err)
	}
	if got != AgentBridge(fb) {
		t.Error("GetBridge did not return the registered bridge")
	}

	if _, err := GetBridge("does-not-exist"); err == nil {
		t.Error("expected error for unknown bridge, got nil")
	}
}

func TestBridgeNamesSorted(t *testing.T) {
	registerFakeBridge(t, "zz-fake-names-b")
	registerFakeBridge(t, "zz-fake-names-a")

	names := BridgeNames()
	found := map[string]bool{}
	for _, n := range names {
		found[n] = true
	}
	for _, want := range []string{"codex", "zz-fake-names-a", "zz-fake-names-b"} {
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
	fb := registerFakeBridge(t, "zz-fake-install")

	if err := Bridge([]string{"zz-fake-install", "install"}); err != nil {
		t.Fatalf("Bridge install failed: %v", err)
	}
	if !fb.installed {
		t.Error("Install was not called on the bridge")
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
	fb := registerFakeBridge(t, "zz-fake-feed")

	head := testutil.CommitFile(t, dir, "feat.go", "package x", "feature work")
	addAgitNote(t, dir, head, testCheckoutNote)
	if _, err := RestoreHEADContext(); err != nil {
		t.Fatalf("RestoreHEADContext failed: %v", err)
	}

	if err := Bridge([]string{"zz-fake-feed", "feed"}); err != nil {
		t.Fatalf("Bridge feed failed: %v", err)
	}
	if fb.fed == nil {
		t.Fatal("Feed was not called on the bridge")
	}
	if fb.fed.Commit != head {
		t.Errorf("fed commit = %q, want %q", fb.fed.Commit, head)
	}
}

func TestBridgeDispatchFeedWithoutContext(t *testing.T) {
	testutil.InitRepo(t)
	registerFakeBridge(t, "zz-fake-feed-empty")

	err := Bridge([]string{"zz-fake-feed-empty", "feed"})
	if err == nil || !strings.Contains(err.Error(), "no checkout context") {
		t.Errorf("expected no-context error, got: %v", err)
	}
}

func TestBridgeDispatchProviderNormalizes(t *testing.T) {
	testutil.InitRepo(t)
	fb := registerFakeBridge(t, "zz-fake-provider")
	fb.provideJSON = `{"intent": "ship it"}`

	// Output goes to stdout; assert the wiring ran and normalization
	// is covered by TestNormalizeProviderContext below.
	if err := Bridge([]string{"zz-fake-provider", "provider"}); err != nil {
		t.Fatalf("Bridge provider failed: %v", err)
	}
	if !fb.provided {
		t.Error("Provide was not called on the bridge")
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
	registerFakeBridge(t, "zz-fake-active")

	testutil.RunGit(t, dir, "config", "agit.bridge", "zz-fake-active")
	b, err := ActiveBridge()
	if err != nil {
		t.Fatalf("ActiveBridge failed: %v", err)
	}
	if b.Name() != "zz-fake-active" {
		t.Errorf("bridge = %q, want zz-fake-active", b.Name())
	}

	testutil.RunGit(t, dir, "config", "agit.bridge", "no-such-agent")
	if _, err := ActiveBridge(); err == nil {
		t.Error("expected error for unknown configured bridge, got nil")
	}
}

func TestFeedCheckoutContextUsesActiveBridge(t *testing.T) {
	dir := testutil.InitRepo(t)
	fb := registerFakeBridge(t, "zz-fake-checkout-feed")
	testutil.RunGit(t, dir, "config", "agit.bridge", "zz-fake-checkout-feed")

	head := testutil.CommitFile(t, dir, "feat.go", "package x", "feature work")
	addAgitNote(t, dir, head, testCheckoutNote)
	if _, err := RestoreHEADContext(); err != nil {
		t.Fatalf("RestoreHEADContext failed: %v", err)
	}

	if err := feedCheckoutContext(); err != nil {
		t.Fatalf("feedCheckoutContext failed: %v", err)
	}
	if fb.fed == nil || fb.fed.Commit != head {
		t.Errorf("active bridge was not fed the restored context")
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
