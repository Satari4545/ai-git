package agit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func readMCPConfig(t *testing.T, relPath string) map[string]any {
	t.Helper()

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(home, relPath))
	if err != nil {
		t.Fatalf("MCP config not written: %v", err)
	}
	var config map[string]any
	if err := json.Unmarshal(raw, &config); err != nil {
		t.Fatalf("MCP config is not valid JSON: %v", err)
	}
	return config
}

func agitServerEntry(t *testing.T, config map[string]any) map[string]any {
	t.Helper()

	servers, ok := config["mcpServers"].(map[string]any)
	if !ok {
		t.Fatalf("mcpServers missing or not an object: %v", config)
	}
	entry, ok := servers["agit"].(map[string]any)
	if !ok {
		t.Fatalf("mcpServers.agit missing: %v", servers)
	}
	return entry
}

func TestMCPInstallCursor(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := MCPInstall("cursor"); err != nil {
		t.Fatalf("MCPInstall failed: %v", err)
	}

	config := readMCPConfig(t, filepath.Join(".cursor", "mcp.json"))
	entry := agitServerEntry(t, config)
	if entry["command"] != agitMCPCommand() {
		t.Errorf("command = %v, want %s", entry["command"], agitMCPCommand())
	}
	args, _ := entry["args"].([]any)
	if len(args) != 2 || args[0] != "mcp" || args[1] != "serve" {
		t.Errorf("args = %v, want [mcp serve]", args)
	}

	if err := MCPInstall("cursor"); err != nil {
		t.Fatalf("second MCPInstall failed: %v", err)
	}
	config = readMCPConfig(t, filepath.Join(".cursor", "mcp.json"))
	servers := config["mcpServers"].(map[string]any)
	count := 0
	for name := range servers {
		if name == "agit" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("install is not idempotent, agit entries: %d", count)
	}
}

