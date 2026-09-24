package state

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Satari4545/ai-git/internal/testutil"
)

var contextIDPattern = regexp.MustCompile(`^ctx_sha256:[0-9a-f]{64}$`)

func TestCaptureWithStagedChanges(t *testing.T) {
	dir := testutil.InitRepo(t)
	testutil.StageFile(t, dir, "feat/new.go", "package feat\n")
	testutil.StageFile(t, dir, "README.md", "# docs\n")

	delta, contextID, raw, err := Capture("implement feature")
	if err != nil {
		t.Fatalf("Capture failed: %v", err)
	}

	if delta.AgentThinking.IntentSummary != "implement feature" {
		t.Errorf("IntentSummary = %q", delta.AgentThinking.IntentSummary)
	}
	for _, want := range []string{"feat/new.go", "README.md"} {
		found := false
		for _, f := range delta.CodeReality.ActiveFiles {
			if f == want {
				found = true
			}
		}
		if !found {
			t.Errorf("ActiveFiles = %v, missing %q", delta.CodeReality.ActiveFiles, want)
		}
	}
	if delta.CodeReality.DiffStat == "" {
		t.Error("expected DiffStat to be populated for staged changes")
	}
	if delta.SchemaVersion != "agit.logic-delta.v0" {
		t.Errorf("SchemaVersion = %q", delta.SchemaVersion)
	}
	if delta.Kind != "logic_delta" {
		t.Errorf("Kind = %q", delta.Kind)
	}
	if !contextIDPattern.MatchString(contextID) {
		t.Errorf("contextID %q does not match ctx_sha256:<64 hex>", contextID)
	}

	// The raw payload must be self-consistent JSON carrying the same ID.
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("raw payload is not valid JSON: %v", err)
	}
	if _, err := time.Parse(time.RFC3339, delta.CreatedAt); err != nil {
		t.Errorf("CreatedAt %q is not RFC3339: %v", delta.CreatedAt, err)
	}
}

func TestCaptureEmptyIntent(t *testing.T) {
	dir := testutil.InitRepo(t)
	testutil.StageFile(t, dir, "a.go", "package a\n")

	delta, contextID, _, err := Capture("")
	if err != nil {
		t.Fatalf("Capture failed: %v", err)
	}
	if !strings.Contains(delta.AgentThinking.IntentSummary, "No intent summary") {
		t.Errorf("expected default intent message, got %q", delta.AgentThinking.IntentSummary)
	}
	if !contextIDPattern.MatchString(contextID) {
		t.Errorf("contextID %q has wrong format", contextID)
	}
}

func TestCaptureNoStagedChanges(t *testing.T) {
	testutil.InitRepo(t)

	delta, contextID, _, err := Capture("nothing staged")
	if err != nil {
		t.Fatalf("Capture failed: %v", err)
	}
	if len(delta.CodeReality.ActiveFiles) != 0 {
		t.Errorf("ActiveFiles = %v, want empty", delta.CodeReality.ActiveFiles)
	}
	if !contextIDPattern.MatchString(contextID) {
		t.Errorf("contextID %q has wrong format", contextID)
	}
}

func TestCaptureContextIDBindsToContent(t *testing.T) {
	dir := testutil.InitRepo(t)
	testutil.StageFile(t, dir, "a.go", "package a\n")

	delta1, id1, _, err := Capture("first intent")
	if err != nil {
		t.Fatal(err)
	}
	delta2, id2, _, err := Capture("second intent")
	if err != nil {
		t.Fatal(err)
	}
	if id1 == id2 {
		t.Error("different intents produced the same context ID; the ID must bind to content")
	}

	// The ID must be the SHA-256 of the canonical (compact) JSON payload
	// Capture hashes, not just a unique-per-call value.
	for i, tc := range []struct {
		delta *LogicDelta
		id    string
	}{
		{delta1, id1},
		{delta2, id2},
	} {
		raw, err := json.Marshal(tc.delta)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(raw)
		want := "ctx_sha256:" + hex.EncodeToString(sum[:])
		if tc.id != want {
			t.Errorf("capture %d: context ID %q != sha256 of captured payload (%q)", i+1, tc.id, want)
		}
	}
}

func TestMapFilesToResources(t *testing.T) {
	res := mapFilesToResources([]string{"a.go", "dir/b.go"})
	if len(res) != 2 {
		t.Fatalf("got %d resources, want 2", len(res))
	}
	if res[0].URI != "file://a.go" || res[0].Name != "a.go" {
		t.Errorf("unexpected resource: %+v", res[0])
	}
	if got := mapFilesToResources(nil); len(got) != 0 {
		t.Errorf("expected empty resources for nil, got %v", got)
	}
}
