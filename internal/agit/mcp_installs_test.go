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
	want := "agit"
	if runtime.GOOS == "windows" {
		want = "agit.cmd"
	}
	if got := agitMCPCommand(); got != want {
		t.Errorf("agitMCPCommand() = %q, want %q", got, want)
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
