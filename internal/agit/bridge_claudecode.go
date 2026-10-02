package agit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func init() {
	RegisterBridge(claudeCodeBridge{})
	registerBridgeExecutable("claude-code", claudeCodeExecutable)
}

// claudeCodeBridge is the AgentBridge implementation for Claude Code.
//
// Unlike Codex, Claude Code has no "resume session and inject" CLI, so
// checkout handoff uses the native SessionStart hook: Install registers
// `agit bridge claude-code hook` in ~/.claude/settings.json, and the hook
// injects the restored checkout context as additionalContext at the start
// of every Claude Code session in the repo. Commit-time context comes from
// `claude -p --output-format json`, whose envelope carries the model's
// response in the `result` field.
type claudeCodeBridge struct{}

var _ AgentBridge = claudeCodeBridge{}

func (claudeCodeBridge) Name() string        { return "claude-code" }
func (claudeCodeBridge) DisplayName() string { return "Claude Code" }

func (claudeCodeBridge) Install() error {
	// Install the hook before touching git config: if the settings file
	// is corrupt or unwritable, fail without leaving a half-installed
	// bridge behind.
	if err := installClaudeCodeSessionStartHook(); err != nil {
		return err
	}
	if err := gitwrapRunConfig("agit.contextCommand", "agit bridge claude-code provider"); err != nil {
		return err
	}
	if err := gitwrapRunConfig("agit.bridge", "claude-code"); err != nil {
		return err
	}

	fmt.Println("agit: Claude Code context provider configured")
	fmt.Println("agit: checkout context will be injected at each Claude Code session start")
	return nil
}

// Provide asks Claude Code for fresh context JSON via print mode, then
// extracts the model's answer from the CLI's JSON envelope.
func (claudeCodeBridge) Provide() ([]byte, error) {
	out, err := runBridgeProvide(claudeCodeBridge{},
		[]string{"-p", "--output-format", "json", claudeCodeContextPrompt()}, nil)
	if err != nil {
		return nil, err
	}

	var envelope struct {
		Result string `json:"result"`
	}
	if err := json.Unmarshal(out, &envelope); err != nil {
		return nil, fmt.Errorf("Claude Code did not return a JSON envelope: %w", err)
	}
	if strings.TrimSpace(envelope.Result) == "" {
		return nil, fmt.Errorf("Claude Code returned an empty result")
	}

	return extractJSONObject([]byte(envelope.Result))
}

// Feed persists the restored checkout context where the user (and the
// SessionStart hook) can find it. Claude Code has no session-injection
// CLI, so the hook delivers it automatically at the next session start;
// the file covers manual paste in the meantime.
func (claudeCodeBridge) Feed(ctx *RestoredCheckoutContext) error {
	text := renderClaudeCodeCheckoutContext(ctx)

	path := filepath.Join(".agit", "state", "claude-code-feed.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return err
	}

	fmt.Println("agit: checkout context written to .agit/state/claude-code-feed.md")
	fmt.Println("agit: it is also injected automatically at your next Claude Code session start")
	return nil
}

func claudeCodeExecutable() string {
	return bridgeExecutableName("claude", ".cmd")
}

func claudeCodeContextPrompt() string {
	return bridgeContextPrompt("claude-code", "Use the current repository state (git status, recent commits, diffs).")
}

// claudeCodeHook implements `agit bridge claude-code hook`, the command
// Install registers as a SessionStart hook. Plain stdout from a
// SessionStart hook becomes additionalContext in the new session, so the
// hook prints the rendered checkout context as text. With no restored
// context it stays silent rather than injecting stale text.
func claudeCodeHook() error {
	out, err := renderClaudeCodeHookContext()
	if err != nil {
		return err
	}
	if out != "" {
		fmt.Print(out)
	}
	return nil
}

func renderClaudeCodeHookContext() (string, error) {
	raw, ok, err := LoadCheckoutContextRaw()
	if err != nil {
		return "", err
	}
	if !ok {
		return "", nil
	}

	var ctx RestoredCheckoutContext
	if err := json.Unmarshal(raw, &ctx); err != nil {
		return "", err
	}
	return renderClaudeCodeCheckoutContext(&ctx), nil
}

func renderClaudeCodeCheckoutContext(ctx *RestoredCheckoutContext) string {
	var b strings.Builder
	b.WriteString("ai-git restored checkout context.\n")
	if ctx.Branch != "" {
		fmt.Fprintf(&b, "branch: %s\n", ctx.Branch)
	}
	fmt.Fprintf(&b, "commit: %s\n", ctx.Commit)
	fmt.Fprintf(&b, "restoredAt: %s\n", ctx.RestoredAt)

	note := bytes.TrimSpace(ctx.Note)
	if len(note) > 0 {
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, note, "", "  "); err == nil {
			note = pretty.Bytes()
		}
		b.WriteString("note:\n")
		b.Write(note)
		b.WriteString("\n")
	}
	return b.String()
}

const claudeCodeHookCommand = "agit bridge claude-code hook"

// installClaudeCodeSessionStartHook merges the agit hook into the native
// ~/.claude/settings.json (plugin-defined SessionStart hooks have their
// context silently dropped upstream, so the native file is the reliable
// surface). Existing settings are preserved; the install is idempotent.
// A corrupt settings file is an error, never silently overwritten.
func installClaudeCodeSessionStartHook() error {
	path, err := claudeCodeSettingsPath()
	if err != nil {
		return err
	}

	settings := map[string]any{}
	raw, err := os.ReadFile(path)
	switch {
	case err == nil:
		if len(bytes.TrimSpace(raw)) == 0 {
			break
		}
		if err := json.Unmarshal(raw, &settings); err != nil {
			return fmt.Errorf("your %s is not valid JSON; fix it or move it aside, then re-run install: %w", path, err)
		}
		if settings == nil {
			return fmt.Errorf("your %s does not contain a JSON object; fix it or move it aside, then re-run install", path)
		}
	case os.IsNotExist(err):
		// Fresh install: settings map stays empty.
	default:
		return err
	}

	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
		settings["hooks"] = hooks
	}
	entries, _ := hooks["SessionStart"].([]any)
	for _, entry := range entries {
		if hookEntryHasCommand(entry, claudeCodeHookCommand) {
			return nil
		}
	}
	hooks["SessionStart"] = append(entries, map[string]any{
		"hooks": []any{
			map[string]any{"type": "command", "command": claudeCodeHookCommand},
		},
	})

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0o644)
}

func claudeCodeSettingsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot locate home directory for Claude Code settings: %w", err)
	}
	return filepath.Join(home, ".claude", "settings.json"), nil
}

// hookEntryHasCommand reports whether a SessionStart hook entry already
// runs the given command. It matches on the serialized entry so minor
// schema drift in settings.json does not break idempotency.
func hookEntryHasCommand(entry any, command string) bool {
	raw, err := json.Marshal(entry)
	if err != nil {
		return false
	}
	return strings.Contains(string(raw), command)
}
