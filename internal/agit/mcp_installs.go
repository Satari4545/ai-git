package agit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func MCPInstall(target string) error {
	switch strings.ToLower(target) {
	case "codex":
		return installCodex()
	case "claude":
		return installClaude()
	default:
		return fmt.Errorf("unsupported MCP target: %s (supported: codex, claude)", target)
	}
}

func installCodex() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	configDir := filepath.Join(home, ".codex")
	configPath := filepath.Join(configDir, "config.toml")

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	block := `[mcp_servers.agit]
command = "agit"
args = ["mcp", "serve"]
enabled = true
startup_timeout_sec = 10
tool_timeout_sec = 30
`

	var existing string
	if raw, err := os.ReadFile(configPath); err == nil {
		existing = string(raw)
	}

	if strings.Contains(existing, "[mcp_servers.agit]") {
		fmt.Println("agit MCP server already configured for Codex.")
		fmt.Println("Config:", configPath)
		return nil
	}

	if existing != "" && !strings.HasSuffix(existing, "\n") {
		existing += "\n"
	}

	newContent := existing + "\n" + block

	if err := os.WriteFile(configPath, []byte(newContent), 0644); err != nil {
		return err
	}

	fmt.Println("Installed agit MCP server for Codex.")
	fmt.Println("Config:", configPath)
	fmt.Println("Restart Codex and run /mcp to verify.")

	return nil
}

func installClaude() error {
	fmt.Println("To install for Claude Code, run:")
	fmt.Println(`claude mcp add --transport stdio --scope user agit -- agit mcp serve`)
	return nil
}
