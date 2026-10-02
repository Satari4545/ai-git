package agit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Satari4545/ai-git/internal/testutil"
)

func writeOpencodeStub(t *testing.T, script string) {
	t.Helper()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "opencode"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func writeOpencodeArgsStub(t *testing.T, output string) (argsPath string) {
	t.Helper()

	dir := t.TempDir()
	argsPath = filepath.Join(dir, "args")
	script := "#!/bin/sh\nprintf '%s\\0' \"$@\" > " + argsPath + "\nprintf '%s' '" + output + "'\n"
	if err := os.WriteFile(filepath.Join(dir, "opencode"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return argsPath
}

func TestOpencodeBridgeRegistered(t *testing.T) {
	b, err := GetBridge("opencode")
	if err != nil {
		t.Fatalf("opencode bridge not registered: %v", err)
	}
	if b.Name() != "opencode" {
		t.Errorf("Name = %q, want opencode", b.Name())
	}
	if b.DisplayName() != "OpenCode" {
		t.Errorf("DisplayName = %q, want OpenCode", b.DisplayName())
	}

	found := false
	for _, name := range BridgeNames() {
		if name == "opencode" {
			found = true
		}
	}
	if !found {
		t.Errorf("BridgeNames missing opencode: %v", BridgeNames())
	}
}

func TestOpencodeInstallSetsGitConfig(t *testing.T) {
	dir := testutil.InitRepo(t)

	b, err := GetBridge("opencode")
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Install(); err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	if got := testutil.RunGit(t, dir, "config", "--get", "agit.contextCommand"); strings.TrimSpace(got) != "agit bridge opencode provider" {
		t.Errorf("agit.contextCommand = %q", strings.TrimSpace(got))
	}
	if got := testutil.RunGit(t, dir, "config", "--get", "agit.bridge"); strings.TrimSpace(got) != "opencode" {
		t.Errorf("agit.bridge = %q", strings.TrimSpace(got))
	}

	if err := b.Install(); err != nil {
		t.Fatalf("second Install failed: %v", err)
	}
}

func TestOpencodeSessionSetGetUnset(t *testing.T) {
	dir := testutil.InitRepo(t)

	if err := Bridge([]string{"opencode", "session", "set", "sess-oc-2"}); err != nil {
		t.Fatalf("session set failed: %v", err)
	}
	if got := testutil.RunGit(t, dir, "config", "--get", "agit.opencodeSession"); strings.TrimSpace(got) != "sess-oc-2" {
		t.Errorf("agit.opencodeSession = %q, want sess-oc-2", strings.TrimSpace(got))
	}

	if err := Bridge([]string{"opencode", "session", "get"}); err != nil {
		t.Fatalf("session get failed: %v", err)
	}
	if err := Bridge([]string{"opencode", "session", "unset"}); err != nil {
		t.Fatalf("session unset failed: %v", err)
	}
	if err := Bridge([]string{"opencode", "session"}); err != nil {
		t.Fatalf("session without subcommand failed: %v", err)
	}
}

func TestOpencodeSessionIDEnvOverride(t *testing.T) {
	testutil.InitRepo(t)
	t.Setenv("AGIT_OPENCODE_SESSION_ID", "env-sess-9")

	if got := opencodeSessionID(); got != "env-sess-9" {
		t.Errorf("opencodeSessionID = %q, want env-sess-9", got)
	}
}

func TestOpencodeFeedPinnedSession(t *testing.T) {
	dir := testutil.InitRepo(t)
	t.Setenv("AGIT_OPENCODE_SESSION_ID", "feed-id-123")
	argsPath := writeOpencodeArgsStub(t, "")

	head := testutil.CommitFile(t, dir, "feat.go", "package x", "feature work")
	addAgitNote(t, dir, head, testCheckoutNote)
	if _, err := RestoreHEADContext(); err != nil {
		t.Fatalf("RestoreHEADContext failed: %v", err)
	}

	b, err := GetBridge("opencode")
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
		t.Fatalf("opencode args = %q, want 4 args", args)
	}
	if args[0] != "run" || args[1] != "--session" || args[2] != "feed-id-123" {
		t.Errorf("opencode session args = %q, want [run --session feed-id-123]", args[:3])
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

func TestOpencodeFeedContinueFallback(t *testing.T) {
	dir := testutil.InitRepo(t)
	argsPath := writeOpencodeArgsStub(t, "")

	head := testutil.CommitFile(t, dir, "feat.go", "package x", "feature work")
	addAgitNote(t, dir, head, testCheckoutNote)
	if _, err := RestoreHEADContext(); err != nil {
		t.Fatalf("RestoreHEADContext failed: %v", err)
	}

	b, err := GetBridge("opencode")
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
		t.Fatalf("opencode args = %q, want 3 args", args)
	}
	if args[0] != "run" || args[1] != "--continue" {
		t.Errorf("opencode fallback args = %q, want [run --continue]", args[:2])
	}
}

func TestOpencodeFeedReportsOpencodeError(t *testing.T) {
	dir := testutil.InitRepo(t)
	writeOpencodeStub(t, "#!/bin/sh\necho 'boom' >&2\nexit 3\n")

	head := testutil.CommitFile(t, dir, "feat.go", "package x", "feature work")
	addAgitNote(t, dir, head, testCheckoutNote)
	if _, err := RestoreHEADContext(); err != nil {
		t.Fatalf("RestoreHEADContext failed: %v", err)
	}

	b, err := GetBridge("opencode")
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := LoadCheckoutContext()
	if err != nil {
		t.Fatal(err)
	}
	err = b.Feed(ctx)
	if err == nil || !strings.Contains(err.Error(), "OpenCode checkout feed failed: boom") {
		t.Errorf("expected OpenCode checkout feed failure, got: %v", err)
	}
}

func TestOpencodeProvide(t *testing.T) {
	testutil.InitRepo(t)
	argsPath := writeOpencodeArgsStub(t, `{"provider": "opencode", "intent": "do things"}`)

	b, err := GetBridge("opencode")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := b.Provide()
	if err != nil {
		t.Fatalf("Provide failed: %v", err)
	}

	args := readNulArgs(t, argsPath)
	if len(args) != 2 || args[0] != "run" {
		t.Fatalf("opencode provide args = %q, want [run <prompt>]", args)
	}
	if !strings.Contains(args[1], "Return ONLY a JSON object") {
		t.Errorf("provide prompt missing JSON instruction:\n%s", args[1])
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("Provide did not return JSON: %v", err)
	}
	if decoded["intent"] != "do things" {
		t.Errorf("intent = %v, want do things", decoded["intent"])
	}
}

func TestOpencodeProvideErrors(t *testing.T) {
	testutil.InitRepo(t)
	writeOpencodeStub(t, "#!/bin/sh\nexit 1\n")

	b, err := GetBridge("opencode")
	if err != nil {
		t.Fatal(err)
	}
	_, err = b.Provide()
	if err == nil || !strings.Contains(err.Error(), "OpenCode context bridge failed") {
		t.Errorf("expected OpenCode context bridge failure, got: %v", err)
	}
}

func TestOpencodeProvideWithoutCLI(t *testing.T) {
	testutil.InitRepo(t)
	t.Setenv("PATH", pathWithGitOnly(t))

	b, err := GetBridge("opencode")
	if err != nil {
		t.Fatal(err)
	}
	_, err = b.Provide()
	if err == nil || !strings.Contains(err.Error(), "OpenCode context bridge failed") {
		t.Errorf("expected OpenCode context bridge failure, got: %v", err)
	}
}

func TestActiveBridgeOpencode(t *testing.T) {
	dir := testutil.InitRepo(t)
	testutil.RunGit(t, dir, "config", "agit.bridge", "opencode")

	b, err := ActiveBridge()
	if err != nil {
		t.Fatalf("ActiveBridge failed: %v", err)
	}
	if b.Name() != "opencode" {
		t.Errorf("bridge = %q, want opencode", b.Name())
	}
}

func TestMCPInstallOpencode(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := MCPInstall("opencode"); err != nil {
		t.Fatalf("MCPInstall(opencode) failed: %v", err)
	}

	configPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("opencode.json not written: %v", err)
	}
	var config struct {
		MCP map[string]struct {
			Type    string   `json:"type"`
			Command []string `json:"command"`
			Enabled bool     `json:"enabled"`
		} `json:"mcp"`
		MCPServers map[string]any `json:"mcpServers"`
	}
	if err := json.Unmarshal(raw, &config); err != nil {
		t.Fatalf("opencode.json is not valid JSON: %v", err)
	}
	if config.MCPServers != nil {
		t.Errorf("opencode.json must not use mcpServers key:\\n%s", raw)
	}
	server, ok := config.MCP["agit"]
	if !ok {
		t.Fatalf("agit server missing from opencode.json:\\n%s", raw)
	}
	if server.Type != "local" {
		t.Errorf("type = %q, want local", server.Type)
	}
	if len(server.Command) != 3 || server.Command[0] != "agit" || server.Command[1] != "mcp" || server.Command[2] != "serve" {
		t.Errorf("command = %v, want [agit mcp serve]", server.Command)
	}
	if !server.Enabled {
		t.Errorf("enabled = false, want true")
	}

	before := string(raw)
	if err := MCPInstall("opencode"); err != nil {
		t.Fatalf("second MCPInstall(opencode) failed: %v", err)
	}
	after, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != before {
		t.Errorf("install is not idempotent:\\n%s", after)
	}
}

func TestMCPInstallOpencodePreservesExisting(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := filepath.Join(home, ".config", "opencode")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	existing := `{"mcp": {"other": {"type": "local", "command": ["other-mcp"], "enabled": true}}, "theme": "dark"}`
	if err := os.WriteFile(filepath.Join(dir, "opencode.json"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := MCPInstall("opencode"); err != nil {
		t.Fatalf("MCPInstall(opencode) failed: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "opencode.json"))
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := json.Unmarshal(raw, &config); err != nil {
		t.Fatalf("opencode.json is not valid JSON: %v", err)
	}
	servers, ok := config["mcp"].(map[string]any)
	if !ok {
		t.Fatalf("mcp key missing:\\n%s", raw)
	}
	if _, ok := servers["agit"]; !ok {
		t.Errorf("agit server missing:\\n%s", raw)
	}
	if _, ok := servers["other"]; !ok {
		t.Errorf("existing server lost:\\n%s", raw)
	}
	if config["theme"] != "dark" {
		t.Errorf("unrelated key lost:\\n%s", raw)
	}
}

func TestMCPInstallOpencodeRefusesCorrupt(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := filepath.Join(home, ".config", "opencode")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(dir, "opencode.json")
	if err := os.WriteFile(configPath, []byte("{nope"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := MCPInstall("opencode"); err == nil || !strings.Contains(err.Error(), "not valid JSON") {
		t.Errorf("expected not-valid-JSON error, got: %v", err)
	}
	if raw, _ := os.ReadFile(configPath); string(raw) != "{nope" {
		t.Errorf("corrupt file was modified:\\n%s", raw)
	}
}

func TestMCPInstallOpencodeRefusesWrongTypedMcp(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := filepath.Join(home, ".config", "opencode")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(dir, "opencode.json")
	if err := os.WriteFile(configPath, []byte(`{"mcp": "oops"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := MCPInstall("opencode"); err == nil || !strings.Contains(err.Error(), `"mcp"`) {
		t.Errorf("expected wrong-typed mcp error, got: %v", err)
	}
}

func TestMCPInstallOpencodePrefersExistingJsonc(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := filepath.Join(home, ".config", "opencode")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "opencode.jsonc"), []byte(`{"mcp": {}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := MCPInstall("opencode"); err != nil {
		t.Fatalf("MCPInstall(opencode) failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "opencode.json")); !os.IsNotExist(err) {
		t.Errorf("wrote opencode.json beside an existing opencode.jsonc")
	}
	raw, err := os.ReadFile(filepath.Join(dir, "opencode.jsonc"))
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := json.Unmarshal(raw, &config); err != nil {
		t.Fatalf("opencode.jsonc is not valid JSON: %v", err)
	}
	if _, ok := config["mcp"].(map[string]any)["agit"]; !ok {
		t.Errorf("agit server missing from opencode.jsonc:\\n%s", raw)
	}
}

func TestMCPInstallUnsupportedListsOpencode(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	err := MCPInstall("windsurf")
	if err == nil || !strings.Contains(err.Error(), "supported: codex, claude, pi, cursor, gemini, opencode") {
		t.Errorf("expected supported-targets error, got: %v", err)
	}
}
