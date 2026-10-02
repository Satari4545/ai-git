package agit

import (
	"fmt"
	"strings"

	gitwrap "github.com/Satari4545/ai-git/internal/git"
)

// notesFetchRefspec makes plain `git fetch` also fetch agit notes.
// It complements `agit fetch`, which fetches notes explicitly.
const notesFetchRefspec = "+refs/notes/agit:refs/notes/agit"

// Init sets up the current directory for agit: it initializes a git repo if
// needed, wires the notes refspec so plain git fetch carries context notes,
// and prints the 5-minute quickstart. It is idempotent.
func Init() error {
	if !inGitRepo() {
		fmt.Println("agit: no git repo here, running git init")
		if err := gitwrap.Run("init", "-q"); err != nil {
			return err
		}
	}

	if err := ensureNotesFetchRefspec(); err != nil {
		return err
	}

	printQuickstart()
	return nil
}

func inGitRepo() bool {
	out, err := gitwrap.OutputQuiet("rev-parse", "--git-dir")
	return err == nil && out != ""
}

// ensureNotesFetchRefspec adds the agit notes refspec to origin's fetch
// config so plain `git fetch` also brings context notes. It is a no-op when
// there is no origin remote or the refspec is already present.
func ensureNotesFetchRefspec() error {
	remotes, err := gitwrap.OutputQuiet("remote")
	if err != nil {
		return err
	}
	hasOrigin := false
	for _, r := range strings.Fields(remotes) {
		if r == "origin" {
			hasOrigin = true
			break
		}
	}
	if !hasOrigin {
		fmt.Println("agit: no origin remote, skipping notes refspec (agit fetch still syncs notes)")
		return nil
	}

	existing, _ := gitwrap.OutputQuiet("config", "--get-all", "remote.origin.fetch")
	for _, line := range strings.Split(existing, "\n") {
		if strings.TrimSpace(line) == notesFetchRefspec {
			fmt.Println("agit: notes refspec already configured")
			return nil
		}
	}

	if err := gitwrap.Run("config", "--add", "remote.origin.fetch", notesFetchRefspec); err != nil {
		return err
	}
	fmt.Println("agit: configured origin to fetch agit notes on plain git fetch")
	return nil
}

func printQuickstart() {
	fmt.Println()
	fmt.Println("agit is ready. 5-minute handoff loop:")
	fmt.Println()
	fmt.Println("  1. Connect your agent:  agit mcp install codex   (or: claude)")
	fmt.Println("  2. Agent records work:  via the record_context MCP tool")
	fmt.Println("  3. Commit with context: agit commit -m \"...\"")
	fmt.Println("  4. Share it:            agit push")
	fmt.Println()
	fmt.Println("Continue in a new session or machine:")
	fmt.Println()
	fmt.Println("  5. Fetch the context:   agit fetch   (or: agit pull)")
	fmt.Println("  6. Restore it:          agit checkout <branch>")
	fmt.Println("  7. Agent reads it:      via the get_context MCP tool")
	fmt.Println()
	fmt.Println("Run `agit --help` for all commands.")
}
