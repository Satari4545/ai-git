package agit

import "fmt"

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
	return installBridgeProvider(opencodeBridge{})
}

func (opencodeBridge) Provide() ([]byte, error) {
	out, err := runBridgeProvide(opencodeBridge{},
		[]string{"run", opencodeContextPrompt()}, nil)
	if err != nil {
		return nil, err
	}
	return extractJSONObject(out)
}

func (opencodeBridge) Feed(ctx *RestoredCheckoutContext) error {
	prompt, err := feedPromptContext(ctx)
	if err != nil {
		return err
	}

	cmdArgs := []string{"run"}
	sessionID, notice := resolveFeedSession(opencodeBridge{}, opencodeSessionID())
	if sessionID != "" {
		cmdArgs = append(cmdArgs, "--session", sessionID)
	} else {
		cmdArgs = append(cmdArgs, "--continue")
	}
	cmdArgs = append(cmdArgs, prompt)

	if err := runBridgeFeed(opencodeBridge{}, cmdArgs, nil); err != nil {
		return err
	}
	if notice != "" {
		fmt.Println(notice)
	}
	return nil
}

func opencodeExecutable() string {
	return bridgeExecutableName("opencode", ".cmd")
}

func opencodeContextPrompt() string {
	return bridgeContextPrompt("opencode", "Use the current repository state (git status, recent commits, diffs).")
}

func opencodeSessionID() string {
	return bridgeSessionID("AGIT_OPENCODE_SESSION_ID", "agit.opencodeSession")
}
