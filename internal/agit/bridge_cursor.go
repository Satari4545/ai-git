package agit

import "fmt"

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
	return installBridgeProvider(cursorBridge{})
}

func (cursorBridge) Provide() ([]byte, error) {
	out, err := runBridgeProvide(cursorBridge{},
		[]string{"-p", "--trust", "--output-format", "text", cursorContextPrompt()}, nil)
	if err != nil {
		return nil, err
	}
	return extractJSONObject(out)
}

func (cursorBridge) Feed(ctx *RestoredCheckoutContext) error {
	prompt, err := feedPromptContext(ctx)
	if err != nil {
		return err
	}

	cmdArgs := []string{"-p", "--trust", "--output-format", "text"}
	sessionID, notice := resolveFeedSession(cursorBridge{}, cursorSessionID())
	if sessionID != "" {
		cmdArgs = append(cmdArgs, "--resume", sessionID)
	} else {
		cmdArgs = append(cmdArgs, "--continue")
	}
	cmdArgs = append(cmdArgs, prompt)

	if err := runBridgeFeed(cursorBridge{}, cmdArgs, nil); err != nil {
		return err
	}
	if notice != "" {
		fmt.Println(notice)
	}
	return nil
}

func cursorAgentExecutable() string {
	return bridgeExecutableName("cursor-agent", ".exe")
}

func cursorContextPrompt() string {
	return bridgeContextPrompt("cursor", "Use the current repository state (git status, recent commits, diffs).")
}

func cursorSessionID() string {
	return bridgeSessionID("AGIT_CURSOR_SESSION_ID", "agit.cursorSession")
}
