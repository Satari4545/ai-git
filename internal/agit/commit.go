package agit

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	gitwrap "github.com/Satari4545/ai-git/internal/git"
	"github.com/Satari4545/ai-git/internal/state"
)

func Commit(args []string) error {
	quiet := hasAgitFlag(args, "--quiet")

	if !quiet {
		fmt.Println("agit: preparing commit context")
	}

	refreshedCtx, err := RefreshContextFromProvider(args)
	if err != nil {
		return err
	}
	if refreshedCtx && !quiet {
		fmt.Println("agit: refreshed agent context from provider")
	}

	agentCtx, hasAgentCtx, err := LoadAgentContext()
	if err != nil {
		return err
	}

	if !hasAgentCtx && !hasAgitFlag(args, "--allow-missing-context") {
		return fmt.Errorf("missing agent context. Configure an agent provider with 'git config agit.contextCommand <command>', connect the AGIT MCP server so your agent can call record_context, or commit with '--allow-missing-context'")
	}

	if hasAgentCtx {
		if err := validateContextFreshness(agentCtx, args); err != nil {
			return err
		}
	}

	commitArgs := stripAgitArgs(args)

	if !hasCommitMessage(commitArgs) && hasAgentCtx && agentCtx.CommitMessageSuggestion != "" {
		commitArgs = append(commitArgs, "-m", agentCtx.CommitMessageSuggestion)
	}

	intent := extractIntent(args)

	if intent == "" && hasAgentCtx {
		intent = agentCtx.IntentSummary
	}

	if intent == "" {
		intent = extractCommitMessage(commitArgs)
	}

	if !quiet {
		fmt.Println("agit: capturing staged Git state")
	}

	_, contextID, rawState, err := state.Capture(intent)
	if err != nil {
		return err
	}

	commitArgs, includedContextID := appendContextIDTrailer(commitArgs, contextID)

	notePayload := map[string]any{
		"contextID":  contextID,
		"logicState": json.RawMessage(rawState),
	}

	if hasAgentCtx {
		notePayload["agentContext"] = agentCtx
	}

	if !quiet {
		fmt.Println("agit: creating Git commit")
	}

	if err := gitwrap.Run(append([]string{"commit"}, commitArgs...)...); err != nil {
		return err
	}

	head, err := gitwrap.Output("rev-parse", "HEAD")
	if err != nil {
		return err
	}

	noteBytes, err := json.MarshalIndent(notePayload, "", "  ")
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp("", "agit-note-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(noteBytes); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	if !quiet {
		fmt.Println("agit: attaching context note")
	}

	if err := gitwrap.Run("notes", "--ref=agit", "add", "-f", "-F", tmp.Name(), head); err != nil {
		return err
	}

	fmt.Println("agit: attached context note")
	if includedContextID {
		fmt.Println("agit: embedded context ID in commit message")
	}
	fmt.Println("agit: ContextID:", contextID)

	return nil
}

func DebugHEAD() error {
	head, err := gitwrap.Output("rev-parse", "HEAD")
	if err != nil {
		return err
	}

	note, err := gitwrap.Output("notes", "--ref=agit", "show", head)
	if err != nil {
		fmt.Println("No agit note found on HEAD.")
		return nil
	}

	fmt.Println("AGIT DEBUG")
	fmt.Println("Commit:", head)
	fmt.Println(note)
	return nil
}

func extractIntent(args []string) string {
	for i, a := range args {
		if a == "--intent" && i+1 < len(args) {
			return args[i+1]
		}
		if strings.HasPrefix(a, "--intent=") {
			return strings.TrimPrefix(a, "--intent=")
		}
	}

	return os.Getenv("AGIT_INTENT")
}

func stripAgitArgs(args []string) []string {
	out := []string{}
	skip := false

	for _, a := range args {
		if skip {
			skip = false
			continue
		}

		if a == "--intent" {
			skip = true
			continue
		}

		if strings.HasPrefix(a, "--intent=") {
			continue
		}

		if a == "--allow-stale-context" {
			continue
		}

		if a == "--allow-missing-context" {
			continue
		}

		if a == "--no-agent-context" {
			continue
		}

		if a == "--quiet" {
			continue
		}

		out = append(out, a)
	}

	return out
}

func hasCommitMessage(args []string) bool {
	for i, a := range args {
		if a == "-m" || a == "--message" {
			return i+1 < len(args)
		}

		if strings.HasPrefix(a, "-m") && len(a) > 2 {
			return true
		}

		if strings.HasPrefix(a, "--message=") {
			return true
		}
	}

	return false
}

func appendContextIDTrailer(args []string, contextID string) ([]string, bool) {
	if contextID == "" || !hasCommitMessage(args) || hasCommitMessageFile(args) {
		return args, false
	}

	return append(args, "-m", "Agit-Context-ID: "+contextID), true
}

func hasCommitMessageFile(args []string) bool {
	for i, a := range args {
		if a == "-F" || a == "--file" {
			return i+1 < len(args)
		}

		if strings.HasPrefix(a, "-F") && len(a) > 2 {
			return true
		}

		if strings.HasPrefix(a, "--file=") {
			return true
		}
	}

	return false
}

func extractCommitMessage(args []string) string {
	for i, a := range args {
		if a == "-m" || a == "--message" {
			if i+1 < len(args) {
				return args[i+1]
			}
		}

		if strings.HasPrefix(a, "-m") && len(a) > 2 {
			return strings.TrimPrefix(a, "-m")
		}

		if strings.HasPrefix(a, "--message=") {
			return strings.TrimPrefix(a, "--message=")
		}
	}

	return ""
}

func errorhandler(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "agit error:", err)
		os.Exit(1)
	}
}

func validateContextFreshness(ctx *AgentContext, args []string) error {
	if hasAgitFlag(args, "--allow-stale-context") {
		return nil
	}

	if ctx.CreatedAt == "" {
		return fmt.Errorf("agent context has no createdAt timestamp. Refresh it with: agit mcp record-context ...")
	}

	createdAt, err := time.Parse(time.RFC3339, ctx.CreatedAt)
	if err != nil {
		return fmt.Errorf("invalid agent context createdAt timestamp: %s", ctx.CreatedAt)
	}

	maxAge := 10 * time.Minute
	age := time.Since(createdAt)

	if age > maxAge {
		return fmt.Errorf(
			"agent context is stale (%s old). Refresh it with 'agit mcp record-context ...' or commit with '--allow-stale-context'",
			age.Round(time.Minute),
		)
	}

	return nil
}

func hasAgitFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}
