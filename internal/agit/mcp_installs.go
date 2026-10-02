package agit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// agitMCPCommand is the executable name MCP hosts should spawn. On
// Windows the npm-installed binary is agit.cmd, and host process
// spawners (CreateProcess-based, no shell) do not resolve extensionless
// names through PATHEXT the way a shell would.
func agitMCPCommand() string {
	if runtime.GOOS == "windows" {
		return "agit.cmd"
	}
	return "agit"
}

// MCPInstallTargets lists the `agit mcp install` targets in suggestion
// order. Keep in sync with the MCPInstall switch below.
func MCPInstallTargets() []string {
	return []string{"codex", "claude", "pi", "cursor", "gemini", "opencode"}
}

func MCPInstall(target string) error {
	switch strings.ToLower(target) {
	case "codex":
		return installCodex()
	case "claude":
		return installClaude()
	case "pi":
		return installPi()
	case "cursor":
		return installCursor()
	case "gemini":
		return installGemini()
	case "opencode":
		return installOpencode()
	default:
		return fmt.Errorf("unsupported MCP target: %s (supported: %s)", target, strings.Join(MCPInstallTargets(), ", "))
	}
}

// mcpInstallTarget maps a bridge name to its `agit mcp install` target.
// They match except claude-code, which installs as "claude".
func mcpInstallTarget(bridgeName string) string {
	if bridgeName == "claude-code" {
		return "claude"
	}
	return bridgeName
}

// mcpTargetExecutable returns the CLI executable whose presence on PATH
// suggests the user works with the given MCP install target.
func mcpTargetExecutable(target string) (string, error) {
	if target == "claude" {
		return BridgeExecutable("claude-code")
	}
	return BridgeExecutable(target)
}

func codexMCPConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".codex", "config.toml"), nil
}

func cursorMCPConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cursor", "mcp.json"), nil
}

func geminiMCPConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".gemini", "settings.json"), nil
}

func installCodex() error {
	configPath, err := codexMCPConfigPath()
	if err != nil {
		return err
	}
	configDir := filepath.Dir(configPath)

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	block := fmt.Sprintf(`[mcp_servers.agit]
command = %q
args = ["mcp", "serve"]
enabled = true
startup_timeout_sec = 10
tool_timeout_sec = 30
`, agitMCPCommand())

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
	fmt.Println("claude mcp add --transport stdio --scope user agit -- " + agitMCPCommand() + " mcp serve")
	return nil
}

// agitMCPDescription is the stdio server description written into Pi's
// mcp.json when the pi CLI is unavailable.
const agitMCPDescription = "ai-git: record and restore AI coding-agent context in Git workflows"

func installPi() error {
	// Prefer Pi's own CLI: `pi mcp add` owns the config layout and the
	// add/replace semantics, so idempotency comes for free.
	if _, err := exec.LookPath(piExecutable()); err == nil {
		cmd := exec.Command(piExecutable(), "mcp", "add", "agit", "--", agitMCPCommand(), "mcp", "serve")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("pi mcp add failed: %w", err)
		}
		fmt.Println("Installed agit MCP server for Pi (user level).")
		fmt.Println("For a single project instead, run: pi mcp add -l agit -- " + agitMCPCommand() + " mcp serve")
		fmt.Println("Restart Pi sessions and run `pi mcp list` to verify.")
		return nil
	}

	// No pi on PATH: write the user-level mcp.json directly, in Pi's
	// documented {"mcpServers": {...}} schema. A corrupt file is an
	// error, never silently overwritten.
	agentDir, err := piAgentDir()
	if err != nil {
		return err
	}
	if err := installJSONMCPTarget("Pi", filepath.Join(agentDir, "mcp.json"), map[string]any{
		"description": agitMCPDescription,
	}); err != nil {
		return err
	}
	fmt.Println("For a single project instead, copy the entry into that repo's .pi/mcp.json.")
	fmt.Println("Restart Pi sessions and run `pi mcp list` to verify.")
	return nil
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

func installCursor() error {
	configPath, err := cursorMCPConfigPath()
	if err != nil {
		return err
	}
	return installJSONMCPTarget("Cursor", configPath, nil)
}

func installGemini() error {
	configPath, err := geminiMCPConfigPath()
	if err != nil {
		return err
	}
	return installJSONMCPTarget("Gemini CLI", configPath, nil)
}

