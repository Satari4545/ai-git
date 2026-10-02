package agit

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	gitwrap "github.com/Satari4545/ai-git/internal/git"
)

type RestoredCheckoutContext struct {
	SchemaVersion string          `json:"schemaVersion"`
	Commit        string          `json:"commit"`
	Branch        string          `json:"branch,omitempty"`
	RestoredAt    string          `json:"restoredAt"`
	Note          json.RawMessage `json:"note"`
}

func Checkout(args []string) error {
	gitArgs, feedOverride := stripCheckoutAgitArgs(args)

	if err := gitwrap.Run(append([]string{"checkout"}, gitArgs...)...); err != nil {
		return err
	}

	restored, err := RestoreHEADContext()
	if err != nil {
		return err
	}

	if restored {
		fmt.Println("agit: restored checkout context from HEAD")
		// Restore and feed are decoupled: the restored context stays in
		// checkout.json no matter the feed decision, so get_context and
		// agent hooks keep working even when the feed is skipped.
		if ShouldFeedCheckoutContext(feedOverride) {
			feedCheckoutContextOrWarn()
		}
	} else {
		fmt.Println("agit: no AGIT context note found on HEAD")
	}

	return nil
}

func stripCheckoutAgitArgs(args []string) ([]string, string) {
	gitArgs := []string{}
	feedOverride := ""

	for _, arg := range args {
		switch arg {
		case "--feed-context":
			feedOverride = "enabled"
		case "--no-feed-context":
			feedOverride = "disabled"
		default:
			gitArgs = append(gitArgs, arg)
		}
	}

	return gitArgs, feedOverride
}

// ShouldFeedCheckoutContext decides whether the restored checkout context
// is fed into the active agent bridge. Explicit flags win, then the
// agit.checkoutFeed config. The prompt only appears when it was explicitly
// configured and stdin is a terminal: auto mode never reads stdin, so
// scripted, piped, and agent-driven checkouts cannot hang or fail on it.
func ShouldFeedCheckoutContext(feedOverride string) bool {
	switch feedOverride {
	case "enabled":
		return true
	case "disabled":
		return false
	}

	switch CheckoutFeedMode() {
	case "enabled":
		return true
	case "disabled":
		return false
	case "prompt":
		return stdinIsTTY() && promptForCheckoutFeed()
	default:
		return false
	}
}

func stdinIsTTY() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func promptForCheckoutFeed() bool {
	name := "your agent"
	if b, err := ActiveBridge(); err == nil {
		name = b.DisplayName()
	}
	fmt.Printf("Feed restored AGIT context to %s? [y/N] ", name)

	reader := bufio.NewReader(os.Stdin)
	answer, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "y" || answer == "yes"
}

// feedCheckoutContextOrWarn delivers the restored checkout context to the
// configured agent bridge when one resolves. With no bridge it skips
// silently; a failed feed is a stderr warning, never a checkout failure,
// because the restored state remains readable via get_context regardless.
func feedCheckoutContextOrWarn() {
	b, err := ActiveBridge()
	if err != nil {
		return
	}
	if err := feedViaBridge(b); err != nil {
		fmt.Fprintf(os.Stderr, "agit: warning: could not feed checkout context to %s: %v\n", b.DisplayName(), err)
	}
}

func RestoreHEADContext() (bool, error) {
	head, err := gitwrap.OutputQuiet("rev-parse", "HEAD")
	if err != nil {
		return false, err
	}

	note, err := gitwrap.OutputQuiet("notes", "--ref=agit", "show", head)
	if err != nil || note == "" {
		// No note: clear any stale restored state so later reads
		// (manual feed verbs, agent hooks) never see a previous checkout.
		_ = os.Remove(checkoutStatePath())
		return false, nil
	}

	var rawNote json.RawMessage
	if err := json.Unmarshal([]byte(note), &rawNote); err != nil {
		return false, fmt.Errorf("invalid AGIT note on HEAD: %w", err)
	}

	branch, _ := gitwrap.OutputQuiet("branch", "--show-current")

	restored := RestoredCheckoutContext{
		SchemaVersion: "agit.checkout-context.v0",
		Commit:        head,
		Branch:        branch,
		RestoredAt:    time.Now().UTC().Format(time.RFC3339),
		Note:          rawNote,
	}

	raw, err := json.MarshalIndent(restored, "", "  ")
	if err != nil {
		return false, err
	}

	if err := os.MkdirAll(filepath.Join(".agit", "state"), 0755); err != nil {
		return false, err
	}

	path := checkoutStatePath()
	if err := os.WriteFile(path, raw, 0644); err != nil {
		return false, err
	}

	return true, nil
}

// checkoutStatePath is the well-known location of the restored checkout
// context. The file exists if and only if the last checkout restored a
// note from HEAD, regardless of the feed decision, so hooks, manual feed
// verbs, and the get_context MCP tool can read it whenever context was
// restored.
func checkoutStatePath() string {
	return filepath.Join(".agit", "state", "checkout.json")
}

func LoadCheckoutContextRaw() ([]byte, bool, error) {
	path := checkoutStatePath()

	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	return raw, true, nil
}

// LoadCheckoutContext returns the restored checkout context as a struct.
// It errors when nothing was restored, so feeders can report that instead
// of silently doing nothing.
func LoadCheckoutContext() (*RestoredCheckoutContext, error) {
	raw, ok, err := LoadCheckoutContextRaw()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("no checkout context restored; run agit checkout on a commit with AGIT notes first")
	}

	var ctx RestoredCheckoutContext
	if err := json.Unmarshal(raw, &ctx); err != nil {
		return nil, fmt.Errorf("invalid checkout context: %w", err)
	}
	return &ctx, nil
}

func PrintCheckoutContext() error {
	raw, ok, err := LoadCheckoutContextRaw()
	if err != nil {
		return err
	}

	if !ok {
		fmt.Println("No checkout context has been restored. Run `agit checkout <branch>` on a commit with AGIT notes.")
		return nil
	}

	fmt.Println(string(raw))
	return nil
}
