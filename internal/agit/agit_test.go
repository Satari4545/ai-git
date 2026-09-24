package agit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Satari4545/ai-git/internal/testutil"
)

// addAgitNote attaches raw JSON as an AGIT note to the given commit,
// using the exact ref the product reads and writes.
func addAgitNote(t *testing.T, dir, commit, noteJSON string) {
	t.Helper()

	noteFile := filepath.Join(dir, ".agit-test-note.json")
	testutil.WriteFile(t, noteFile, noteJSON)
	testutil.RunGit(t, dir, "notes", "--ref=agit", "add", "-f", "-F", noteFile, commit)
	if err := os.Remove(noteFile); err != nil {
		t.Fatal(err)
	}
}