// installJSONMCPTarget merges the agit stdio server into the agent's JSON
// MCP config file at configPath. The {"mcpServers": ...} shape is shared
// by Cursor, Pi, Gemini and other JSON-file MCP clients. Existing entries
// are preserved, the install is idempotent, and a corrupt file (or a
// non-object mcpServers value) is an error rather than silently
// overwritten. extraFields adds client-specific entry keys, such as Pi's
// description.
func installJSONMCPTarget(displayName string, configPath string, extraFields map[string]any) error {
	entry := map[string]any{"command": agitMCPCommand(), "args": []string{"mcp", "serve"}}
	for k, v := range extraFields {
		entry[k] = v
	}
	return mergeMCPServerConfig(displayName, configPath, "mcpServers", entry, false)
}

func installOpencode() error {
	configPath, err := opencodeConfigPath()
	if err != nil {
		return err
	}
	return mergeMCPServerConfig("OpenCode", configPath, "mcp", map[string]any{
		"type":    "local",
		"command": []string{agitMCPCommand(), "mcp", "serve"},
		"enabled": true,
	}, true)
}

// opencodeConfigPath resolves OpenCode's user-level config file. OpenCode
// reads one config format per directory, so an existing opencode.jsonc
// wins over creating a new opencode.json beside it that would be ignored.
func opencodeConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".config", "opencode")
	if _, err := os.Stat(filepath.Join(dir, "opencode.jsonc")); err == nil {
		return filepath.Join(dir, "opencode.jsonc"), nil
	}
	return filepath.Join(dir, "opencode.json"), nil
}

// mergeMCPServerConfig merges the agit server entry into configPath under
// serversKey ("mcpServers" for most clients, "mcp" for OpenCode). Existing
// entries are preserved, the install is idempotent, and a corrupt file (or
// a non-object servers value) is an error rather than silently overwritten.
// An empty file counts as absent config. When allowComments is set
// (OpenCode's JSONC), a file that only fails to parse because of comments
// is retried with comments stripped; anything still invalid is refused.
func mergeMCPServerConfig(displayName string, configPath string, serversKey string, entry map[string]any, allowComments bool) error {
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}

	config := map[string]any{}
	raw, err := os.ReadFile(configPath)
	switch {
	case err == nil:
		if len(bytes.TrimSpace(raw)) == 0 {
			break
		}
		if err := json.Unmarshal(raw, &config); err != nil {
			if !allowComments {
				return fmt.Errorf("your %s is not valid JSON; fix it or move it aside, then re-run install: %w", configPath, err)
			}
			if err := json.Unmarshal(stripJSONComments(raw), &config); err != nil {
				return fmt.Errorf("your %s is not valid JSON (even ignoring comments); fix it or move it aside, then re-run install: %w", configPath, err)
			}
			fmt.Printf("note: comments in %s were normalized during install\n", configPath)
		}
		if config == nil {
			return fmt.Errorf("your %s does not contain a JSON object; fix it or move it aside, then re-run install", configPath)
		}
	case os.IsNotExist(err):
	default:
		return err
	}

	servers, ok := config[serversKey].(map[string]any)
	if !ok {
		if v, exists := config[serversKey]; exists && v != nil {
			return fmt.Errorf("your %s has a %q value that is not an object; fix it or move it aside, then re-run install", configPath, serversKey)
		}
		servers = map[string]any{}
		config[serversKey] = servers
	}
	if _, exists := servers["agit"]; exists {
		fmt.Printf("agit MCP server already configured for %s.\n", displayName)
		fmt.Println("Config:", configPath)
		return nil
	}
	servers["agit"] = entry

	out, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(configPath, append(out, '\n'), 0644); err != nil {
		return err
	}

	fmt.Printf("Installed agit MCP server for %s.\n", displayName)
	fmt.Println("Config:", configPath)
	return nil
}

// stripJSONComments removes // line comments and /* */ block comments
// from JSONC input. String literals are left untouched, including escaped
// quotes, so comment-looking text inside strings survives. Newlines inside
// line comments are kept; unterminated block comments are dropped to EOF.
func stripJSONComments(raw []byte) []byte {
	var out bytes.Buffer
	out.Grow(len(raw))
	inString, escaped, inLine, inBlock := false, false, false, false
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		switch {
		case inLine:
			if c == '\n' {
				inLine = false
				out.WriteByte(c)
			}
		case inBlock:
			if c == '*' && i+1 < len(raw) && raw[i+1] == '/' {
				inBlock = false
				i++
			}
		case inString:
			out.WriteByte(c)
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
		case c == '"':
			inString = true
			out.WriteByte(c)
		case c == '/' && i+1 < len(raw) && raw[i+1] == '/':
			inLine = true
			i++
		case c == '/' && i+1 < len(raw) && raw[i+1] == '*':
			inBlock = true
			i++
		default:
			out.WriteByte(c)
		}
	}
	return out.Bytes()
}
