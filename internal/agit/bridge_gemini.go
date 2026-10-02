package agit

import "fmt"

func init() {
	RegisterBridge(geminiBridge{})
	registerBridgeExecutable("gemini", geminiExecutable)
}

// geminiBridge is the AgentBridge implementation for the Gemini CLI.
//
// Provide runs `gemini -p` (non-interactive) and extracts the JSON object
// from the model's text answer. Feed injects the restored checkout context
// into a live Gemini session via `--resume`; the session is chosen by
// `agit.geminiSession` git config (or AGIT_GEMINI_SESSION_ID), falling back
// to `--resume latest` for the project's latest session.
type geminiBridge struct{}

var _ AgentBridge = geminiBridge{}

func (geminiBridge) Name() string        { return "gemini" }
func (geminiBridge) DisplayName() string { return "Gemini CLI" }

func (geminiBridge) Install() error {
	return installBridgeProvider(geminiBridge{})
}

func (geminiBridge) Provide() ([]byte, error) {
	out, err := runBridgeProvide(geminiBridge{},
		[]string{"-p", "--output-format", "text", geminiContextPrompt()}, nil)
	if err != nil {
		return nil, err
	}
	return extractJSONObject(out)
}

func (geminiBridge) Feed(ctx *RestoredCheckoutContext) error {
	prompt, err := feedPromptContext(ctx)
	if err != nil {
		return err
	}

	cmdArgs := []string{"-p", "--output-format", "text"}
	sessionID, notice := resolveFeedSession(geminiBridge{}, geminiSessionID())
	if sessionID != "" {
		cmdArgs = append(cmdArgs, "--resume", sessionID)
	} else {
		cmdArgs = append(cmdArgs, "--resume", "latest")
	}
	cmdArgs = append(cmdArgs, prompt)

	if err := runBridgeFeed(geminiBridge{}, cmdArgs, nil); err != nil {
		return err
	}
	if notice != "" {
		fmt.Println(notice)
	}
	return nil
}

func geminiExecutable() string {
	return bridgeExecutableName("gemini", ".cmd")
}

func geminiContextPrompt() string {
	return bridgeContextPrompt("gemini", "Use the current repository state (git status, recent commits, diffs).")
}

func geminiSessionID() string {
	return bridgeSessionID("AGIT_GEMINI_SESSION_ID", "agit.geminiSession")
}
