package agit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func init() {
	RegisterBridge(codexBridge{})
	registerBridgeExecutable("codex", codexExecutable)
}

// codexBridge is the AgentBridge implementation for Codex.
type codexBridge struct{}

var _ AgentBridge = codexBridge{}

func (codexBridge) Name() string        { return "codex" }
func (codexBridge) DisplayName() string { return "Codex" }

func (codexBridge) Install() error {
	return installBridgeProvider(codexBridge{})
}

// Provide asks the Codex session for fresh context JSON. The response is
// parsed and normalized by the generic provider verb.
func (codexBridge) Provide() ([]byte, error) {
	return askCodexForContext()
}

// Feed delivers restored checkout context to the Codex session: the pinned
// session when set, else the single live session for this repo, else the
// most recent one (with a notice), else Codex's native --last.
func (codexBridge) Feed(ctx *RestoredCheckoutContext) error {
	prompt, err := feedPromptContext(ctx)
	if err != nil {
		return err
	}

	cmdArgs := []string{"exec", "resume"}
	sessionID, notice := resolveFeedSession(codexBridge{}, codexSessionID())
	if sessionID != "" {
		cmdArgs = append(cmdArgs, sessionID)
	} else {
		cmdArgs = append(cmdArgs, "--last")
	}
	cmdArgs = append(cmdArgs, prompt)

	if err := runBridgeFeed(codexBridge{}, cmdArgs, []string{"AGIT_CODEX_CONTEXT_FEED=1"}); err != nil {
		return err
	}
	if notice != "" {
		fmt.Println(notice)
	}
	return nil
}

// askCodexForContext runs the provider prompt through Codex. Codex streams
// session chatter to stdout, so the model's answer is captured via
// --output-last-message into a temp file instead of the process output.
func askCodexForContext() ([]byte, error) {
	tmp, err := os.CreateTemp("", "agit-codex-context-*.json")
	if err != nil {
		return nil, err
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(tmpPath)

	cmdArgs := []string{"exec", "resume"}
	if sessionID := codexSessionID(); sessionID != "" {
		cmdArgs = append(cmdArgs, sessionID)
	} else {
		cmdArgs = append(cmdArgs, "--last")
	}
	cmdArgs = append(cmdArgs, "--output-last-message", tmpPath, codexContextPrompt())

	cmd := exec.Command(codexExecutable(), cmdArgs...)
	cmd.Env = append(os.Environ(), "AGIT_CODEX_CONTEXT_PROVIDER=1")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("Codex context bridge failed: %s", message)
	}

	raw, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, err
	}

	return extractJSONObject(raw)
}

func codexExecutable() string {
	return bridgeExecutableName("codex", ".cmd")
}

func codexContextPrompt() string {
	return bridgeContextPrompt("codex", "Use the current Codex session and current repository state.")
}

func extractJSONObject(raw []byte) ([]byte, error) {
	start := bytes.IndexByte(raw, '{')
	end := bytes.LastIndexByte(raw, '}')
	if start < 0 || end < start {
		return nil, errors.New("agent did not return a JSON object")
	}

	candidate := raw[start : end+1]
	var payload map[string]any
	if err := json.Unmarshal(candidate, &payload); err != nil {
		return nil, err
	}

	return candidate, nil
}

func gitwrapRunConfig(key string, value string) error {
	cmd := exec.Command("git", "config", key, value)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir, _ = filepath.Abs(".")
	return cmd.Run()
}

func gitwrapOutputConfig(key string) (string, error) {
	var stdout bytes.Buffer
	cmd := exec.Command("git", "config", "--get", key)
	cmd.Stdout = &stdout
	cmd.Dir, _ = filepath.Abs(".")
	err := cmd.Run()
	return strings.TrimSpace(stdout.String()), err
}

func gitwrapUnsetConfig(key string) error {
	cmd := exec.Command("git", "config", "--unset", key)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir, _ = filepath.Abs(".")
	return cmd.Run()
}

func codexSessionID() string {
	return bridgeSessionID("AGIT_CODEX_SESSION_ID", "agit.codexSession")
}

// codexSessionsDir is the on-disk rollout store: $CODEX_HOME/sessions when
// set, else ~/.codex/sessions. Files are rollout-*.jsonl, optionally nested
// under YYYY/MM/DD.
func codexSessionsDir() string {
	if dir := os.Getenv("CODEX_HOME"); dir != "" {
		return filepath.Join(dir, "sessions")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".codex", "sessions")
}

// ListSessions enumerates Codex sessions from the rollout store. The
// session id and cwd come from the session_meta record; recency from file
// mtime; the preview from the first user message. Unparseable files are
// skipped, never fatal.
func (codexBridge) ListSessions() ([]SessionInfo, error) {
	var out []SessionInfo
	err := walkSessionFiles(codexSessionsDir(), func(path string, d fs.DirEntry) {
		if !strings.HasPrefix(d.Name(), "rollout-") {
			return
		}
		if s, ok := readCodexSession(path, d); ok {
			out = append(out, s)
		}
	})
	return out, err
}

func readCodexSession(path string, d fs.DirEntry) (SessionInfo, bool) {
	info, err := d.Info()
	if err != nil {
		return SessionInfo{}, false
	}
	var id, cwd, preview string
	_ = scanJSONL(path, 128*1024, func(obj map[string]any) bool {
		objType, _ := obj["type"].(string)
		payload, _ := obj["payload"].(map[string]any)
		switch objType {
		case "session_meta":
			if id == "" {
				id, _ = payload["id"].(string)
				if id == "" {
					id, _ = payload["session_id"].(string)
				}
			}
			if cwd == "" {
				cwd, _ = payload["cwd"].(string)
			}
		case "event_msg":
			if preview == "" && payload["type"] == "user_message" {
				preview, _ = payload["message"].(string)
			}
		}
		return id == "" || cwd == "" || preview == ""
	})
	if id == "" || cwd == "" {
		return SessionInfo{}, false
	}
	return SessionInfo{ID: id, LastActive: info.ModTime(), Preview: singleLinePreview(preview), RepoPath: cwd}, true
}
