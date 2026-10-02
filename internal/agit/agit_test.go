package agit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Satari4545/ai-git/internal/testutil"
)

func addAgitNote(t *testing.T, dir, commit, noteJSON string) {
	t.Helper()

	noteFile := filepath.Join(dir, ".agit-test-note.json")
	testutil.WriteFile(t, noteFile, noteJSON)
	testutil.RunGit(t, dir, "notes", "--ref=agit", "add", "-f", "-F", noteFile, commit)
	if err := os.Remove(noteFile); err != nil {
		t.Fatal(err)
	}
}
