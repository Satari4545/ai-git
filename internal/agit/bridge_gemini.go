package agit

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

func init() {
	RegisterBridge(geminiBridge{})
}

// geminiBridge is the AgentBridge implementation for the Gemini CLI.
//
// Provide runs `gemini -p` (non-interactive) and extracts the JSON object
// from the model's text answer. Feed injects the restored checkout context
// into a live Gemini session via `--resume`; the session is chosen by
// `agit.geminiSession` git config (or AGIT_GEMINI_SESSION_ID), falling back
// to `--resume` with no argument for the project's latest session.
type geminiBridge struct{}

var _ AgentBridge = geminiBridge{}

func (geminiBridge) Name() string        { return "gemini" }
func (geminiBridge) DisplayName() string { return "Gemini CLI" }

func (geminiBridge) Install() error {
	if err := gitwrapRunConfig("agit.contextCommand", "agit bridge gemini provider"); err != nil {
		return err
	}
	if err := gitwrapRunConfig("agit.bridge", "gemini"); err != nil {
		return err
	}

	fmt.Println("agit: Gemini CLI context provider configured")
	fmt.Println("agit: run 'agit mcp install gemini' so Gemini CLI can also restore/read context through MCP")
	return nil
}

// Provide asks the Gemini CLI for fresh context JSON in non-interactive
// mode. The response is parsed and normalized by the generic provider verb.
func (geminiBridge) Provide() ([]byte, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(geminiExecutable(), "-p", "--output-format", "text", geminiContextPrompt())
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("Gemini CLI context bridge failed: %s", message)
	}

	return extractJSONObject(stdout.Bytes())
}

// Feed delivers restored checkout context to the Gemini CLI session.
func (geminiBridge) Feed(ctx *RestoredCheckoutContext) error {
	prompt, err := feedPromptContext(ctx)
	if err != nil {
		return err
	}

	cmdArgs := []string{"-p", "--output-format", "text"}
	if sessionID := geminiSessionID(); sessionID != "" {
		cmdArgs = append(cmdArgs, "--resume", sessionID)
	} else {
		cmdArgs = append(cmdArgs, "--resume", "latest")
	}
	cmdArgs = append(cmdArgs, prompt)

	cmd := exec.Command(geminiExecutable(), cmdArgs...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("Gemini CLI checkout feed failed: %s", message)
	}

	fmt.Println("agit: fed checkout context to Gemini CLI")
	return nil
}

func geminiExecutable() string {
	if runtime.GOOS == "windows" {
		return "gemini.cmd"
	}
	return "gemini"
}

func geminiContextPrompt() string {
	return `Return ONLY a JSON object for ai-git agent context. Do not include markdown or commentary.

Use this schema:
{
  "provider": "gemini",
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

func geminiSessionID() string {
	if sessionID := os.Getenv("AGIT_GEMINI_SESSION_ID"); sessionID != "" {
		return sessionID
	}

	sessionID, _ := gitwrapOutputConfig("agit.geminiSession")
	return sessionID
}
