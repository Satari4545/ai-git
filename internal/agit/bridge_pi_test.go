package agit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Satari4545/ai-git/internal/testutil"
)

func TestPiBridgeRegistered(t *testing.T) {
	b, err := GetBridge("pi")
	if err != nil {
		t.Fatalf("pi bridge not registered: %v", err)
	}
	if b.Name() != "pi" {
		t.Errorf("Name = %q, want pi", b.Name())
	}
	if b.DisplayName() != "Pi" {
		t.Errorf("DisplayName = %q, want Pi", b.DisplayName())
	}

	found := false
	for _, name := range BridgeNames() {
		if name == "pi" {
			found = true
		}
	}
	if !found {
		t.Errorf("BridgeNames missing pi: %v", BridgeNames())
	}
}

func writePiStub(t *testing.T, script string) (argsPath string) {
	t.Helper()

	dir := t.TempDir()
	argsPath = filepath.Join(dir, "args")
	if err := os.WriteFile(filepath.Join(dir, "pi"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return argsPath
}

func writePiArgsStub(t *testing.T, output string) (argsPath string) {
	t.Helper()

	dir := t.TempDir()
	argsPath = filepath.Join(dir, "args")
	script := "#!/bin/sh\nprintf '%s\\0' \"$@\" > " + argsPath + "\nprintf '%s' '" + output + "'\n"
	if err := os.WriteFile(filepath.Join(dir, "pi"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return argsPath
}

func readNulArgs(t *testing.T, argsPath string) []string {
	t.Helper()

	raw, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("args file not written: %v", err)
	}
	return strings.Split(strings.TrimSuffix(string(raw), "\x00"), "\x00")
}

func TestPiInstallSetsGitConfig(t *testing.T) {
	dir := testutil.InitRepo(t)

	b, err := GetBridge("pi")
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Install(); err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	if got := testutil.RunGit(t, dir, "config", "--get", "agit.contextCommand"); strings.TrimSpace(got) != "agit bridge pi provider" {
		t.Errorf("agit.contextCommand = %q", strings.TrimSpace(got))
	}
	if got := testutil.RunGit(t, dir, "config", "--get", "agit.bridge"); strings.TrimSpace(got) != "pi" {
		t.Errorf("agit.bridge = %q", strings.TrimSpace(got))
	}
}

func TestPiSessionVerbs(t *testing.T) {
	testutil.InitRepo(t)
	t.Setenv("AGIT_PI_SESSION_ID", "")

	if err := Bridge([]string{"pi", "session", "set", "feed-id-123"}); err != nil {
		t.Fatalf("session set failed: %v", err)
	}
	if got := piSessionID(); got != "feed-id-123" {
		t.Errorf("piSessionID = %q, want feed-id-123", got)
	}
	if got := testutil.RunGit(t, ".", "config", "--get", "agit.piSession"); strings.TrimSpace(got) != "feed-id-123" {
		t.Errorf("agit.piSession = %q", strings.TrimSpace(got))
	}

	if err := Bridge([]string{"pi", "session", "unset"}); err != nil {
		t.Fatalf("session unset failed: %v", err)
	}
	if got := piSessionID(); got != "" {
		t.Errorf("piSessionID after unset = %q, want empty", got)
	}
}

func TestPiSessionIDEnvOverride(t *testing.T) {
	testutil.InitRepo(t)
	t.Setenv("AGIT_PI_SESSION_ID", "env-id")

	if err := Bridge([]string{"pi", "session", "set", "git-id"}); err != nil {
		t.Fatalf("session set failed: %v", err)
	}
	if got := piSessionID(); got != "env-id" {
		t.Errorf("piSessionID = %q, want env-id", got)
	}
}

func TestPiFeedPinnedSession(t *testing.T) {
	dir := testutil.InitRepo(t)
	t.Setenv("AGIT_PI_SESSION_ID", "feed-id-123")
	argsPath := writePiArgsStub(t, "")

	head := testutil.CommitFile(t, dir, "feat.go", "package x", "feature work")
	addAgitNote(t, dir, head, testCheckoutNote)
	if _, err := RestoreHEADContext(); err != nil {
		t.Fatalf("RestoreHEADContext failed: %v", err)
	}

	b, err := GetBridge("pi")
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

	args := readNulArgs(t, argsPath)
	if len(args) != 4 {
		t.Fatalf("pi args = %q, want 4 args", args)
	}
	if args[0] != "--session" || args[1] != "feed-id-123" || args[2] != "-p" {
		t.Errorf("pi session args = %q, want [--session feed-id-123 -p]", args[:3])
	}
	if !strings.Contains(args[3], "ai-git restored checkout context") {
		t.Errorf("feed prompt missing ai-git marker:\n%s", args[3])
	}
	if !strings.Contains(args[3], "work on feature branch") {
		t.Errorf("feed prompt missing intent:\n%s", args[3])
	}
	if !strings.Contains(args[3], head) {
		t.Errorf("feed prompt missing commit %s", head)
	}
}

func TestPiFeedContinueFallback(t *testing.T) {
	dir := testutil.InitRepo(t)
	t.Setenv("AGIT_PI_SESSION_ID", "")
	argsPath := writePiArgsStub(t, "")

	head := testutil.CommitFile(t, dir, "feat.go", "package x", "feature work")
	addAgitNote(t, dir, head, testCheckoutNote)
	if _, err := RestoreHEADContext(); err != nil {
		t.Fatalf("RestoreHEADContext failed: %v", err)
	}

	b, err := GetBridge("pi")
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

	args := readNulArgs(t, argsPath)
	if len(args) != 3 {
		t.Fatalf("pi args = %q, want 3 args", args)
	}
	if args[0] != "--continue" || args[1] != "-p" {
		t.Errorf("pi fallback args = %q, want [--continue -p]", args[:2])
	}
}

func TestPiFeedReportsPiError(t *testing.T) {
	testutil.InitRepo(t)
	t.Setenv("AGIT_PI_SESSION_ID", "feed-id-123")
	writePiStub(t, "#!/bin/sh\necho 'session not found' >&2\nexit 1\n")

	b, err := GetBridge("pi")
	if err != nil {
		t.Fatal(err)
	}
	ctx := &RestoredCheckoutContext{Branch: "main", Commit: "abc"}
	err = b.Feed(ctx)
	if err == nil || !strings.Contains(err.Error(), "session not found") {
		t.Errorf("expected pi stderr in Feed error, got: %v", err)
	}
}

func TestPiProvide(t *testing.T) {
	testutil.InitRepo(t)
	t.Setenv("AGIT_PI_SESSION_ID", "provide-id-456")
	argsPath := writePiArgsStub(t, `{"provider":"pi","intent":"wire the bridge"}`)

	b, err := GetBridge("pi")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := b.Provide()
	if err != nil {
		t.Fatalf("Provide failed: %v", err)
	}
	if !strings.Contains(string(raw), `"intent":"wire the bridge"`) {
		t.Errorf("Provide did not return the JSON object: %s", raw)
	}

	args := readNulArgs(t, argsPath)
	if len(args) != 4 {
		t.Fatalf("pi args = %q, want 4 args", args)
	}
	if args[0] != "--session" || args[1] != "provide-id-456" || args[2] != "-p" {
		t.Errorf("pi provide args = %q, want [--session provide-id-456 -p]", args[:3])
	}
	if !strings.Contains(args[3], `"provider": "pi"`) {
		t.Errorf("provide prompt missing pi schema:\n%s", args[3])
	}
}

func TestPiProvideExtractsObjectFromNoise(t *testing.T) {
	testutil.InitRepo(t)
	t.Setenv("AGIT_PI_SESSION_ID", "provide-id-456")
	writePiArgsStub(t, "checking version...\n{\"intent\":\"noisy output\"}\n")

	b, err := GetBridge("pi")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := b.Provide()
	if err != nil {
		t.Fatalf("Provide failed: %v", err)
	}
	if !strings.Contains(string(raw), `"intent":"noisy output"`) {
		t.Errorf("Provide did not extract the JSON object: %s", raw)
	}
}

func TestPiProvideErrors(t *testing.T) {
	testutil.InitRepo(t)
	t.Setenv("AGIT_PI_SESSION_ID", "provide-id-456")

	b, err := GetBridge("pi")
	if err != nil {
		t.Fatal(err)
	}

	writePiStub(t, "#!/bin/sh\necho 'not json at all'\n")
	if _, err := b.Provide(); err == nil {
		t.Error("expected error for non-JSON stdout, got nil")
	}

	writePiStub(t, "#!/bin/sh\necho 'pi exploded' >&2\nexit 1\n")
	_, err = b.Provide()
	if err == nil || !strings.Contains(err.Error(), "pi exploded") {
		t.Errorf("expected pi stderr in Provide error, got: %v", err)
	}
}

func TestPiSessionDispatchUsage(t *testing.T) {
	testutil.InitRepo(t)
	if err := Bridge([]string{"pi", "session"}); err != nil {
		t.Errorf("session with no verb should print usage, not error: %v", err)
	}
	if err := Bridge([]string{"pi"}); err != nil {
		t.Errorf("bare pi dispatch should print usage, not error: %v", err)
	}
}

func TestActiveBridgePi(t *testing.T) {
	dir := testutil.InitRepo(t)
	testutil.RunGit(t, dir, "config", "agit.bridge", "pi")

	b, err := ActiveBridge()
	if err != nil {
		t.Fatalf("ActiveBridge failed: %v", err)
	}
	if b.Name() != "pi" {
		t.Errorf("bridge = %q, want pi", b.Name())
	}
}

func TestMCPInstallPiViaCLI(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	argsPath := writePiArgsStub(t, "")

	if err := MCPInstall("pi"); err != nil {
		t.Fatalf("MCPInstall(pi) failed: %v", err)
	}

	args := readNulArgs(t, argsPath)
	want := []string{"mcp", "add", "agit", "--", "agit", "mcp", "serve"}
	if len(args) != len(want) {
		t.Fatalf("pi args = %q, want %q", args, want)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("pi args = %q, want %q", args, want)
		}
	}
}

func TestMCPInstallPiJSONFallback(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", t.TempDir())
	t.Setenv("PI_CODING_AGENT_DIR", "")

	if err := MCPInstall("pi"); err != nil {
		t.Fatalf("MCPInstall(pi) failed: %v", err)
	}

	configPath := filepath.Join(home, ".pi", "agent", "mcp.json")
	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("mcp.json not written: %v", err)
	}
	var config struct {
		MCPServers map[string]struct {
			Command     string   `json:"command"`
			Args        []string `json:"args"`
			Description string   `json:"description"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(raw, &config); err != nil {
		t.Fatalf("mcp.json is not valid JSON: %v", err)
	}
	server, ok := config.MCPServers["agit"]
	if !ok {
		t.Fatalf("agit server missing from mcp.json:\n%s", raw)
	}
	if server.Command != "agit" {
		t.Errorf("command = %q, want agit", server.Command)
	}
	if len(server.Args) != 2 || server.Args[0] != "mcp" || server.Args[1] != "serve" {
		t.Errorf("args = %v, want [mcp serve]", server.Args)
	}

	before := string(raw)
	if err := MCPInstall("pi"); err != nil {
		t.Fatalf("second MCPInstall(pi) failed: %v", err)
	}
	after, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != before {
		t.Errorf("install is not idempotent:\n%s", after)
	}
}

func TestMCPInstallPiJSONFallbackPreservesExisting(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", t.TempDir())
	t.Setenv("PI_CODING_AGENT_DIR", "")

	agentDir := filepath.Join(home, ".pi", "agent")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	existing := `{"mcpServers": {"other": {"command": "other-server", "args": ["x"]}}}`
	if err := os.WriteFile(filepath.Join(agentDir, "mcp.json"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := MCPInstall("pi"); err != nil {
		t.Fatalf("MCPInstall(pi) failed: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(agentDir, "mcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		MCPServers map[string]map[string]any `json:"mcpServers"`
	}
	if err := json.Unmarshal(raw, &config); err != nil {
		t.Fatalf("mcp.json is not valid JSON: %v", err)
	}
	if config.MCPServers["other"]["command"] != "other-server" {
		t.Errorf("existing server lost:\n%s", raw)
	}
	if _, ok := config.MCPServers["agit"]; !ok {
		t.Errorf("agit server not merged:\n%s", raw)
	}
}

func TestMCPInstallPiJSONFallbackEnvOverride(t *testing.T) {
	agentDir := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", agentDir)
	t.Setenv("PATH", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	if err := MCPInstall("pi"); err != nil {
		t.Fatalf("MCPInstall(pi) failed: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(agentDir, "mcp.json"))
	if err != nil {
		t.Fatalf("mcp.json not written under PI_CODING_AGENT_DIR: %v", err)
	}
	var config map[string]any
	if err := json.Unmarshal(raw, &config); err != nil {
		t.Fatalf("mcp.json is not valid JSON: %v", err)
	}
	servers, _ := config["mcpServers"].(map[string]any)
	if _, ok := servers["agit"]; !ok {
		t.Errorf("agit server missing from mcp.json:\n%s", raw)
	}
}

func TestMCPInstallPiJSONFallbackRefusesCorrupt(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", t.TempDir())
	t.Setenv("PI_CODING_AGENT_DIR", "")

	agentDir := filepath.Join(home, ".pi", "agent")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentDir, "mcp.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := MCPInstall("pi")
	if err == nil || !strings.Contains(err.Error(), "mcp.json") {
		t.Errorf("expected error naming mcp.json, got: %v", err)
	}
}

func TestMCPInstallUnsupportedListsPi(t *testing.T) {
	err := MCPInstall("bogus")
	if err == nil || !strings.Contains(err.Error(), "codex, claude, pi") {
		t.Errorf("expected supported list to include pi, got: %v", err)
	}
}
