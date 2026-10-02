package agit

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	gitwrap "github.com/Satari4545/ai-git/internal/git"
)

type doctorStatus string

const (
	doctorPass doctorStatus = "PASS"
	doctorWarn doctorStatus = "WARN"
	doctorFail doctorStatus = "FAIL"
)

type doctorCheck struct {
	name string
	run  func() (doctorStatus, string)
}

var errDoctorFailed = errors.New("one or more doctor checks failed")

// Doctor runs the setup checklist and prints PASS/WARN/FAIL per item with a
// one-line remediation hint. It exits 0 when nothing FAILs; WARN covers
// degraded-but-working setups. Without a git repo the remaining checks
// cannot run, so that check gates the rest.
func Doctor() error {
	status, detail := checkDoctorGitRepo()
	printDoctorResult("git repository", status, detail)
	if status != doctorPass {
		return errDoctorFailed
	}

	checks := []doctorCheck{
		{"agit init (notes refspec)", checkDoctorNotesRefspec},
		{"bridge configured", checkDoctorBridgeConfigured},
		{"bridge CLI on PATH", checkDoctorBridgeCLI},
		{"MCP server installed", checkDoctorMCPInstalled},
		{"local agit notes", checkDoctorLocalNotes},
		{"restored checkout context", checkDoctorCheckoutContext},
	}
	failed := false
	for _, c := range checks {
		status, detail := c.run()
		printDoctorResult(c.name, status, detail)
		if status == doctorFail {
			failed = true
		}
	}
	if failed {
		return errDoctorFailed
	}
	return nil
}

func printDoctorResult(name string, status doctorStatus, detail string) {
	fmt.Printf("[%s] %s\n", status, name)
	if detail != "" {
		fmt.Printf("       %s\n", detail)
	}
}

func checkDoctorGitRepo() (doctorStatus, string) {
	if inGitRepo() {
		return doctorPass, ""
	}
	return doctorFail, "not in a git repo — run `agit init` (it runs git init for you)"
}

func checkDoctorNotesRefspec() (doctorStatus, string) {
	remotes, _ := gitwrap.OutputQuiet("remote")
	if strings.TrimSpace(remotes) == "" {
		return doctorPass, "no remotes configured; `agit fetch`/`agit push` sync notes explicitly"
	}
	fetch, _ := gitwrap.OutputQuiet("config", "--get-regexp", `^remote\..*\.fetch$`)
	for _, line := range strings.Split(fetch, "\n") {
		if strings.Contains(line, NotesRef) {
			return doctorPass, ""
		}
	}
	return doctorWarn, "plain `git fetch` will skip agit notes — run `agit init` to add the notes refspec"
}

func checkDoctorBridgeConfigured() (doctorStatus, string) {
	name, _ := gitwrap.OutputQuiet("config", "--get", "agit.bridge")
	name = strings.TrimSpace(name)
	if name == "" {
		return doctorWarn, "agit.bridge not set (defaults to codex) — run `agit bridge <agent> install` to select your agent"
	}
	b, err := GetBridge(name)
	if err != nil {
		return doctorFail, fmt.Sprintf("invalid agit.bridge %q — pick one from `agit bridge list`", name)
	}
	return doctorPass, fmt.Sprintf("%s (%s)", b.Name(), b.DisplayName())
}

func checkDoctorBridgeCLI() (doctorStatus, string) {
	b, err := ActiveBridge()
	if err != nil {
		return doctorFail, err.Error()
	}
	exe, err := BridgeExecutable(b.Name())
	if err != nil {
		return doctorFail, err.Error()
	}
	if _, err := exec.LookPath(exe); err != nil {
		return doctorWarn, fmt.Sprintf("%s CLI %q not found on PATH — install %s or run `agit bridge <agent> install` for another agent", b.DisplayName(), exe, b.DisplayName())
	}
	return doctorPass, exe
}

func checkDoctorMCPInstalled() (doctorStatus, string) {
	b, err := ActiveBridge()
	if err != nil {
		return doctorFail, err.Error()
	}
	installed, where, err := bridgeMCPInstalled(b.Name())
	if err != nil {
		return doctorWarn, fmt.Sprintf("could not verify MCP config: %v", err)
	}
	if installed {
		return doctorPass, where
	}
	return doctorWarn, fmt.Sprintf("agit MCP server not found for %s — run `agit mcp install %s`", b.DisplayName(), mcpInstallTarget(b.Name()))
}

