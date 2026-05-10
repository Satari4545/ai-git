package agit

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	gitwrap "github.com/Satari4545/AGIT/internal/git"
	"github.com/Satari4545/AGIT/internal/state"
)

func Commit(args []string) error {
	agentCtx, hasAgentCtx, err := LoadAgentContext()
	if err != nil {
		return err
	}

	commitArgs := stripAgitArgs(args)

	if !hasCommitMessage(commitArgs) && hasAgentCtx && agentCtx.CommitMessageSuggestion != "" {
		commitArgs = append(commitArgs, "-m", agentCtx.CommitMessageSuggestion)
	}

	intent := extractIntent(args)

	if intent == "" && hasAgentCtx {
		intent = agentCtx.IntentSummary
	}

	_, contextID, rawState, err := state.Capture(intent)
	if err != nil {
		return err
	}

	notePayload := map[string]any{
		"contextID": contextID,
		"logicState": json.RawMessage(rawState),
	}

	if hasAgentCtx {
		notePayload["agentContext"] = agentCtx
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
	tmp.Close()

	if err := gitwrap.Run("notes", "--ref=agit", "add", "-f", "-F", tmp.Name(), head); err != nil {
		return err
	}

	fmt.Println("agit: attached context note")
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