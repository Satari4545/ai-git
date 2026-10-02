package agit

import (
	"bytes"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

func init() {
	RegisterBridge(cursorBridge{})
	registerBridgeExecutable("cursor", cursorAgentExecutable)
}

// cursorBridge is the AgentBridge implementation for Cursor.
//
// Provide runs `cursor-agent -p` (headless print mode) and extracts the
// JSON object from the model's text answer. Feed injects the restored
// checkout context into a live Cursor session via `--resume`; the session
// is chosen by `agit.cursorSession` git config (or AGIT_CURSOR_SESSION_ID),
// falling back to `--continue` for the previous session.
type cursorBridge struct{}

var _ AgentBridge = cursorBridge{}

func (cursorBridge) Name() string        { return "cursor" }
func (cursorBridge) DisplayName() string { return "Cursor" }

func (cursorBridge) Install() error {
	if err := gitwrapRunConfig("agit.contextCommand", "agit bridge cursor provider"); err != nil {
		return err
	}
	if err := gitwrapRunConfig("agit.bridge", "cursor"); err != nil {
		return err
	}

	fmt.Println("agit: Cursor context provider configured")
	fmt.Println("agit: run 'agit mcp install cursor' so Cursor can also restore/read context through MCP")
	return nil
}

// Provide asks the Cursor agent for fresh context JSON in print mode. The
// response is parsed and normalized by the generic provider verb.
func (cursorBridge) Provide() ([]byte, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(cursorAgentExecutable(), "-p", "--trust", "--output-format", "text", cursorContextPrompt())
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("Cursor context bridge failed: %s", message)
	}

	return extractJSONObject(stdout.Bytes())
}

// Feed delivers restored checkout context to the Cursor session.
func (cursorBridge) Feed(ctx *RestoredCheckoutContext) error {
	prompt, err := feedPromptContext(ctx)
	if err != nil {
		return err
	}

	cmdArgs := []string{"-p", "--trust", "--output-format", "text"}
	if sessionID := cursorSessionID(); sessionID != "" {
		cmdArgs = append(cmdArgs, "--resume", sessionID)
	} else {
		cmdArgs = append(cmdArgs, "--continue")
	}
	cmdArgs = append(cmdArgs, prompt)

	cmd := exec.Command(cursorAgentExecutable(), cmdArgs...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("Cursor checkout feed failed: %s", message)
	}

	fmt.Println("agit: fed checkout context to Cursor")
	return nil
}

func cursorAgentExecutable() string {
	if runtime.GOOS == "windows" {
		return "cursor-agent.exe"
	}
	return "cursor-agent"
}

func cursorContextPrompt() string {
	return `Return ONLY a JSON object for ai-git agent context. Do not include markdown or commentary.

Use this schema:
{
  "provider": "cursor",
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

func cursorSessionID() string {
	return bridgeSessionID("AGIT_CURSOR_SESSION_ID", "agit.cursorSession")
}
