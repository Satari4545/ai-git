package agit

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	gitwrap "github.com/Satari4545/ai-git/internal/git"
)

// AgentBridge is one coding-agent integration. It knows how to wire agit
// into the agent (Install), how to deliver restored checkout context to a
// running session (Feed), and how to ask the agent for fresh context JSON
// at commit time (Provide).
//
// New agents plug in by implementing this interface and calling
// RegisterBridge, usually from an init function in their own file.
// Nothing else in agit needs to change.
type AgentBridge interface {
	// Name is the CLI identifier, e.g. "codex".
	Name() string
	// DisplayName is the human-facing name used in prompts, e.g. "Codex".
	DisplayName() string
	// Install wires agit into the agent: provider command, native config,
	// and selecting this bridge via `git config agit.bridge`.
	Install() error
	// Feed delivers restored checkout context to the agent.
	Feed(ctx *RestoredCheckoutContext) error
	// Provide asks the agent for fresh context JSON. The caller parses
	// and normalizes the response; only the agent invocation differs.
	Provide() ([]byte, error)
}

var bridgeRegistry = map[string]AgentBridge{}

// RegisterBridge makes an agent bridge available under its Name.
func RegisterBridge(b AgentBridge) {
	bridgeRegistry[b.Name()] = b
}

// GetBridge returns the registered bridge for name, or an error naming
// 'agit bridge list' as the remedy.
func GetBridge(name string) (AgentBridge, error) {
	b, ok := bridgeRegistry[name]
	if !ok {
		return nil, fmt.Errorf("unknown bridge %q (see 'agit bridge list')", name)
	}
	return b, nil
}

// BridgeNames returns the registered bridge names, sorted.
func BridgeNames() []string {
	names := make([]string, 0, len(bridgeRegistry))
	for name := range bridgeRegistry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ActiveBridge returns the bridge selected by `git config agit.bridge`.
// It defaults to codex, preserving the behavior from before the registry.
func ActiveBridge() (AgentBridge, error) {
	name, _ := gitwrap.OutputQuiet("config", "--get", "agit.bridge")
	name = strings.TrimSpace(name)
	if name == "" {
		name = "codex"
	}
	b, err := GetBridge(name)
	if err != nil {
		return nil, fmt.Errorf("invalid agit.bridge %q: %w", name, err)
	}
	return b, nil
}

const bridgeUsage = `usage: agit bridge list
       agit bridge <agent> install|provider|feed
       agit bridge codex session set <id>|get|unset
       agit bridge claude-code hook`

// Bridge implements `agit bridge`: generic verbs dispatch through the
// registry, while agent-specific verbs (codex session) keep working.
func Bridge(args []string) error {
	if len(args) == 0 {
		fmt.Println(bridgeUsage)
		return nil
	}

	if args[0] == "list" {
		for _, name := range BridgeNames() {
			fmt.Println(name)
		}
		return nil
	}

	name := args[0]

	// Codex session management predates the registry; keep it working.
	if name == "codex" && len(args) > 1 && args[1] == "session" {
		return codexBridgeSession(args[2:])
	}

	// The Claude Code SessionStart hook entry point.
	if name == "claude-code" && len(args) > 1 && args[1] == "hook" {
		return claudeCodeHook()
	}

	b, err := GetBridge(name)
	if err != nil {
		return fmt.Errorf("%w\n%s", err, bridgeUsage)
	}

	if len(args) < 2 {
		fmt.Println(bridgeUsage)
		return nil
	}

	switch args[1] {
	case "install":
		return b.Install()
	case "feed":
		return feedViaBridge(b)
	case "provider":
		return provideViaBridge(b)
	default:
		fmt.Println(bridgeUsage)
		return nil
	}
}

func feedViaBridge(b AgentBridge) error {
	ctx, err := LoadCheckoutContext()
	if err != nil {
		return err
	}
	return b.Feed(ctx)
}

func provideViaBridge(b AgentBridge) error {
	raw, err := b.Provide()
	if err != nil {
		return err
	}
	normalized, err := normalizeProviderContext(raw, b.Name())
	if err != nil {
		return err
	}
	fmt.Println(string(normalized))
	return nil
}

// normalizeProviderContext parses the agent's raw response and stamps the
// bridge name as provider when the agent did not name one itself.
func normalizeProviderContext(raw []byte, providerName string) ([]byte, error) {
	ctx, err := ParseAgentContextJSON(raw)
	if err != nil {
		return nil, err
	}
	ctx.Provider = firstNonEmpty(ctx.Provider, providerName)
	return json.Marshal(ctx)
}

// feedCheckoutContext delivers the restored checkout context to the
// currently configured agent bridge.
func feedCheckoutContext() error {
	b, err := ActiveBridge()
	if err != nil {
		return err
	}
	return feedViaBridge(b)
}