func TestMCPInstallCursorPreservesExisting(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cursorDir := filepath.Join(home, ".cursor")
	if err := os.MkdirAll(cursorDir, 0o755); err != nil {
		t.Fatal(err)
	}
	existing := `{"mcpServers": {"other": {"command": "other-server", "args": []}}}`
	if err := os.WriteFile(filepath.Join(cursorDir, "mcp.json"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := MCPInstall("cursor"); err != nil {
		t.Fatalf("MCPInstall failed: %v", err)
	}

	config := readMCPConfig(t, filepath.Join(".cursor", "mcp.json"))
	servers := config["mcpServers"].(map[string]any)
	if _, ok := servers["other"]; !ok {
		t.Errorf("existing server entry lost: %v", servers)
	}
	agitServerEntry(t, config)
}

func TestMCPInstallCursorRefusesCorrupt(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cursorDir := filepath.Join(home, ".cursor")
	if err := os.MkdirAll(cursorDir, 0o755); err != nil {
		t.Fatal(err)
	}
	corrupt := []byte("{not json")
	if err := os.WriteFile(filepath.Join(cursorDir, "mcp.json"), corrupt, 0o644); err != nil {
		t.Fatal(err)
	}

	err := MCPInstall("cursor")
	if err == nil || !strings.Contains(err.Error(), "mcp.json") {
		t.Errorf("expected error naming mcp.json, got: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(cursorDir, "mcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != string(corrupt) {
		t.Errorf("corrupt config was modified: %q", raw)
	}
}

func TestMCPInstallGemini(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := MCPInstall("gemini"); err != nil {
		t.Fatalf("MCPInstall failed: %v", err)
	}

	config := readMCPConfig(t, filepath.Join(".gemini", "settings.json"))
	entry := agitServerEntry(t, config)
	if entry["command"] != agitMCPCommand() {
		t.Errorf("command = %v, want %s", entry["command"], agitMCPCommand())
	}
	args, _ := entry["args"].([]any)
	if len(args) != 2 || args[0] != "mcp" || args[1] != "serve" {
		t.Errorf("args = %v, want [mcp serve]", args)
	}

	if err := MCPInstall("gemini"); err != nil {
		t.Fatalf("second MCPInstall failed: %v", err)
	}
	config = readMCPConfig(t, filepath.Join(".gemini", "settings.json"))
	servers := config["mcpServers"].(map[string]any)
	count := 0
	for name := range servers {
		if name == "agit" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("install is not idempotent, agit entries: %d", count)
	}
}

func TestMCPInstallGeminiPreservesExisting(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	geminiDir := filepath.Join(home, ".gemini")
	if err := os.MkdirAll(geminiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	existing := `{"theme": "dark", "mcpServers": {"other": {"command": "other-server"}}}`
	if err := os.WriteFile(filepath.Join(geminiDir, "settings.json"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := MCPInstall("gemini"); err != nil {
		t.Fatalf("MCPInstall failed: %v", err)
	}

	config := readMCPConfig(t, filepath.Join(".gemini", "settings.json"))
	if config["theme"] != "dark" {
		t.Errorf("existing settings key lost: %v", config)
	}
	servers := config["mcpServers"].(map[string]any)
	if _, ok := servers["other"]; !ok {
		t.Errorf("existing server entry lost: %v", servers)
	}
	agitServerEntry(t, config)
}

func TestMCPInstallGeminiRefusesCorrupt(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	geminiDir := filepath.Join(home, ".gemini")
	if err := os.MkdirAll(geminiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	corrupt := []byte("null")
	if err := os.WriteFile(filepath.Join(geminiDir, "settings.json"), corrupt, 0o644); err != nil {
		t.Fatal(err)
	}

	err := MCPInstall("gemini")
	if err == nil || !strings.Contains(err.Error(), "settings.json") {
		t.Errorf("expected error naming settings.json, got: %v", err)
	}
}

func TestMCPInstallUnsupportedTarget(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	err := MCPInstall("windsurf")
	if err == nil || !strings.Contains(err.Error(), "supported: codex, claude, pi, cursor, gemini, opencode") {
		t.Errorf("expected supported-targets error, got: %v", err)
	}
}

func TestMCPInstallTargetCaseInsensitive(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if err := MCPInstall("CURSOR"); err != nil {
		t.Fatalf("MCPInstall(CURSOR) failed: %v", err)
	}
	if err := MCPInstall("Gemini"); err != nil {
		t.Fatalf("MCPInstall(Gemini) failed: %v", err)
	}
}

func TestAgitMCPCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		got := agitMCPCommand()
		if !filepath.IsAbs(got) {
			t.Errorf("agitMCPCommand() = %q, want an absolute path on Windows", got)
		}
		if strings.ToLower(filepath.Ext(got)) != ".exe" {
			t.Errorf("agitMCPCommand() = %q, want a .exe path on Windows", got)
		}
		return
	}
	if got := agitMCPCommand(); got != "agit" {
		t.Errorf("agitMCPCommand() = %q, want %q", got, "agit")
	}
}

func TestInstallCodexWritesPlatformCommand(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := installCodex(); err != nil {
		t.Fatal(err)
	}
	home, _ := os.UserHomeDir()
	raw, err := os.ReadFile(filepath.Join(home, ".codex", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("command = %q", agitMCPCommand())
	if !strings.Contains(string(raw), want) {
		t.Errorf("codex config.toml missing %q:\n%s", want, raw)
	}
}
func TestStripJSONComments(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "line comment",
			input: "{\"a\": 1 // trailing\n}",
			want:  "{\"a\": 1 \n}",
		},
		{
			name:  "full line comment",
			input: "// header\n{\"a\": 1}",
			want:  "\n{\"a\": 1}",
		},
		{
			name:  "block comment",
			input: "{\"a\": /* mid */ 1}",
			want:  "{\"a\":  1}",
		},
		{
			name:  "multiline block comment",
			input: "{\"a\": 1, /*\nspan\n*/ \"b\": 2}",
			want:  "{\"a\": 1,  \"b\": 2}",
		},
		{
			name:  "slashes in string survive",
			input: "{\"url\": \"https://example.com//x\"}",
			want:  "{\"url\": \"https://example.com//x\"}",
		},
		{
			name:  "block markers in string survive",
			input: "{\"note\": \"a /* not a comment */ b\"}",
			want:  "{\"note\": \"a /* not a comment */ b\"}",
		},
		{
			name:  "escaped quote keeps string open",
			input: "{\"q\": \"a\\\" // still string\"}",
			want:  "{\"q\": \"a\\\" // still string\"}",
		},
		{
			name:  "comment after string ends",
			input: "{\"q\": \"done\"} // real comment\n",
			want:  "{\"q\": \"done\"} \n",
		},
		{
			name:  "unterminated block comment dropped",
			input: "{\"a\": 1} /* never ends",
			want:  "{\"a\": 1} ",
		},
		{
			name:  "no comments unchanged",
			input: "{\"a\": [1, 2]}",
			want:  "{\"a\": [1, 2]}",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := string(stripJSONComments([]byte(tc.input))); got != tc.want {
				t.Errorf("stripJSONComments(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestMCPInstallEmptyFileTreatedAsAbsent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	configPath := filepath.Join(home, ".cursor", "mcp.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("  \n "), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := MCPInstall("cursor"); err != nil {
		t.Fatalf("MCPInstall over empty file failed: %v", err)
	}

	config := readMCPConfig(t, filepath.Join(".cursor", "mcp.json"))
	agitServerEntry(t, config)
}

func writeOpencodeJSONC(t *testing.T, home, body string) string {
	t.Helper()
	dir := filepath.Join(home, ".config", "opencode")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "opencode.jsonc")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func readOpencodeMCP(t *testing.T, home string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(home, ".config", "opencode", "opencode.jsonc"))
	if err != nil {
		t.Fatalf("opencode config not written: %v", err)
	}
	var config map[string]any
	if err := json.Unmarshal(raw, &config); err != nil {
		t.Fatalf("opencode config is not valid JSON after install: %v", err)
	}
	servers, ok := config["mcp"].(map[string]any)
	if !ok {
		t.Fatalf("mcp key missing or not an object: %v", config)
	}
	if _, ok := servers["agit"]; !ok {
		t.Fatalf("mcp.agit missing: %v", servers)
	}
	return config
}

func TestMCPInstallOpencodeCommentedJSONC(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	writeOpencodeJSONC(t, home, "// user comments\n{\n  // servers below\n  \"mcp\": {\n    \"other\": {\"type\": \"local\", \"command\": [\"other\"]} /* trailing */\n  }\n}\n")

	if err := MCPInstall("opencode"); err != nil {
		t.Fatalf("MCPInstall over commented jsonc failed: %v", err)
	}

	config := readOpencodeMCP(t, home)
	servers := config["mcp"].(map[string]any)
	if _, ok := servers["other"]; !ok {
		t.Errorf("existing server lost during commented jsonc install: %v", servers)
	}
	entry := servers["agit"].(map[string]any)
	if entry["type"] != "local" {
		t.Errorf("agit entry type = %v, want local", entry["type"])
	}
}

func TestMCPInstallOpencodeCommentedJSONCAlreadyConfigured(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	body := "// keep me\n{\"mcp\": {\"agit\": {\"type\": \"local\", \"command\": [\"agit\", \"mcp\", \"serve\"], \"enabled\": true}}}\n"
	path := writeOpencodeJSONC(t, home, body)

	if err := MCPInstall("opencode"); err != nil {
		t.Fatalf("MCPInstall failed: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != body {
		t.Errorf("already-configured commented jsonc was rewritten, want byte-identical:\n%s", raw)
	}
}

func TestMCPInstallOpencodeCommentedJSONCStillInvalid(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	writeOpencodeJSONC(t, home, "// comment\n{not valid json\n")

	err := MCPInstall("opencode")
	if err == nil || !strings.Contains(err.Error(), "not valid JSON") {
		t.Errorf("MCPInstall over corrupt jsonc = %v, want not-valid-JSON refusal", err)
	}
}

func TestMCPInstallCorruptJSONStillRefused(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	configPath := filepath.Join(home, ".cursor", "mcp.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("{oops"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := MCPInstall("cursor")
	if err == nil || !strings.Contains(err.Error(), "not valid JSON") {
		t.Errorf("MCPInstall over corrupt JSON = %v, want not-valid-JSON refusal", err)
	}
}
