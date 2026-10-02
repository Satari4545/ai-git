package agit

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// SessionInfo describes one agent session found in a bridge's on-disk
// session store.
type SessionInfo struct {
	ID         string
	LastActive time.Time
	Preview    string
	RepoPath   string
}

// sessionLister is implemented by bridges whose on-disk session store can
// be read reliably. It stays optional so bridges without a dependable
// store (or without session-targeted feeds, like claude-code) don't carry
// dead implementations.
type sessionLister interface {
	ListSessions() ([]SessionInfo, error)
}

// errSessionListUnsupported is returned when a bridge has no readable
// session store. Callers treat it as "fall back to the bridge's native
// latest-session behavior", never as a failure.
var errSessionListUnsupported = errors.New("session listing not supported for this bridge")

// repoSessions returns the bridge's sessions belonging to repoPath, most
// recent first. Sessions without a determinable repo path are skipped.
func repoSessions(b AgentBridge, repoPath string) ([]SessionInfo, error) {
	l, ok := b.(sessionLister)
	if !ok {
		return nil, errSessionListUnsupported
	}
	all, err := l.ListSessions()
	if err != nil {
		return nil, err
	}
	var out []SessionInfo
	for _, s := range all {
		if s.ID == "" || s.RepoPath == "" {
			continue
		}
		if !repoPathMatches(s.RepoPath, repoPath) {
			continue
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LastActive.After(out[j].LastActive) })
	return out, nil
}

// repoPathMatches reports whether a session path belongs to a repo: equal,
// or one nested inside the other (agents are often launched from a
// subdirectory, or from a parent of the repo).
func repoPathMatches(sessionPath, repoPath string) bool {
	s, r := filepath.Clean(sessionPath), filepath.Clean(repoPath)
	if s == r {
		return true
	}
	sep := string(os.PathSeparator)
	return strings.HasPrefix(s, r+sep) || strings.HasPrefix(r, s+sep)
}

// resolveFeedSession picks which session a checkout feed targets: the
// pinned id when set; otherwise the single live session for this repo
// (silent); otherwise the most recent live session, with a one-line notice
// naming it and how to pin a different one. An empty id means "fall back
// to the bridge's native latest-session flag".
func resolveFeedSession(b AgentBridge, pinnedID string) (id, notice string) {
	if pinnedID != "" {
		return pinnedID, ""
	}
	repoPath, err := gitTopLevel()
	if err != nil {
		return "", ""
	}
	sessions, err := repoSessions(b, repoPath)
	if err != nil || len(sessions) == 0 {
		return "", ""
	}
	if len(sessions) == 1 {
		return sessions[0].ID, ""
	}
	latest := sessions[0]
	notice = fmt.Sprintf("agit: note: %d %s sessions for this repo; fed the most recent (%s). Pin one with 'agit bridge %s session set <id>'.",
		len(sessions), b.DisplayName(), shortSessionID(latest.ID), b.Name())
	return latest.ID, notice
}

// listBridgeSessions implements `agit bridge <agent> session list`.
func listBridgeSessions(name, displayName string) error {
	b, err := GetBridge(name)
	if err != nil {
		return err
	}
	repoPath, err := gitTopLevel()
	if err != nil {
		return fmt.Errorf("agit bridge %s session list: not in a git repo", name)
	}
	sessions, err := repoSessions(b, repoPath)
	if errors.Is(err, errSessionListUnsupported) {
		fmt.Printf("agit: session listing is not supported for %s\n", displayName)
		return nil
	}
	if err != nil {
		return err
	}
	if len(sessions) == 0 {
		fmt.Printf("agit: no %s sessions for this repo\n", displayName)
		return nil
	}
	for _, s := range sessions {
		if s.Preview != "" {
			fmt.Printf("%s  %s  %s\n", shortSessionID(s.ID), sessionAge(s.LastActive), s.Preview)
		} else {
			fmt.Printf("%s  %s\n", shortSessionID(s.ID), sessionAge(s.LastActive))
		}
	}
	return nil
}

func shortSessionID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func sessionAge(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < 0:
		return "just now"
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

// scanJSONL decodes JSON objects line by line, calling fn for each until fn
// returns false, the byte budget is spent, or EOF. Malformed lines are
// skipped, so a corrupt tail never breaks discovery.
func scanJSONL(path string, budget int64, fn func(map[string]any) bool) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	const maxLine = 1 << 20
	r := bufio.NewReader(f)
	var used int64
	for {
		line, err := r.ReadBytes('\n')
		used += int64(len(line))
		if len(line) > 0 && len(line) <= maxLine {
			var obj map[string]any
			if json.Unmarshal(line, &obj) == nil {
				if !fn(obj) {
					return nil
				}
			}
		}
		if used > budget || err != nil {
			return nil
		}
	}
}

// singleLinePreview collapses a message to one short printable line.
func singleLinePreview(s string) string {
	return truncateLine(strings.Join(strings.Fields(s), " "), 80)
}

// walkSessionFiles walks dir for *.jsonl files, skipping missing dirs.
func walkSessionFiles(dir string, fn func(path string, d fs.DirEntry)) error {
	if dir == "" {
		return nil
	}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".jsonl") {
			return nil
		}
		fn(path, d)
		return nil
	})
	if err != nil && os.IsNotExist(err) {
		return nil
	}
	return err
}
