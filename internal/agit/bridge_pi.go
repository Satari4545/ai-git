package agit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

func init() {
	RegisterBridge(piBridge{})
}

// piBridge is the AgentBridge implementation for the Pi coding agent
// (github.com/earendil-works/pi).
//
// Checkout handoff runs `pi --session <id> -p <prompt>`: it resumes the
// session, appends the prompt as a turn, runs one model turn, persists it,
// and exits — the Pi analog of `codex exec resume <id> <prompt>`. The
// RPC mode was rejected for Feed: it requires the caller to own the
// long-lived RPC process, and Pi exposes no way to attach it to a
// session already running in the TUI.
type piBridge struct{}

var _ AgentBridge = piBridge{}

func (piBridge) Name() string        { return "pi" }
func (piBridge) DisplayName() string { return "Pi" }

func (piBridge) Install() error {
	if err := gitwrapRunConfig("agit.contextCommand", "agit bridge pi provider"); err != nil {
		return err
	}
	if err := gitwrapRunConfig("agit.bridge", "pi"); err != nil {
		return err
	}

	fmt.Println("agit: Pi context provider configured")
	fmt.Println("agit: run 'agit mcp install pi' so Pi can also restore/read context through MCP")
	return nil
}

// Provide asks Pi for fresh context JSON via print mode. The response is
// parsed and normalized by the generic provider verb.
func (piBridge) Provide() ([]byte, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(piExecutable(), piPromptArgs(piContextPrompt())...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(), "PI_SKIP_VERSION_CHECK=1")

	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("Pi context bridge failed: %s", message)
	}

	return extractJSONObject(stdout.Bytes())
}

// Feed delivers restored checkout context to the Pi session.
func (piBridge) Feed(ctx *RestoredCheckoutContext) error {
	raw, err := json.MarshalIndent(ctx, "", "  ")
	if err != nil {
		return err
	}

	var stderr bytes.Buffer
	cmd := exec.Command(piExecutable(), piPromptArgs(checkoutFeedPrompt(raw))...)
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(), "PI_SKIP_VERSION_CHECK=1")

	// The success signal is the exit code, not stdout: the feed prompt
	// asks the model to stay silent, and anything the CLI prints around
	// the model turn (version checks, session banners) is noise.
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("Pi checkout feed failed: %s", message)
	}

	fmt.Println("agit: fed checkout context to Pi")
	return nil
}

// piPromptArgs builds the session-targeting prefix shared by Provide and
// Feed. With a pinned session id it is `--session <id>`; otherwise Pi's
// `--continue` resumes the most recent session.
func piPromptArgs(prompt string) []string {
	args := []string{}
	if sessionID := piSessionID(); sessionID != "" {
		args = append(args, "--session", sessionID)
	} else {
		args = append(args, "--continue")
	}
	return append(args, "-p", prompt)
}

func piExecutable() string {
	if runtime.GOOS == "windows" {
		return "pi.cmd"
	}
	return "pi"
}

func piSessionID() string {
	return bridgeSessionID("AGIT_PI_SESSION_ID", "agit.piSession")
}

func piContextPrompt() string {
	return `Return ONLY a JSON object for ai-git agent context. Do not include markdown or commentary.

Use this schema:
{
  "provider": "pi",
  "intent": "short task intent",
  "message": "suggested git commit message",
  "files": ["path/to/file"],
  "planned": "planned change summary",
  "actual": "actual change summary",
  "decisions": ["important decision"],
  "risks": ["risk or caveat"],
  "nextSteps": ["useful next step"]
}

Use the current Pi session and current repository state. Do not include hidden chain-of-thought. Keep values concise and auditable.`
}

func piBridgeSession(args []string) error {
	if len(args) == 2 && args[0] == "set" {
		if err := gitwrapRunConfig("agit.piSession", args[1]); err != nil {
			return err
		}
		fmt.Println("agit: Pi session configured")
		return nil
	}

	if len(args) == 1 && args[0] == "get" {
		sessionID, _ := gitwrapOutputConfig("agit.piSession")
		if sessionID == "" {
			fmt.Println("No Pi session configured. Provider will use --continue.")
			return nil
		}
		fmt.Println(sessionID)
		return nil
	}

	if len(args) == 1 && args[0] == "unset" {
		_ = gitwrapUnsetConfig("agit.piSession")
		fmt.Println("agit: Pi session unset")
		return nil
	}

	fmt.Println("usage: agit bridge pi session set <id>|get|unset")
	return nil
}
