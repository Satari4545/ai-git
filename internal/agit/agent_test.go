package agit

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/Satari4545/ai-git/internal/testutil"
)

func TestParseAgentContextJSONAliases(t *testing.T) {
	raw := `{
		"provider": "codex",
		"intent": "legacy intent field",
		"message": "legacy message field",
		"files": ["a.go"],
		"planned": "legacy planned",
		"actual": "legacy actual",
		"decisions": ["d1"],
		"risks": ["r1"],
		"nextSteps": ["n1"],
		"taskID": "task-1"
	}`

	ctx, err := ParseAgentContextJSON([]byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ctx.Provider != "codex" {
		t.Errorf("Provider = %q", ctx.Provider)
	}
	if ctx.IntentSummary != "legacy intent field" {
		t.Errorf("IntentSummary = %q (legacy alias not applied)", ctx.IntentSummary)
	}
	if ctx.CommitMessageSuggestion != "legacy message field" {
		t.Errorf("CommitMessageSuggestion = %q (legacy alias not applied)", ctx.CommitMessageSuggestion)
	}
	if !reflect.DeepEqual(ctx.ActiveFiles, []string{"a.go"}) {
		t.Errorf("ActiveFiles = %v (legacy alias not applied)", ctx.ActiveFiles)
	}
	if ctx.PlannedDelta != "legacy planned" || ctx.ActualDelta != "legacy actual" {
		t.Errorf("PlannedDelta = %q, ActualDelta = %q", ctx.PlannedDelta, ctx.ActualDelta)
	}
	if ctx.TaskID != "task-1" {
		t.Errorf("TaskID = %q", ctx.TaskID)
	}
	if ctx.CreatedAt == "" {
		t.Error("CreatedAt was not stamped")
	}
}

func TestParseAgentContextJSONPrefersCanonicalFields(t *testing.T) {
	raw := `{
		"intentSummary": "canonical",
		"intent": "legacy",
		"commitMessageSuggestion": "canonical msg",
		"message": "legacy msg",
		"activeFiles": ["b.go"],
		"files": ["a.go"]
	}`

	ctx, err := ParseAgentContextJSON([]byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.IntentSummary != "canonical" {
		t.Errorf("IntentSummary = %q, want canonical field to win", ctx.IntentSummary)
	}
	if ctx.CommitMessageSuggestion != "canonical msg" {
		t.Errorf("CommitMessageSuggestion = %q, want canonical field to win", ctx.CommitMessageSuggestion)
	}
	if !reflect.DeepEqual(ctx.ActiveFiles, []string{"b.go"}) {
		t.Errorf("ActiveFiles = %v, want canonical field to win", ctx.ActiveFiles)
	}
}

func TestParseAgentContextJSONMissingIntent(t *testing.T) {
	if _, err := ParseAgentContextJSON([]byte(`{"provider":"x"}`)); err == nil {
		t.Error("expected error when intent is missing, got nil")
	}
}

func TestParseAgentContextJSONInvalid(t *testing.T) {
	if _, err := ParseAgentContextJSON([]byte(`{oops`)); err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestAgentWriteRoundTrip(t *testing.T) {
	testutil.InitRepo(t)
	t.Setenv("AGIT_MCP_SILENT", "1")

	err := AgentWrite([]string{
		"--provider", "codex",
		"--intent", "add feature",
		"--files", "a.go, b.go",
		"--decisions", "d1, d2",
		"--risks", "r1",
		"--next-steps", "n1",
		"--task-id", "t-9",
	})
	if err != nil {
		t.Fatalf("AgentWrite failed: %v", err)
	}

	ctx, ok, err := LoadAgentContext()
	if err != nil {
		t.Fatalf("LoadAgentContext failed: %v", err)
	}
	if !ok {
		t.Fatal("expected context to be present after AgentWrite")
	}

	if ctx.Provider != "codex" {
		t.Errorf("Provider = %q", ctx.Provider)
	}
	if ctx.IntentSummary != "add feature" {
		t.Errorf("IntentSummary = %q", ctx.IntentSummary)
	}
	if ctx.CommitMessageSuggestion != "add feature" {
		t.Errorf("CommitMessageSuggestion = %q, want intent default", ctx.CommitMessageSuggestion)
	}
	if !reflect.DeepEqual(ctx.ActiveFiles, []string{"a.go", "b.go"}) {
		t.Errorf("ActiveFiles = %v", ctx.ActiveFiles)
	}
	if !reflect.DeepEqual(ctx.Decisions, []string{"d1", "d2"}) {
		t.Errorf("Decisions = %v", ctx.Decisions)
	}
	if ctx.SchemaVersion != "agit.agent-context.v0" {
		t.Errorf("SchemaVersion = %q", ctx.SchemaVersion)
	}
	if _, err := time.Parse(time.RFC3339, ctx.CreatedAt); err != nil {
		t.Errorf("CreatedAt %q is not RFC3339: %v", ctx.CreatedAt, err)
	}
}

func TestAgentWriteMissingIntent(t *testing.T) {
	testutil.InitRepo(t)
	t.Setenv("AGIT_MCP_SILENT", "1")

	if err := AgentWrite([]string{"--provider", "codex"}); err == nil {
		t.Error("expected error when --intent is missing, got nil")
	}
}

func TestAgentWriteUnknownFlag(t *testing.T) {
	testutil.InitRepo(t)

	if err := AgentWrite([]string{"--intent", "x", "--bogus", "y"}); err == nil {
		t.Error("expected error for unknown flag, got nil")
	}
}

func TestAgentWriteMissingFlagValue(t *testing.T) {
	testutil.InitRepo(t)

	if err := AgentWrite([]string{"--intent"}); err == nil {
		t.Error("expected error when --intent has no value, got nil")
	}
}

func TestLoadAgentContextMissing(t *testing.T) {
	testutil.InitRepo(t)

	ctx, ok, err := LoadAgentContext()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok || ctx != nil {
		t.Errorf("expected (nil, false) for missing context, got (%v, %v)", ctx, ok)
	}
}

func TestLoadAgentContextCorrupt(t *testing.T) {
	testutil.InitRepo(t)

	if err := os.MkdirAll(filepath.Join(".agit", "state"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(".agit", "state", "current.json"), []byte("{corrupt"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, ok, err := LoadAgentContext()
	if err == nil {
		t.Error("expected error for corrupt context file, got nil")
	}
	if !ok {
		t.Error("expected ok=true when the file exists but is corrupt")
	}
}

func TestSaveAgentContextDefaults(t *testing.T) {
	testutil.InitRepo(t)

	ctx := &AgentContext{IntentSummary: "do thing"}
	if err := SaveAgentContext(ctx); err != nil {
		t.Fatalf("SaveAgentContext failed: %v", err)
	}

	loaded, ok, err := LoadAgentContext()
	if err != nil || !ok {
		t.Fatalf("LoadAgentContext after save: ok=%v err=%v", ok, err)
	}
	if loaded.Provider != "unknown-agent" {
		t.Errorf("Provider = %q, want unknown-agent default", loaded.Provider)
	}
	if loaded.SchemaVersion != "agit.agent-context.v0" {
		t.Errorf("SchemaVersion = %q", loaded.SchemaVersion)
	}
	if loaded.CommitMessageSuggestion != "do thing" {
		t.Errorf("CommitMessageSuggestion = %q, want intent default", loaded.CommitMessageSuggestion)
	}
	if loaded.CreatedAt == "" {
		t.Error("CreatedAt was not defaulted")
	}
}

func TestSplitCSV(t *testing.T) {
	got := splitCSV("a.go, b.go ,,c.go,")
	want := []string{"a.go", "b.go", "c.go"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("splitCSV = %v, want %v", got, want)
	}
	if got := splitCSV(""); len(got) != 0 {
		t.Errorf("splitCSV(\"\") = %v, want empty", got)
	}
}

func TestFirstNonEmpty(t *testing.T) {
	if got := firstNonEmpty("", "", "c"); got != "c" {
		t.Errorf("firstNonEmpty = %q, want c", got)
	}
	if got := firstNonEmpty("", ""); got != "" {
		t.Errorf("firstNonEmpty = %q, want empty", got)
	}
}
