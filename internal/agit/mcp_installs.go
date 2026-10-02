package agit

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func MCPInstall(target string) error {
	switch strings.ToLower(target) {
	case "codex":
		return installCodex()
	case "claude":
		return installClaude()
	case "pi":
		return installPi()
	default:
		return fmt.Errorf("unsupported MCP target: %s (supported: codex, claude, pi)", target)
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

// agitMCPDescription is the stdio server description written into Pi's
// mcp.json when the pi CLI is unavailable.
const agitMCPDescription = "ai-git: record and restore AI coding-agent context in Git workflows"

func installPi() error {
	// Prefer Pi's own CLI: `pi mcp add` owns the config layout and the
	// add/replace semantics, so idempotency comes for free.
	if _, err := exec.LookPath(piExecutable()); err == nil {
		cmd := exec.Command(piExecutable(), "mcp", "add", "agit", "--", "agit", "mcp", "serve")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("pi mcp add failed: %w", err)
		}
		fmt.Println("Installed agit MCP server for Pi (user level).")
		fmt.Println("For a single project instead, run: pi mcp add -l agit -- agit mcp serve")
		fmt.Println("Restart Pi sessions and run `pi mcp list` to verify.")
		return nil
	}

	// No pi on PATH: write the user-level mcp.json directly, in Pi's
	// documented {"mcpServers": {...}} schema. A corrupt file is an
	// error, never silently overwritten.
	return installPiMCPJSON()
}

// piAgentDir is Pi's user-level config dir: $PI_CODING_AGENT_DIR, or
// ~/.pi/agent by default.
func piAgentDir() (string, error) {
	if dir := os.Getenv("PI_CODING_AGENT_DIR"); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot locate home directory for Pi MCP config: %w", err)
	}
	return filepath.Join(home, ".pi", "agent"), nil
}

func installPiMCPJSON() error {
	agentDir, err := piAgentDir()
	if err != nil {
		return err
	}
	configPath := filepath.Join(agentDir, "mcp.json")

	config := map[string]any{}
	if raw, err := os.ReadFile(configPath); err == nil {
		if err := json.Unmarshal(raw, &config); err != nil {
			return fmt.Errorf("your %s is not valid JSON; fix it or move it aside, then re-run install: %w", configPath, err)
		}
		if config == nil {
			return fmt.Errorf("your %s does not contain a JSON object; fix it or move it aside, then re-run install", configPath)
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	servers, _ := config["mcpServers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
		config["mcpServers"] = servers
	}
	if _, ok := servers["agit"]; ok {
		fmt.Println("agit MCP server already configured for Pi.")
		fmt.Println("Config:", configPath)
		return nil
	}
	servers["agit"] = map[string]any{
		"command":     "agit",
		"args":        []any{"mcp", "serve"},
		"description": agitMCPDescription,
	}

	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		return err
	}
	out, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(configPath, append(out, '\n'), 0o644); err != nil {
		return err
	}

	fmt.Println("Installed agit MCP server for Pi.")
	fmt.Println("Config:", configPath)
	fmt.Println("For a single project instead, copy the entry into that repo's .pi/mcp.json.")
	fmt.Println("Restart Pi sessions and run `pi mcp list` to verify.")
	return nil
}
