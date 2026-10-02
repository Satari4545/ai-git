package agit

import (
	"fmt"
	"os/exec"
	"strings"

	gitwrap "github.com/Satari4545/ai-git/internal/git"
)

// Without this refspec, plain `git fetch` silently skips agit notes;
// `agit fetch` covers the explicit path, this covers the plain-git path.
const notesFetchRefspec = "+" + NotesRef + ":" + NotesRef

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

func ensureNotesFetchRefspec() error {
	remotes, err := gitwrap.OutputQuiet("remote")
	if err != nil {
		return err
	}
	names := strings.Fields(remotes)
	if len(names) == 0 {
		fmt.Println("agit: no remotes, skipping notes refspec (agit fetch still syncs notes)")
		return nil
	}
	for _, name := range names {
		if err := ensureRemoteNotesRefspec(name); err != nil {
			return err
		}
	}
	return nil
}

func ensureRemoteNotesRefspec(name string) error {
	existing, _ := gitwrap.OutputQuiet("config", "--get-all", "remote."+name+".fetch")
	for _, line := range strings.Split(existing, "\n") {
		if strings.Contains(line, NotesRef) {
			fmt.Printf("agit: notes refspec already configured for '%s'\n", name)
			return nil
		}
	}
	if err := gitwrap.Run("config", "--add", "remote."+name+".fetch", notesFetchRefspec); err != nil {
		return err
	}
	fmt.Printf("agit: configured '%s' to fetch agit notes on plain git fetch\n", name)
	return nil
}

func printQuickstart() {
	fmt.Println()
	fmt.Println("agit is ready. 5-minute handoff loop:")
	fmt.Println()
	fmt.Println("  1. Connect your agent:  " + mcpInstallSuggestion())
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

// mcpInstallSuggestion suggests the `agit mcp install` target matching the
// agent CLIs found on PATH, so init points at the agent actually installed
// instead of guessing. It reuses the bridges' registered executables.
func mcpInstallSuggestion() string {
	var found []string
	for _, target := range MCPInstallTargets() {
		exe, err := mcpTargetExecutable(target)
		if err != nil {
			continue
		}
		if _, err := exec.LookPath(exe); err == nil {
			found = append(found, target)
		}
	}
	switch len(found) {
	case 1:
		return "agit mcp install " + found[0]
	case 0:
		return "agit mcp install <agent>   (e.g. " + strings.Join(MCPInstallTargets(), ", ") + ")"
	default:
		return "agit mcp install <agent>   (detected on PATH: " + strings.Join(found, ", ") + ")"
	}
}
