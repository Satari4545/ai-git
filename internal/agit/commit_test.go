package agit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/Satari4545/ai-git/internal/state"
	"github.com/Satari4545/ai-git/internal/testutil"
)

func TestExtractIntent(t *testing.T) {
	t.Setenv("AGIT_INTENT", "from-env")

	tests := []struct {
		name string
		args []string
		want string
	}{
		{"space form", []string{"--intent", "from-flag"}, "from-flag"},
		{"equals form", []string{"--intent=from-equals"}, "from-equals"},
		{"flag beats env", []string{"--intent", "flag-wins"}, "flag-wins"},
		{"falls back to env", []string{"-m", "msg"}, "from-env"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractIntent(tt.args); got != tt.want {
				t.Errorf("extractIntent(%v) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}

	t.Run("empty without env", func(t *testing.T) {
		t.Setenv("AGIT_INTENT", "")
		if got := extractIntent(nil); got != "" {
			t.Errorf("extractIntent(nil) = %q, want empty", got)
		}
	})
}

func TestStripAgitArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			"removes intent pair and quiet",
			[]string{"-m", "msg", "--intent", "summarize", "--quiet"},
			[]string{"-m", "msg"},
		},
		{
			"removes equals form and passthrough flags",
			[]string{"--intent=x", "--allow-stale-context", "--allow-missing-context", "--no-agent-context", "HEAD"},
			[]string{"HEAD"},
		},
		{
			"keeps git args untouched",
			[]string{"-m", "msg", "--amend"},
			[]string{"-m", "msg", "--amend"},
		},
		{
			"dangling intent consumes nothing else",
			[]string{"--intent"},
			[]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stripAgitArgs(tt.args); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("stripAgitArgs(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

func TestHasCommitMessage(t *testing.T) {
	tests := []struct {
		args []string
		want bool
	}{
		{[]string{"-m", "msg"}, true},
		{[]string{"--message", "msg"}, true},
		{[]string{"-mmsg"}, true},
		{[]string{"--message=msg"}, true},
		{[]string{"-m"}, false},        // flag without value
		{[]string{"--message"}, false}, // flag without value
		{[]string{"--amend"}, false},   // similar prefix, not a message
		{nil, false},
	}

	for _, tt := range tests {
		if got := hasCommitMessage(tt.args); got != tt.want {
			t.Errorf("hasCommitMessage(%v) = %v, want %v", tt.args, got, tt.want)
		}
	}
}

func TestHasCommitMessageFile(t *testing.T) {
	tests := []struct {
		args []string
		want bool
	}{
		{[]string{"-F", "file"}, true},
		{[]string{"--file", "file"}, true},
		{[]string{"-Ffile"}, true},
		{[]string{"--file=file"}, true},
		{[]string{"-F"}, false},
		{nil, false},
	}

	for _, tt := range tests {
		if got := hasCommitMessageFile(tt.args); got != tt.want {
			t.Errorf("hasCommitMessageFile(%v) = %v, want %v", tt.args, got, tt.want)
		}
	}
}

func TestAppendContextIDTrailer(t *testing.T) {
	const id = "ctx_sha256:deadbeef"

	t.Run("appends when message present", func(t *testing.T) {
		args, included := appendContextIDTrailer([]string{"-m", "msg"}, id)
		if !included {
			t.Fatal("expected trailer to be included")
		}
		want := []string{"-m", "msg", "-m", "Agit-Context-ID: " + id}
		if !reflect.DeepEqual(args, want) {
			t.Errorf("args = %v, want %v", args, want)
		}
	})

	t.Run("skips without message", func(t *testing.T) {
		args, included := appendContextIDTrailer([]string{"--amend"}, id)
		if included || !reflect.DeepEqual(args, []string{"--amend"}) {
			t.Errorf("args = %v, included = %v; want unchanged and false", args, included)
		}
	})

	t.Run("skips with empty id", func(t *testing.T) {
		args, included := appendContextIDTrailer([]string{"-m", "msg"}, "")
		if included || !reflect.DeepEqual(args, []string{"-m", "msg"}) {
			t.Errorf("args = %v, included = %v; want unchanged and false", args, included)
		}
	})

	t.Run("skips when message comes from file", func(t *testing.T) {
		args, included := appendContextIDTrailer([]string{"-F", "msg.txt"}, id)
		if included || !reflect.DeepEqual(args, []string{"-F", "msg.txt"}) {
			t.Errorf("args = %v, included = %v; want unchanged and false", args, included)
		}
	})
}

func TestExtractCommitMessage(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"-m", "hello"}, "hello"},
		{[]string{"--message", "hi"}, "hi"},
		{[]string{"-mhi"}, "hi"},
		{[]string{"--message=yo"}, "yo"},
		{[]string{"--amend"}, ""},
		{nil, ""},
	}

	for _, tt := range tests {
		if got := extractCommitMessage(tt.args); got != tt.want {
			t.Errorf("extractCommitMessage(%v) = %q, want %q", tt.args, got, tt.want)
		}
	}
}

func TestHasAgitFlag(t *testing.T) {
	args := []string{"commit", "--quiet", "-m", "x"}
	if !hasAgitFlag(args, "--quiet") {
		t.Error("expected --quiet to be found")
	}
	if hasAgitFlag(args, "--intent") {
		t.Error("expected --intent to be absent")
	}
}

// TestCommitAttachesTrailerAndNote exercises the full Commit wiring: it must
// embed the Agit-Context-ID trailer in the commit message and attach a
// refs/notes/agit note to HEAD carrying the same ID bound to the captured
// payload.
func TestCommitAttachesTrailerAndNote(t *testing.T) {
	// Commit refreshes from the provider before honoring
	// --allow-missing-context, and the provider is driven by
	// AGIT_CONTEXT_COMMAND. Clear it so this test stays hermetic even
	// where that variable is set in the ambient environment.
	t.Setenv("AGIT_CONTEXT_COMMAND", "")

	dir := testutil.InitRepo(t)
	testutil.StageFile(t, dir, "feat.go", "package feat\n")

	if err := Commit([]string{"--quiet", "--allow-missing-context", "--intent", "ship feature", "-m", "add feature"}); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	msg := testutil.RunGit(t, dir, "log", "-1", "--format=%B")
	var trailerID string
	for _, line := range strings.Split(msg, "\n") {
		if id, ok := strings.CutPrefix(line, "Agit-Context-ID: "); ok {
			trailerID = strings.TrimSpace(id)
		}
	}
	if trailerID == "" {
		t.Fatalf("commit message has no Agit-Context-ID trailer:\n%s", msg)
	}

	head := strings.TrimSpace(testutil.RunGit(t, dir, "rev-parse", "HEAD"))
	noteRaw := testutil.RunGit(t, dir, "notes", "--ref=agit", "show", head)
	var note struct {
		ContextID  string          `json:"contextID"`
		LogicState json.RawMessage `json:"logicState"`
	}
	if err := json.Unmarshal([]byte(noteRaw), &note); err != nil {
		t.Fatalf("note on HEAD is not valid JSON: %v\n%s", err, noteRaw)
	}
	if note.ContextID == "" {
		t.Fatal("note on HEAD has no contextID")
	}
	if note.ContextID != trailerID {
		t.Errorf("note contextID %q does not match commit trailer %q", note.ContextID, trailerID)
	}

	// The ID must bind to the payload the note carries.
	var delta state.LogicDelta
	if err := json.Unmarshal(note.LogicState, &delta); err != nil {
		t.Fatalf("note logicState is not a valid logic delta: %v", err)
	}
	raw, err := json.Marshal(&delta)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	if want := "ctx_sha256:" + hex.EncodeToString(sum[:]); note.ContextID != want {
		t.Errorf("note contextID %q != sha256 of note payload (%q)", note.ContextID, want)
	}
	if delta.AgentThinking.IntentSummary != "ship feature" {
		t.Errorf("note intent = %q, want %q", delta.AgentThinking.IntentSummary, "ship feature")
	}
}
