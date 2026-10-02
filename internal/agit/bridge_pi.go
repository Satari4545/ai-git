package agit

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

func init() {
	RegisterBridge(piBridge{})
	registerBridgeExecutable("pi", piExecutable)
}

// piBridge is the AgentBridge implementation for the Pi coding agent
// (github.com/earendil-works/pi).
//
// Checkout handoff runs `pi --session <id> -p <prompt>`: it resumes the
// session, appends the prompt as a turn, runs one model turn, persists it,
// and exits — the Pi analog of `codex exec resume <id> <prompt>`. The
// RPC mode was rejected for Feed: it requires the caller to own the
// long-lived RPC process, and Pi exposes no way to attach it to a
// session already running in the TUI.
type piBridge struct{}

var _ AgentBridge = piBridge{}

func (piBridge) Name() string        { return "pi" }
func (piBridge) DisplayName() string { return "Pi" }

func (piBridge) Install() error {
	return installBridgeProvider(piBridge{})
}

func (piBridge) Provide() ([]byte, error) {
	out, err := runBridgeProvide(piBridge{},
		piPromptArgs(piContextPrompt()), []string{"PI_SKIP_VERSION_CHECK=1"})
	if err != nil {
		return nil, err
	}
	return extractJSONObject(out)
}

func (piBridge) Feed(ctx *RestoredCheckoutContext) error {
	prompt, err := feedPromptContext(ctx)
	if err != nil {
		return err
	}

	args := []string{}
	sessionID, notice := resolveFeedSession(piBridge{}, piSessionID())
	if sessionID != "" {
		args = append(args, "--session", sessionID)
	} else {
		args = append(args, "--continue")
	}
	args = append(args, "-p", prompt)

	if err := runBridgeFeed(piBridge{}, args, []string{"PI_SKIP_VERSION_CHECK=1"}); err != nil {
		return err
	}
	if notice != "" {
		fmt.Println(notice)
	}
	return nil
}

// piPromptArgs builds the session-targeting prefix shared by Provide and
// Feed. With a pinned session id it is `--session <id>`; otherwise Pi's
// `--continue` resumes the most recent session.
func piPromptArgs(prompt string) []string {
	args := []string{}
	if sessionID := piSessionID(); sessionID != "" {
		args = append(args, "--session", sessionID)
	} else {
		args = append(args, "--continue")
	}
	return append(args, "-p", prompt)
}

func piExecutable() string {
	return bridgeExecutableName("pi", ".cmd")
}

func piSessionID() string {
	return bridgeSessionID("AGIT_PI_SESSION_ID", "agit.piSession")
}

// piSessionsDir is Pi's on-disk session store: $PI_CODING_AGENT_SESSION_DIR
// when set, else <agent dir>/sessions, honoring $PI_CODING_AGENT_DIR. Files
// are <timestamp>_<uuid>.jsonl under --<cwd-slug>-- directories.
func piSessionsDir() string {
	if dir := os.Getenv("PI_CODING_AGENT_SESSION_DIR"); dir != "" {
		return dir
	}
	agentDir, err := piAgentDir()
	if err != nil {
		return ""
	}
	return filepath.Join(agentDir, "sessions")
}

// ListSessions enumerates Pi sessions from the JSONL store. The session id
// and cwd come from the {"type":"session"} header record; recency from
// file mtime; the preview from the first user message. Unparseable files
// are skipped, never fatal.
func (piBridge) ListSessions() ([]SessionInfo, error) {
	var out []SessionInfo
	err := walkSessionFiles(piSessionsDir(), func(path string, d fs.DirEntry) {
		if s, ok := readPiSession(path, d); ok {
			out = append(out, s)
		}
	})
	return out, err
}

func readPiSession(path string, d fs.DirEntry) (SessionInfo, bool) {
	info, err := d.Info()
	if err != nil {
		return SessionInfo{}, false
	}
	var id, cwd, preview string
	_ = scanJSONL(path, 128*1024, func(obj map[string]any) bool {
		if id == "" && obj["type"] == "session" {
			id, _ = obj["id"].(string)
			cwd, _ = obj["cwd"].(string)
		}
		if preview == "" {
			if p, ok := piUserPreview(obj); ok {
				preview = p
			}
		}
		return id == "" || cwd == "" || preview == ""
	})
	if id == "" || cwd == "" {
		return SessionInfo{}, false
	}
	return SessionInfo{ID: id, LastActive: info.ModTime(), Preview: singleLinePreview(preview), RepoPath: cwd}, true
}

// piUserPreview extracts preview text from a user message record,
// tolerating the content shapes Pi has used (plain string or text parts).
func piUserPreview(obj map[string]any) (string, bool) {
	if role, _ := obj["role"].(string); role != "user" {
		return "", false
	}
	if s, ok := obj["content"].(string); ok && s != "" {
		return s, true
	}
	if parts, ok := obj["content"].([]any); ok {
		for _, p := range parts {
			if pm, ok := p.(map[string]any); ok {
				if t, _ := pm["text"].(string); t != "" {
					return t, true
				}
			}
		}
	}
	return "", false
}

func piContextPrompt() string {
	return bridgeContextPrompt("pi", "Use the current Pi session and current repository state.")
}
