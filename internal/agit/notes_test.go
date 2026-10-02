package agit

import (
	"encoding/json"
	"testing"

	gitwrap "github.com/Satari4545/ai-git/internal/git"

	"github.com/Satari4545/ai-git/internal/testutil"
)

const testNoteJSON = `{
  "contextID": "ctx_sha256:noteref123",
  "logicState": {
    "schemaVersion": "agit.logic-delta.v0",
    "agentThinking": {"intentSummary": "ref round trip"},
    "codeReality": {"activeFiles": ["ref.go"]}
  },
  "agentContext": {
    "provider": "codex",
    "intentSummary": "ref round trip",
    "decisions": ["keep one ref"]
  }
}`

func TestAgitNoteRefRoundTrip(t *testing.T) {
	dir := testutil.InitRepo(t)

	head := testutil.CommitFile(t, dir, "ref.go", "package x", "note me")
	addAgitNote(t, dir, head, testNoteJSON)

	raw, err := gitwrap.OutputQuiet("notes", "--ref=agit", "show", head)
	if err != nil {
		t.Fatalf("reading note failed: %v", err)
	}
	if raw == "" {
		t.Fatal("expected note content, got empty")
	}

	summary, err := decodeTestNote(raw)
	if err != nil {
		t.Fatalf("decoding round-tripped note failed: %v", err)
	}
	if summary.ContextID != "ctx_sha256:noteref123" {
		t.Errorf("ContextID = %q", summary.ContextID)
	}
	if summary.Intent != "ref round trip" {
		t.Errorf("Intent = %q", summary.Intent)
	}
	if summary.Provider != "codex" {
		t.Errorf("Provider = %q", summary.Provider)
	}
}

func decodeTestNote(raw string) (struct {
	ContextID string
	Intent    string
	Provider  string
}, error) {
	var note struct {
		ContextID string `json:"contextID"`
		Logic     *struct {
			Thinking *struct {
				Intent string `json:"intentSummary"`
			} `json:"agentThinking"`
		} `json:"logicState"`
		Agent *struct {
			Provider string `json:"provider"`
		} `json:"agentContext"`
	}
	out := struct {
		ContextID string
		Intent    string
		Provider  string
	}{}
	if err := json.Unmarshal([]byte(raw), &note); err != nil {
		return out, err
	}
	out.ContextID = note.ContextID
	if note.Logic != nil && note.Logic.Thinking != nil {
		out.Intent = note.Logic.Thinking.Intent
	}
	if note.Agent != nil {
		out.Provider = note.Agent.Provider
	}
	return out, nil
}

func TestAgitNoteMissing(t *testing.T) {
	dir := testutil.InitRepo(t)

	head := testutil.CommitFile(t, dir, "plain.go", "package x", "no note")

	raw, err := gitwrap.OutputQuiet("notes", "--ref=agit", "show", head)
	if err == nil {
		t.Error("expected error for missing note, got nil")
	}
	if raw != "" {
		t.Errorf("expected empty output for missing note, got %q", raw)
	}
}

func TestAgitNoteRefListsObject(t *testing.T) {
	dir := testutil.InitRepo(t)

	head := testutil.CommitFile(t, dir, "listed.go", "package x", "list me")
	addAgitNote(t, dir, head, testNoteJSON)

	out, err := gitwrap.OutputQuiet("notes", "--ref=agit", "list")
	if err != nil {
		t.Fatalf("notes list failed: %v", err)
	}
	if out == "" {
		t.Error("expected notes list to contain the annotated commit")
	}
}
