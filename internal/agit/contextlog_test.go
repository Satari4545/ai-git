package agit

import (
	"strings"
	"testing"
)

const testNoteFull = `{
  "contextID": "ctx_sha256:abc123def456",
  "logicState": {
    "schemaVersion": "agit.logic-delta.v0",
    "kind": "logic_delta",
    "agentThinking": {
      "intentSummary": "Add context log command",
      "riskFlags": ["large note output"]
    },
    "codeReality": {
      "activeFiles": ["internal/agit/contextlog.go", "cmd/agit/main.go"]
    },
    "createdAt": "2026-09-24T20:00:00Z"
  },
  "agentContext": {
    "provider": "codex",
    "intentSummary": "Agent-level intent",
    "decisions": ["use context subcommand"],
    "risks": ["scope creep"],
    "nextSteps": ["write tests"],
    "activeFiles": ["should/not/appear.go"],
    "createdAt": "2026-09-24T19:00:00Z"
  }
}`

func TestSummarizeNoteFull(t *testing.T) {
	s, err := summarizeNote([]byte(testNoteFull))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if s.ContextID != "ctx_sha256:abc123def456" {
		t.Errorf("ContextID = %q", s.ContextID)
	}
	if s.Intent != "Add context log command" {
		t.Errorf("Intent = %q", s.Intent)
	}
	if s.Provider != "codex" {
		t.Errorf("Provider = %q", s.Provider)
	}
	if len(s.Files) != 2 || s.Files[0] != "internal/agit/contextlog.go" {
		t.Errorf("Files = %v", s.Files)
	}
	if len(s.Decisions) != 1 || s.Decisions[0] != "use context subcommand" {
		t.Errorf("Decisions = %v", s.Decisions)
	}
	if len(s.Risks) != 2 {
		t.Errorf("Risks = %v, want 2 merged entries", s.Risks)
	}
	if len(s.NextSteps) != 1 || s.NextSteps[0] != "write tests" {
		t.Errorf("NextSteps = %v", s.NextSteps)
	}
	if s.CreatedAt != "2026-09-24T20:00:00Z" {
		t.Errorf("CreatedAt = %q", s.CreatedAt)
	}
}

func TestSummarizeNoteLogicStateOnly(t *testing.T) {
	raw := `{"contextID":"ctx_sha256:x","logicState":{"agentThinking":{"intentSummary":"solo"},"codeReality":{"activeFiles":["a.go"]}}}`
	s, err := summarizeNote([]byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Intent != "solo" || s.Provider != "" || len(s.Files) != 1 {
		t.Errorf("unexpected summary: %+v", s)
	}
}

func TestSummarizeNoteIgnoresUnknownFields(t *testing.T) {
	raw := `{"contextID":"ctx_sha256:x","futureField":{"nested":[1,2,3]},"logicState":{"agentThinking":{"intentSummary":"ok","brandNew":["a"]}}}`
	s, err := summarizeNote([]byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Intent != "ok" {
		t.Errorf("Intent = %q", s.Intent)
	}
}

func TestSummarizeNoteMalformed(t *testing.T) {
	for _, raw := range []string{"", "{not json", "[1,2", "null"} {
		if _, err := summarizeNote([]byte(raw)); err == nil {
			t.Errorf("expected error for %q, got nil", raw)
		}
	}
}

func TestSummarizeNoteEmptyObject(t *testing.T) {
	s, err := summarizeNote([]byte(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.ContextID != "" || s.Intent != "" {
		t.Errorf("expected empty summary, got %+v", s)
	}
}

func TestParseContextLogArgsDefaults(t *testing.T) {
	opts, err := parseContextLogArgs(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.maxCount != defaultContextLogCount || opts.asJSON || len(opts.paths) != 0 {
		t.Errorf("unexpected defaults: %+v", opts)
	}
}

func TestParseContextLogArgsFlags(t *testing.T) {
	opts, err := parseContextLogArgs([]string{"-n", "5", "--json", "--", "foo.go", "bar/"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.maxCount != 5 || !opts.asJSON {
		t.Errorf("unexpected opts: %+v", opts)
	}
	if len(opts.paths) != 2 || opts.paths[0] != "foo.go" || opts.paths[1] != "bar/" {
		t.Errorf("unexpected paths: %v", opts.paths)
	}
}

func TestParseContextLogArgsLongForm(t *testing.T) {
	opts, err := parseContextLogArgs([]string{"--max-count=7", "main.go"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.maxCount != 7 || len(opts.paths) != 1 || opts.paths[0] != "main.go" {
		t.Errorf("unexpected opts: %+v", opts)
	}
}

func TestParseContextLogArgsInvalid(t *testing.T) {
	for _, args := range [][]string{
		{"-n"},
		{"-n", "0"},
		{"-n", "-3"},
		{"-n", "abc"},
		{"--max-count=0"},
		{"--bogus"},
	} {
		if _, err := parseContextLogArgs(args); err == nil {
			t.Errorf("expected error for %v, got nil", args)
		}
	}
}

func TestShortContextID(t *testing.T) {
	long := "ctx_sha256:abcdef1234567890"
	short := shortContextID(long)
	if !strings.HasPrefix(short, "ctx_sha256:abcdef123456") || !strings.HasSuffix(short, "...") {
		t.Errorf("unexpected short ID: %q", short)
	}
	if got := shortContextID("plain"); got != "plain" {
		t.Errorf("unexpected short ID for plain input: %q", got)
	}
}

func TestTruncateLineMultibyte(t *testing.T) {
	s := strings.Repeat("é", 20)
	got := truncateLine(s, 10)
	want := strings.Repeat("é", 7) + "..."
	if got != want {
		t.Errorf("truncateLine multibyte = %q, want %q", got, want)
	}
	if short := truncateLine("hi", 10); short != "hi" {
		t.Errorf("truncateLine short = %q, want unchanged", short)
	}
}
