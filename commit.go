package agit

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	gitwrap "github.com/yourname/agit/internal/git"
	"github.com/yourname/agit/internal/state"
)

func Commit(args []string) error {
	intent := extractIntent(args)
	_, contextID, rawState, err := state.Capture(intent)
	if err != nil {
		return err
	}

	// Fast path: only one JSON capture + normal git commit.
	if err := gitwrap.Run(append([]string{"commit"}, stripAgitArgs(args)...)...); err != nil {
		return err
	}

	head, err := gitwrap.Output("rev-parse", "HEAD")
	if err != nil {
		return err
	}

	notePayload := map[string]any{
		"contextID": contextID,
		"state":     json.RawMessage(rawState),
	}

	noteBytes, _ := json.MarshalIndent(notePayload, "", "  ")

	tmp, err := os.CreateTemp("", "agit-note-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(noteBytes); err != nil {
		return err
	}
	tmp.Close()

	return gitwrap.Run("notes", "--ref=agit", "add", "-f", "-F", tmp.Name(), head)
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