package agit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func init() {
	RegisterBridge(codexBridge{})
	registerBridgeExecutable("codex", codexExecutable)
}

// codexBridge is the AgentBridge implementation for Codex.
type codexBridge struct{}

var _ AgentBridge = codexBridge{}

func (codexBridge) Name() string        { return "codex" }
func (codexBridge) DisplayName() string { return "Codex" }

func (codexBridge) Install() error {
	return installBridgeProvider(codexBridge{})
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

	return runBridgeFeed(codexBridge{}, cmdArgs, []string{"AGIT_CODEX_CONTEXT_FEED=1"})
}

// askCodexForContext runs the provider prompt through Codex. Codex streams
// session chatter to stdout, so the model's answer is captured via
// --output-last-message into a temp file instead of the process output.
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
	return bridgeExecutableName("codex", ".cmd")
}

func codexContextPrompt() string {
	return bridgeContextPrompt("codex", "Use the current Codex session and current repository state.")
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
