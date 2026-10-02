package agit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Satari4545/ai-git/internal/testutil"
)

func writeCodexSessionFile(t *testing.T, home, id, cwd, userMsg string, modTime time.Time) {
	t.Helper()
	dir := filepath.Join(home, ".codex", "sessions", "2026", "10", "02")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	lines := []map[string]any{
		{"timestamp": "2026-10-02T00:00:00.000Z", "type": "session_meta",
			"payload": map[string]any{"id": id, "session_id": id, "cwd": cwd}},
	}
	if userMsg != "" {
		lines = append(lines, map[string]any{
			"timestamp": "2026-10-02T00:00:01.000Z", "type": "event_msg",
			"payload": map[string]any{"type": "user_message", "message": userMsg},
		})
	}
	var sb strings.Builder
	for _, l := range lines {
		raw, err := json.Marshal(l)
		if err != nil {
			t.Fatal(err)
		}
		sb.Write(raw)
		sb.WriteByte('\n')
	}
	path := filepath.Join(dir, "rollout-2026-10-02T00-00-00-"+id+".jsonl")
	if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatal(err)
	}
}

func writePiSessionFile(t *testing.T, agentDir, slug, id, cwd, userMsg string, modTime time.Time) {
	t.Helper()
	dir := filepath.Join(agentDir, "sessions", "--"+slug+"--")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	lines := []map[string]any{
		{"type": "session", "version": 3, "id": id, "timestamp": "2026-10-02T00:00:00.000Z", "cwd": cwd},
	}
	if userMsg != "" {
		lines = append(lines, map[string]any{
			"type": "message", "role": "user",
			"content": []any{map[string]any{"type": "text", "text": userMsg}},
		})
	}
	var sb strings.Builder
	for _, l := range lines {
		raw, err := json.Marshal(l)
		if err != nil {
			t.Fatal(err)
		}
		sb.Write(raw)
		sb.WriteByte('\n')
	}
	path := filepath.Join(dir, "2026-10-02T00-00-00_"+id+".jsonl")
	if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatal(err)
	}
}

func isolateSessionEnv(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", "")
	t.Setenv("PI_CODING_AGENT_DIR", "")
	t.Setenv("PI_CODING_AGENT_SESSION_DIR", "")
	return home
}

