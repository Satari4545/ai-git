package agit

import (
	"bytes"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

func init() {
	RegisterBridge(opencodeBridge{})
	registerBridgeExecutable("opencode", opencodeExecutable)
}

// opencodeBridge is the AgentBridge implementation for OpenCode.
//
// Provide runs `opencode run` (non-interactive) and extracts the JSON
// object from the model's text answer. Feed injects the restored checkout
// context into a live session via `opencode run --session <id>`; the
// session is chosen by `agit.opencodeSession` git config (or
// AGIT_OPENCODE_SESSION_ID), falling back to `--continue` for the most
// recent session.
type opencodeBridge struct{}

var _ AgentBridge = opencodeBridge{}

func (opencodeBridge) Name() string        { return "opencode" }
func (opencodeBridge) DisplayName() string { return "OpenCode" }

func (opencodeBridge) Install() error {
	if err := gitwrapRunConfig("agit.contextCommand", "agit bridge opencode provider"); err != nil {
		return err
	}
	if err := gitwrapRunConfig("agit.bridge", "opencode"); err != nil {
		return err
	}

	fmt.Println("agit: OpenCode context provider configured")
	fmt.Println("agit: run 'agit mcp install opencode' so OpenCode can also restore/read context through MCP")
	return nil
}

// Provide asks OpenCode for fresh context JSON in non-interactive mode.
// The response is parsed and normalized by the generic provider verb.
func (opencodeBridge) Provide() ([]byte, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(opencodeExecutable(), "run", opencodeContextPrompt())
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("OpenCode context bridge failed: %s", message)
	}

	return extractJSONObject(stdout.Bytes())
}

// Feed delivers restored checkout context to the OpenCode session.
func (opencodeBridge) Feed(ctx *RestoredCheckoutContext) error {
	prompt, err := feedPromptContext(ctx)
	if err != nil {
		return err
	}

	cmdArgs := []string{"run"}
	if sessionID := opencodeSessionID(); sessionID != "" {
		cmdArgs = append(cmdArgs, "--session", sessionID)
	} else {
		cmdArgs = append(cmdArgs, "--continue")
	}
	cmdArgs = append(cmdArgs, prompt)

	cmd := exec.Command(opencodeExecutable(), cmdArgs...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("OpenCode checkout feed failed: %s", message)
	}

	fmt.Println("agit: fed checkout context to OpenCode")
	return nil
}

func opencodeExecutable() string {
	if runtime.GOOS == "windows" {
		return "opencode.cmd"
	}
	return "opencode"
}

func opencodeContextPrompt() string {
	return `Return ONLY a JSON object for ai-git agent context. Do not include markdown or commentary.

Use this schema:
{
  "provider": "opencode",
  "intent": "short task intent",
  "message": "suggested git commit message",
  "files": ["path/to/file"],
  "planned": "planned change summary",
  "actual": "actual change summary",
  "decisions": ["important decision"],
  "risks": ["risk or caveat"],
  "nextSteps": ["useful next step"]
}

Use the current repository state (git status, recent commits, diffs). Do not include hidden chain-of-thought. Keep values concise and auditable.`
}

func opencodeSessionID() string {
	return bridgeSessionID("AGIT_OPENCODE_SESSION_ID", "agit.opencodeSession")
}