// bridgeMCPInstalled checks, read-only, whether the agit MCP server entry
// exists in the bridge's config, reusing the installers' path logic.
func bridgeMCPInstalled(bridgeName string) (installed bool, where string, err error) {
	switch bridgeName {
	case "codex":
		path, err := codexMCPConfigPath()
		if err != nil {
			return false, "", err
		}
		raw, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			return false, "", nil
		}
		if err != nil {
			return false, "", err
		}
		return strings.Contains(string(raw), "[mcp_servers.agit]"), path, nil
	case "claude-code":
		exe, err := BridgeExecutable("claude-code")
		if err != nil {
			return false, "", err
		}
		if _, err := exec.LookPath(exe); err != nil {
			return false, "", fmt.Errorf("%s not on PATH", exe)
		}
		out, err := exec.Command(exe, "mcp", "list").CombinedOutput()
		if err != nil {
			return false, "", fmt.Errorf("claude mcp list failed: %w", err)
		}
		return strings.Contains(string(out), "agit"), exe + " mcp list", nil
	case "pi":
		var paths []string
		if dir, err := piAgentDir(); err == nil {
			paths = append(paths, filepath.Join(dir, "mcp.json"))
		}
		if top, err := gitTopLevel(); err == nil {
			paths = append(paths, filepath.Join(top, ".pi", "mcp.json"))
		}
		for _, path := range paths {
			ok, where, err := jsonMCPHasAgit(path)
			if err != nil {
				return false, "", err
			}
			if ok {
				return true, where, nil
			}
		}
		return false, "", nil
	case "cursor":
		path, err := cursorMCPConfigPath()
		if err != nil {
			return false, "", err
		}
		return jsonMCPHasAgit(path)
	case "gemini":
		path, err := geminiMCPConfigPath()
		if err != nil {
			return false, "", err
		}
		return jsonMCPHasAgit(path)
	case "opencode":
		path, err := opencodeConfigPath()
		if err != nil {
			return false, "", err
		}
		return jsonMCPHasAgitKey(path, "mcp")
	default:
		return false, "", fmt.Errorf("no MCP check for bridge %q", bridgeName)
	}
}

// jsonMCPHasAgit reports whether the JSON MCP config at path declares the
// agit server under mcpServers. A missing file means "not installed", not
// an error.
func jsonMCPHasAgit(path string) (bool, string, error) {
	return jsonMCPHasAgitKey(path, "mcpServers")
}

// jsonMCPHasAgitKey reports whether the JSON MCP config at path declares
// the agit server under serversKey ("mcpServers" for most clients, "mcp"
// for OpenCode). A missing file means "not installed", not an error.
func jsonMCPHasAgitKey(path string, serversKey string) (bool, string, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, "", nil
	}
	if err != nil {
		return false, "", err
	}
	var config map[string]any
	if err := json.Unmarshal(raw, &config); err != nil {
		return false, "", fmt.Errorf("%s is not valid JSON: %w", path, err)
	}
	servers, _ := config[serversKey].(map[string]any)
	if _, ok := servers["agit"]; ok {
		return true, path, nil
	}
	return false, "", nil
}

func gitTopLevel() (string, error) {
	top, err := gitwrap.OutputQuiet("rev-parse", "--show-toplevel")
	if err != nil || top == "" {
		return "", errors.New("not in a git repo")
	}
	return top, nil
}

func checkDoctorLocalNotes() (doctorStatus, string) {
	if err := gitwrap.Run("show-ref", "--verify", "--quiet", NotesRef); err != nil {
		return doctorWarn, "no local agit notes yet — `agit commit` with context creates them, `agit fetch` syncs them"
	}
	return doctorPass, ""
}

func checkDoctorCheckoutContext() (doctorStatus, string) {
	raw, ok, err := LoadCheckoutContextRaw()
	if err != nil {
		return doctorWarn, fmt.Sprintf("could not read checkout state: %v — re-run `agit checkout <branch>`", err)
	}
	if !ok {
		return doctorWarn, "no restored checkout context — run `agit checkout <branch>` after `agit fetch` so agents can read it via get_context"
	}
	var ctx RestoredCheckoutContext
	if err := json.Unmarshal(raw, &ctx); err != nil {
		return doctorWarn, "checkout context is not valid JSON — re-run `agit checkout <branch>`"
	}
	return doctorPass, ""
}