func TestListCodexSessionsFiltersByRepo(t *testing.T) {
	dir := testutil.InitRepo(t)
	home := isolateSessionEnv(t)
	now := time.Now()
	writeCodexSessionFile(t, home, "id-old-1111", dir, "old work", now.Add(-2*time.Hour))
	writeCodexSessionFile(t, home, "id-new-2222", dir, "new work", now.Add(-time.Hour))
	writeCodexSessionFile(t, home, "id-other-3333", "/some/other/repo", "other", now)

	b, err := GetBridge("codex")
	if err != nil {
		t.Fatal(err)
	}
	sessions, err := repoSessions(b, dir)
	if err != nil {
		t.Fatalf("repoSessions failed: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("got %d sessions, want 2", len(sessions))
	}
	if sessions[0].ID != "id-new-2222" || sessions[1].ID != "id-old-1111" {
		t.Errorf("wrong order or ids: %+v", sessions)
	}
	if sessions[0].Preview != "new work" || sessions[0].RepoPath != dir {
		t.Errorf("wrong preview/repo: %+v", sessions[0])
	}
}

func TestListCodexSessionsNoStore(t *testing.T) {
	testutil.InitRepo(t)
	isolateSessionEnv(t)
	b, _ := GetBridge("codex")
	sessions, err := repoSessions(b, "/repo")
	if err != nil {
		t.Fatalf("repoSessions failed: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("got %d sessions, want 0", len(sessions))
	}
}

func TestListCodexSessionsSkipsCorrupt(t *testing.T) {
	dir := testutil.InitRepo(t)
	home := isolateSessionEnv(t)
	now := time.Now()
	writeCodexSessionFile(t, home, "id-good-1111", dir, "good", now)
	badDir := filepath.Join(home, ".codex", "sessions", "2026", "10", "02")
	if err := os.WriteFile(filepath.Join(badDir, "rollout-bad.jsonl"), []byte("{not json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b, _ := GetBridge("codex")
	sessions, err := repoSessions(b, dir)
	if err != nil {
		t.Fatalf("repoSessions failed: %v", err)
	}
	if len(sessions) != 1 || sessions[0].ID != "id-good-1111" {
		t.Errorf("got %+v, want the one good session", sessions)
	}
}

func TestListPiSessions(t *testing.T) {
	dir := testutil.InitRepo(t)
	home := isolateSessionEnv(t)
	agentDir := filepath.Join(home, ".pi", "agent")
	t.Setenv("PI_CODING_AGENT_DIR", agentDir)
	now := time.Now()
	writePiSessionFile(t, agentDir, "tmp-repo", "pi-id-1111", dir, "pi work", now)
	writePiSessionFile(t, agentDir, "other", "pi-id-2222", "/elsewhere", "other", now)

	b, err := GetBridge("pi")
	if err != nil {
		t.Fatal(err)
	}
	sessions, err := repoSessions(b, dir)
	if err != nil {
		t.Fatalf("repoSessions failed: %v", err)
	}
	if len(sessions) != 1 || sessions[0].ID != "pi-id-1111" {
		t.Fatalf("got %+v, want the one pi session", sessions)
	}
	if sessions[0].Preview != "pi work" {
		t.Errorf("preview = %q, want pi work", sessions[0].Preview)
	}
}

func TestBridgeSessionListVerb(t *testing.T) {
	dir := testutil.InitRepo(t)
	home := isolateSessionEnv(t)
	now := time.Now()
	writeCodexSessionFile(t, home, "id-aaa-1111", dir, "first task", now.Add(-time.Hour))
	writeCodexSessionFile(t, home, "id-bbb-2222", dir, "second task", now)

	out := captureStdout(t, func() {
		if err := Bridge([]string{"codex", "session", "list"}); err != nil {
			t.Errorf("session list failed: %v", err)
		}
	})
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2:\n%s", len(lines), out)
	}
	if !strings.Contains(lines[0], "id-bbb-2") || !strings.Contains(lines[0], "second task") {
		t.Errorf("first row should be newest session:\n%s", out)
	}
	if !strings.Contains(lines[1], "id-aaa-1") {
		t.Errorf("second row should be older session:\n%s", out)
	}
}

func TestBridgeSessionListNoSessions(t *testing.T) {
	testutil.InitRepo(t)
	isolateSessionEnv(t)
	out := captureStdout(t, func() {
		if err := Bridge([]string{"codex", "session", "list"}); err != nil {
			t.Errorf("session list failed: %v", err)
		}
	})
	if !strings.Contains(out, "no Codex sessions for this repo") {
		t.Errorf("want clean empty message, got:\n%s", out)
	}
}

func TestBridgeSessionListUnsupported(t *testing.T) {
	testutil.InitRepo(t)
	isolateSessionEnv(t)
	out := captureStdout(t, func() {
		if err := Bridge([]string{"gemini", "session", "list"}); err != nil {
			t.Errorf("session list failed: %v", err)
		}
	})
	if !strings.Contains(out, "not supported for Gemini CLI") {
		t.Errorf("want unsupported message, got:\n%s", out)
	}
}

func TestResolveFeedSessionSingle(t *testing.T) {
	dir := testutil.InitRepo(t)
	home := isolateSessionEnv(t)
	writeCodexSessionFile(t, home, "only-session-id", dir, "work", time.Now())

	b, _ := GetBridge("codex")
	id, notice := resolveFeedSession(b, "")
	if id != "only-session-id" {
		t.Errorf("id = %q, want only-session-id", id)
	}
	if notice != "" {
		t.Errorf("notice = %q, want empty for single session", notice)
	}
}

func TestResolveFeedSessionMultiple(t *testing.T) {
	dir := testutil.InitRepo(t)
	home := isolateSessionEnv(t)
	now := time.Now()
	writeCodexSessionFile(t, home, "older-session-id", dir, "old", now.Add(-time.Hour))
	writeCodexSessionFile(t, home, "newer-session-id", dir, "new", now)

	b, _ := GetBridge("codex")
	id, notice := resolveFeedSession(b, "")
	if id != "newer-session-id" {
		t.Errorf("id = %q, want newer-session-id", id)
	}
	if !strings.Contains(notice, "newer-se") || !strings.Contains(notice, "agit bridge codex session set <id>") {
		t.Errorf("notice missing short id or pin hint: %q", notice)
	}
}

func TestResolveFeedSessionPinnedWins(t *testing.T) {
	dir := testutil.InitRepo(t)
	home := isolateSessionEnv(t)
	writeCodexSessionFile(t, home, "live-session-id", dir, "work", time.Now())

	b, _ := GetBridge("codex")
	id, notice := resolveFeedSession(b, "pinned-id")
	if id != "pinned-id" || notice != "" {
		t.Errorf("got (%q, %q), want pinned-id with no notice", id, notice)
	}
}

func TestResolveFeedSessionNone(t *testing.T) {
	testutil.InitRepo(t)
	isolateSessionEnv(t)
	b, _ := GetBridge("codex")
	id, notice := resolveFeedSession(b, "")
	if id != "" || notice != "" {
		t.Errorf("got (%q, %q), want empty", id, notice)
	}
}

func TestCodexFeedSmartSingleSession(t *testing.T) {
	dir := testutil.InitRepo(t)
	home := isolateSessionEnv(t)
	stubDir := writeCodexStub(t, "")
	writeCodexSessionFile(t, home, "smart-session-id", dir, "work", time.Now())

	head := testutil.CommitFile(t, dir, "feat.go", "package x", "feature work")
	addAgitNote(t, dir, head, testCheckoutNote)
	if _, err := RestoreHEADContext(); err != nil {
		t.Fatalf("RestoreHEADContext failed: %v", err)
	}
	b, _ := GetBridge("codex")
	ctx, _ := LoadCheckoutContext()
	if err := b.Feed(ctx); err != nil {
		t.Fatalf("Feed failed: %v", err)
	}
	args := readCodexStubArgs(t, stubDir)
	if !strings.Contains(args, "smart-session-id") {
		t.Errorf("feed should target the single live session:\n%s", args)
	}
	if strings.Contains(args, "--last") {
		t.Errorf("feed should not use --last with a known session:\n%s", args)
	}
}

func TestCodexFeedSmartMultipleSessions(t *testing.T) {
	dir := testutil.InitRepo(t)
	home := isolateSessionEnv(t)
	stubDir := writeCodexStub(t, "")
	now := time.Now()
	writeCodexSessionFile(t, home, "older-session-id", dir, "old", now.Add(-time.Hour))
	writeCodexSessionFile(t, home, "newer-session-id", dir, "new", now)

	head := testutil.CommitFile(t, dir, "feat.go", "package x", "feature work")
	addAgitNote(t, dir, head, testCheckoutNote)
	if _, err := RestoreHEADContext(); err != nil {
		t.Fatalf("RestoreHEADContext failed: %v", err)
	}
	b, _ := GetBridge("codex")
	ctx, _ := LoadCheckoutContext()
	out := captureStdout(t, func() {
		if err := b.Feed(ctx); err != nil {
			t.Errorf("Feed failed: %v", err)
		}
	})
	args := readCodexStubArgs(t, stubDir)
	if !strings.Contains(args, "newer-session-id") {
		t.Errorf("feed should target the most recent session:\n%s", args)
	}
	if !strings.Contains(out, "newer-se") || !strings.Contains(out, "session set <id>") {
		t.Errorf("notice missing short id or pin hint:\n%s", out)
	}
}
