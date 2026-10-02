package agit

import (
	"fmt"
	"strings"

	gitwrap "github.com/Satari4545/ai-git/internal/git"
)

const NotesRef = "refs/notes/agit"

// Plain `git fetch` never fetches refs/notes/agit unless the refspec is
// configured; without the explicit notes fetch below, context notes silently
// never reach the receiving machine and the handoff loop breaks there.
func Fetch(args []string) error {
	if err := gitwrap.Run(append([]string{"fetch"}, args...)...); err != nil {
		return err
	}
	return fetchNotes(remoteFromArgs(args))
}

func Pull(args []string) error {
	if err := gitwrap.Run(append([]string{"pull"}, args...)...); err != nil {
		return err
	}
	return fetchNotes(remoteFromArgs(args))
}

// valueFlags are fetch/pull flags that consume the following arg, so the
// next arg is not the remote name.
var valueFlags = map[string]bool{"--depth": true, "--deepen": true}

// remoteFromArgs returns the remote name from push/fetch/pull args,
// defaulting to "origin".
func remoteFromArgs(args []string) string {
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

// A remote with no agit notes is not an error: it means no context exists yet.
func fetchNotes(remote string) error {
	if !hasRemote(remote) {
		fmt.Printf("agit: no '%s' remote configured, skipping logic notes\n", remote)
		return nil
	}
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

func hasRemote(remote string) bool {
	out, err := gitwrap.OutputQuiet("remote")
	if err != nil {
		return false
	}
	for _, r := range strings.Fields(out) {
		if r == remote {
			return true
		}
	}
	return false
}
