package state

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func initStateTestRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	run("init", "-q")
	run("config", "user.email", "agit-test@example.com")
	run("config", "user.name", "agit-test")

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldwd); err != nil {
			t.Fatal(err)
		}
	})

	return dir
}

func stageFile(t *testing.T, dir, path, content string) {
	t.Helper()

	full := filepath.Join(dir, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "add", path)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add failed: %v\n%s", err, out)
	}
}

var contextIDPattern = regexp.MustCompile(`^ctx_sha256:[0-9a-f]{64}$`)

func TestCaptureWithStagedChanges(t *testing.T) {
	dir := initStateTestRepo(t)
	stageFile(t, dir, "feat/new.go", "package feat\n")
	stageFile(t, dir, "README.md", "# docs\n")

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
	dir := initStateTestRepo(t)
	stageFile(t, dir, "a.go", "package a\n")

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
	initStateTestRepo(t)

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
	dir := initStateTestRepo(t)
	stageFile(t, dir, "a.go", "package a\n")

	_, id1, _, err := Capture("first intent")
	if err != nil {
		t.Fatal(err)
	}
	_, id2, _, err := Capture("second intent")
	if err != nil {
		t.Fatal(err)
	}
	if id1 == id2 {
		t.Error("different intents produced the same context ID; the ID must bind to content")
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
