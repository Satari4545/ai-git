package agit

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/Satari4545/ai-git/internal/testutil"
)

func historyNoteJSON(t *testing.T, contextID, intent string, files, decisions, nextSteps []string) string {
	t.Helper()
	note := map[string]any{
		"contextID": contextID,
		"agentContext": map[string]any{
			"provider":      "codex",
			"intentSummary": intent,
			"decisions":     decisions,
			"risks":         []string{"token expiry"},
			"nextSteps":     nextSteps,
			"activeFiles":   files,
			"createdAt":     "2026-10-01T00:00:00Z",
		},
	}
	raw, err := json.Marshal(note)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func commitNoted(t *testing.T, dir, msg, contextID, intent string, files, decisions, nextSteps []string) {
	t.Helper()
	commitEmpty(t, dir, msg)
	addAgitNote(t, dir, "HEAD", historyNoteJSON(t, contextID, intent, files, decisions, nextSteps))
}

func commitEmpty(t *testing.T, dir, msg string) {
	t.Helper()
	testutil.RunGit(t, dir, "commit", "--allow-empty", "-q", "-m", msg)
}

func TestNormalizeHistoryCount(t *testing.T) {
	if _, err := normalizeHistoryCount(0); err == nil {
		t.Error("n=0: want error, got nil")
	}
	if _, err := normalizeHistoryCount(-3); err == nil {
		t.Error("n=-3: want error, got nil")
	}
	for _, n := range []int{1, 10, 50} {
		if got, err := normalizeHistoryCount(n); err != nil || got != n {
			t.Errorf("n=%d: got %d, err %v; want %d, nil", n, got, err, n)
		}
	}
	for _, n := range []int{51, 1000} {
		if got, err := normalizeHistoryCount(n); err != nil || got != maxHistoryCount {
			t.Errorf("n=%d: got %d, err %v; want cap %d", n, got, err, maxHistoryCount)
		}
	}
}

func TestGetHistoryReturnsNotesNewestFirst(t *testing.T) {
	dir := testutil.InitRepo(t)

	commitNoted(t, dir, "first", "ctx_sha256:111", "set up project", []string{"main.go"}, []string{"use stdlib"}, []string{"add tests"})
	commitNoted(t, dir, "second", "ctx_sha256:222", "add login", []string{"web/login.go"}, []string{"use JWT"}, []string{"write tests"})
	commitNoted(t, dir, "third", "ctx_sha256:333", "add logout", []string{"web/logout.go"}, []string{"clear session"}, []string{"ship it"})

	entries, err := GetHistory(10, "", "")
	if err != nil {
		t.Fatalf("GetHistory = %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("len(entries) = %d, want 3", len(entries))
	}

	wantIDs := []string{"ctx_sha256:333", "ctx_sha256:222", "ctx_sha256:111"}
	for i, want := range wantIDs {
		if entries[i].ContextID != want {
			t.Errorf("entries[%d].ContextID = %q, want %q", i, entries[i].ContextID, want)
		}
	}

	head := testutil.RunGit(t, dir, "rev-parse", "HEAD")
	head1 := testutil.RunGit(t, dir, "rev-parse", "HEAD~1")
	head2 := testutil.RunGit(t, dir, "rev-parse", "HEAD~2")
	if entries[0].Commit != strings.TrimSpace(head) ||
		entries[1].Commit != strings.TrimSpace(head1) ||
		entries[2].Commit != strings.TrimSpace(head2) {
		t.Errorf("commit SHAs out of order: %q %q %q", entries[0].Commit, entries[1].Commit, entries[2].Commit)
	}

	e := entries[0]
	if e.Intent != "add logout" {
		t.Errorf("Intent = %q, want %q", e.Intent, "add logout")
	}
	if len(e.Files) != 1 || e.Files[0] != "web/logout.go" {
		t.Errorf("Files = %v, want [web/logout.go]", e.Files)
	}
	if len(e.Decisions) != 1 || e.Decisions[0] != "clear session" {
		t.Errorf("Decisions = %v, want [clear session]", e.Decisions)
	}
	if len(e.NextSteps) != 1 || e.NextSteps[0] != "ship it" {
		t.Errorf("NextSteps = %v, want [ship it]", e.NextSteps)
	}
	if e.Subject != "third" || e.Date == "" || e.Short == "" {
		t.Errorf("entry metadata incomplete: %+v", e)
	}
}

func TestGetHistorySkipsCommitsWithoutNotes(t *testing.T) {
	dir := testutil.InitRepo(t)

	commitNoted(t, dir, "with note 1", "ctx_sha256:aaa", "first", nil, nil, nil)
	commitEmpty(t, dir, "no note here")
	commitNoted(t, dir, "with note 2", "ctx_sha256:bbb", "second", nil, nil, nil)

	entries, err := GetHistory(10, "", "")
	if err != nil {
		t.Fatalf("GetHistory = %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2 (note-less commit skipped)", len(entries))
	}
	if entries[0].ContextID != "ctx_sha256:bbb" || entries[1].ContextID != "ctx_sha256:aaa" {
		t.Errorf("unexpected entries: %q, %q", entries[0].ContextID, entries[1].ContextID)
	}
}

func TestGetHistoryEmptyWhenNoNotes(t *testing.T) {
	dir := testutil.InitRepo(t)

	commitEmpty(t, dir, "one")
	commitEmpty(t, dir, "two")

	entries, err := GetHistory(10, "", "")
	if err != nil {
		t.Fatalf("GetHistory = %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("len(entries) = %d, want 0", len(entries))
	}
}

func TestGetHistoryInvalidN(t *testing.T) {
	dir := testutil.InitRepo(t)
	commitEmpty(t, dir, "one")

	for _, n := range []int{0, -1, -100} {
		if _, err := GetHistory(n, "", ""); err == nil {
			t.Errorf("GetHistory(%d): want error, got nil", n)
		}
	}
}

func TestGetHistoryCapsLargeN(t *testing.T) {
	dir := testutil.InitRepo(t)

	commitNoted(t, dir, "one", "ctx_sha256:cap1", "only", nil, nil, nil)

	entries, err := GetHistory(1000, "", "")
	if err != nil {
		t.Fatalf("GetHistory(1000) = %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("len(entries) = %d, want 1", len(entries))
	}
}

func TestGetHistoryBranchFilter(t *testing.T) {
	dir := testutil.InitRepo(t)

	commitEmpty(t, dir, "base")
	testutil.RunGit(t, dir, "checkout", "-q", "-b", "feature")
	commitNoted(t, dir, "feature work", "ctx_sha256:feat", "feature thing", nil, nil, nil)
	testutil.RunGit(t, dir, "checkout", "-q", "-")

	entries, err := GetHistory(10, "", "")
	if err != nil {
		t.Fatalf("GetHistory default branch = %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("default branch entries = %d, want 0", len(entries))
	}

	entries, err = GetHistory(10, "feature", "")
	if err != nil {
		t.Fatalf("GetHistory(feature) = %v", err)
	}
	if len(entries) != 1 || entries[0].ContextID != "ctx_sha256:feat" {
		t.Errorf("feature branch history = %+v, want the feature note", entries)
	}
}

func TestGetHistoryUnknownBranch(t *testing.T) {
	dir := testutil.InitRepo(t)
	commitEmpty(t, dir, "one")

	if _, err := GetHistory(10, "no-such-branch", ""); err == nil {
		t.Error("unknown branch: want error, got nil")
	}
}

func TestGetHistorySinceFilter(t *testing.T) {
	dir := testutil.InitRepo(t)

	cmd := exec.Command("git", "commit", "--allow-empty", "-q", "-m", "old work")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_DATE=2020-06-01T12:00:00",
		"GIT_COMMITTER_DATE=2020-06-01T12:00:00",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit with backdated env failed: %v\n%s", err, out)
	}
	addAgitNote(t, dir, "HEAD", historyNoteJSON(t, "ctx_sha256:since1", "old work", nil, nil, nil))

	entries, err := GetHistory(10, "", "2021-01-01")
	if err != nil {
		t.Fatalf("GetHistory since=2021-01-01 = %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("since=2021-01-01 entries = %d, want 0", len(entries))
	}

	entries, err = GetHistory(10, "", "2019-01-01")
	if err != nil {
		t.Fatalf("GetHistory since=2019-01-01 = %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("since=2019-01-01 entries = %d, want 1", len(entries))
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	fn()

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdout = old
	out := make([]byte, 0, 4096)
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		out = append(out, buf[:n]...)
		if err != nil {
			break
		}
	}
	return string(out)
}

func TestMCPGetHistoryDispatch(t *testing.T) {
	dir := testutil.InitRepo(t)

	commitNoted(t, dir, "one", "ctx_sha256:mcp1", "mcp work", []string{"a.go"}, []string{"d1"}, []string{"n1"})
	commitNoted(t, dir, "two", "ctx_sha256:mcp2", "more mcp work", nil, nil, nil)

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"get_history","arguments":{"n":5}}`),
	}

	out := captureStdout(t, func() { handleToolCall(req) })

	var resp MCPResponse
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &resp); err != nil {
		t.Fatalf("response is not JSON: %v\n%s", err, out)
	}
	if resp.Error != nil {
		t.Fatalf("response error: %+v", resp.Error)
	}
	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("result is %T, want object", resp.Result)
	}
	content, ok := result["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("result.content missing: %v", result)
	}
	text, _ := content[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, "ctx_sha256:mcp2") || !strings.Contains(text, "ctx_sha256:mcp1") {
		t.Errorf("response text missing context IDs:\n%s", text)
	}
	if !strings.Contains(text, "mcp work") {
		t.Errorf("response text missing intent:\n%s", text)
	}
	if strings.Index(text, "ctx_sha256:mcp2") > strings.Index(text, "ctx_sha256:mcp1") {
		t.Errorf("entries not newest-first:\n%s", text)
	}
}

func TestMCPGetHistoryDispatchInvalidN(t *testing.T) {
	_ = testutil.InitRepo(t)

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"get_history","arguments":{"n":0}}`),
	}

	out := captureStdout(t, func() { handleToolCall(req) })

	var resp MCPResponse
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &resp); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if resp.Error == nil {
		t.Errorf("n=0: want error response, got %+v", resp.Result)
	}
}

func TestMCPToolsListIncludesGetHistory(t *testing.T) {
	_ = testutil.InitRepo(t)

	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldStdin := os.Stdin
	os.Stdin = stdinR
	defer func() { os.Stdin = oldStdin }()

	if _, err := stdinW.WriteString("{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/list\"}\n"); err != nil {
		t.Fatal(err)
	}
	stdinW.Close()

	out := captureStdout(t, func() {
		if err := MCPServe(); err != nil {
			t.Errorf("MCPServe = %v", err)
		}
	})

	if !strings.Contains(out, `"name":"get_history"`) {
		t.Errorf("tools/list response missing get_history:\n%s", out)
	}
	if !strings.Contains(out, `"name":"get_context"`) {
		t.Errorf("tools/list response missing existing get_context (regression):\n%s", out)
	}
}

func TestGetHistoryOutsideRepo(t *testing.T) {
	dir := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(oldwd); err != nil {
			t.Fatal(err)
		}
	}()
	if _, err := GetHistory(10, "", ""); err == nil || !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("GetHistory outside repo = %v, want 'not a git repository' error", err)
	}
}
