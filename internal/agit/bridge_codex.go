package agit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func init() {
	RegisterBridge(codexBridge{})
}

// codexBridge is the AgentBridge implementation for Codex.
type codexBridge struct{}

var _ AgentBridge = codexBridge{}

func (codexBridge) Name() string        { return "codex" }
func (codexBridge) DisplayName() string { return "Codex" }

func (codexBridge) Install() error {
	if err := gitwrapRunConfig("agit.contextCommand", "agit bridge codex provider"); err != nil {
		return err
	}
	if err := gitwrapRunConfig("agit.bridge", "codex"); err != nil {
		return err
	}

	fmt.Println("agit: Codex context provider configured")
	fmt.Println("agit: run 'agit mcp install codex' so Codex can also restore/read context through MCP")
	return nil
}

// Provide asks the Codex session for fresh context JSON. The response is
// parsed and normalized by the generic provider verb.
func (codexBridge) Provide() ([]byte, error) {
	return askCodexForContext()
}

// Feed delivers restored checkout context to the Codex session.
func (codexBridge) Feed(ctx *RestoredCheckoutContext) error {
	prompt, err := feedPromptContext(ctx)
	if err != nil {
		return err
	}

	cmdArgs := []string{"exec", "resume"}
	if sessionID := codexSessionID(); sessionID != "" {
		cmdArgs = append(cmdArgs, sessionID)
	} else {
		cmdArgs = append(cmdArgs, "--last")
	}
	cmdArgs = append(cmdArgs, prompt)

	cmd := exec.Command(codexExecutable(), cmdArgs...)
	cmd.Env = append(os.Environ(), "AGIT_CODEX_CONTEXT_FEED=1")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("Codex checkout feed failed: %s", message)
	}

	fmt.Println("agit: fed checkout context to Codex")
	return nil
}

func askCodexForContext() ([]byte, error) {
	tmp, err := os.CreateTemp("", "agit-codex-context-*.json")
	if err != nil {
		return nil, err
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(tmpPath)

	cmdArgs := []string{"exec", "resume"}
	if sessionID := codexSessionID(); sessionID != "" {
		cmdArgs = append(cmdArgs, sessionID)
	} else {
		cmdArgs = append(cmdArgs, "--last")
	}
	cmdArgs = append(cmdArgs, "--output-last-message", tmpPath, codexContextPrompt())

	cmd := exec.Command(codexExecutable(), cmdArgs...)
	cmd.Env = append(os.Environ(), "AGIT_CODEX_CONTEXT_PROVIDER=1")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("Codex context bridge failed: %s", message)
	}

	raw, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, err
	}

	return extractJSONObject(raw)
}

func codexExecutable() string {
	if runtime.GOOS == "windows" {
		return "codex.cmd"
	}
	return "codex"
}

func codexContextPrompt() string {
	return `Return ONLY a JSON object for ai-git agent context. Do not include markdown or commentary.

Use this schema:
{
  "provider": "codex",
  "intent": "short task intent",
  "message": "suggested git commit message",
  "files": ["path/to/file"],
  "planned": "planned change summary",
  "actual": "actual change summary",
  "decisions": ["important decision"],
  "risks": ["risk or caveat"],
  "nextSteps": ["useful next step"]
}

Use the current Codex session and current repository state. Do not include hidden chain-of-thought. Keep values concise and auditable.`
}

func extractJSONObject(raw []byte) ([]byte, error) {
	start := bytes.IndexByte(raw, '{')
	end := bytes.LastIndexByte(raw, '}')
	if start < 0 || end < start {
		return nil, errors.New("agent did not return a JSON object")
	}

	candidate := raw[start : end+1]
	var payload map[string]any
	if err := json.Unmarshal(candidate, &payload); err != nil {
		return nil, err
	}

	return candidate, nil
}

func gitwrapRunConfig(key string, value string) error {
	cmd := exec.Command("git", "config", key, value)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir, _ = filepath.Abs(".")
	return cmd.Run()
}

func gitwrapOutputConfig(key string) (string, error) {
	var stdout bytes.Buffer
	cmd := exec.Command("git", "config", "--get", key)
	cmd.Stdout = &stdout
	cmd.Dir, _ = filepath.Abs(".")
	err := cmd.Run()
	return strings.TrimSpace(stdout.String()), err
}

func gitwrapUnsetConfig(key string) error {
	cmd := exec.Command("git", "config", "--unset", key)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir, _ = filepath.Abs(".")
	return cmd.Run()
}

func codexSessionID() string {
	return bridgeSessionID("AGIT_CODEX_SESSION_ID", "agit.codexSession")
}
