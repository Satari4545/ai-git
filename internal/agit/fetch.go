package agit

import (
	"fmt"
	"strings"

	gitwrap "github.com/Satari4545/ai-git/internal/git"
)

// NotesRef is the git notes ref agit uses for context payloads.
const NotesRef = "refs/notes/agit"

// Fetch runs git fetch with the given args, then fetches the agit notes ref
// from the same remote. Plain `git fetch` does not fetch notes refs unless
// the refspec is configured, so without this step context notes never reach
// the receiving machine and the multi-agent handoff loop breaks there.
func Fetch(args []string) error {
	if err := gitwrap.Run(append([]string{"fetch"}, args...)...); err != nil {
		return err
	}
	return fetchNotes(firstNonFlagArg(args))
}

// Pull runs git pull with the given args, then fetches the agit notes ref
// from the same remote so context notes stay in sync with pulled commits.
func Pull(args []string) error {
	if err := gitwrap.Run(append([]string{"pull"}, args...)...); err != nil {
		return err
	}
	return fetchNotes(firstNonFlagArg(args))
}

// valueFlags are fetch/pull flags that consume the following arg, so the
// next arg is not the remote name.
var valueFlags = map[string]bool{"--depth": true, "--deepen": true}

// firstNonFlagArg returns the first arg that is not a flag, which for fetch
// and pull is the remote name. Defaults to "origin".
func firstNonFlagArg(args []string) string {
	skipNext := false
	for _, a := range args {
		if skipNext {
			skipNext = false
			continue
		}
		if a == "" {
			continue
		}
		if strings.HasPrefix(a, "-") {
			skipNext = valueFlags[a]
			continue
		}
		return a
	}
	return "origin"
}

// fetchNotes fetches NotesRef from remote. A remote that has never seen agit
// notes is not an error: it just means there is no context to restore yet.
func fetchNotes(remote string) error {
	fmt.Println("agit: fetching logic notes")
	out, err := gitwrap.OutputQuiet("ls-remote", remote, NotesRef)
	if err != nil {
		return err
	}
	if out == "" {
		fmt.Println("agit: no logic notes on remote")
		return nil
	}
	if err := gitwrap.Run("fetch", remote, NotesRef+":"+NotesRef); err != nil {
		return err
	}
	fmt.Println("agit: fetched logic notes")
	return nil
}
