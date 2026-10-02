package agit

func init() {
	RegisterBridge(piBridge{})
	registerBridgeExecutable("pi", piExecutable)
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
	return installBridgeProvider(piBridge{})
}

func (piBridge) Provide() ([]byte, error) {
	out, err := runBridgeProvide(piBridge{},
		piPromptArgs(piContextPrompt()), []string{"PI_SKIP_VERSION_CHECK=1"})
	if err != nil {
		return nil, err
	}
	return extractJSONObject(out)
}

func (piBridge) Feed(ctx *RestoredCheckoutContext) error {
	prompt, err := feedPromptContext(ctx)
	if err != nil {
		return err
	}

	return runBridgeFeed(piBridge{}, piPromptArgs(prompt), []string{"PI_SKIP_VERSION_CHECK=1"})
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
	return bridgeExecutableName("pi", ".cmd")
}

func piSessionID() string {
	return bridgeSessionID("AGIT_PI_SESSION_ID", "agit.piSession")
}

func piContextPrompt() string {
	return bridgeContextPrompt("pi", "Use the current Pi session and current repository state.")
}
