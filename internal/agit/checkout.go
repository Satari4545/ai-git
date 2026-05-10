package agit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	gitwrap "github.com/Satari4545/ai-git/internal/git"
)

type RestoredCheckoutContext struct {
	SchemaVersion string          `json:"schemaVersion"`
	Commit        string          `json:"commit"`
	Branch        string          `json:"branch,omitempty"`
	RestoredAt    string          `json:"restoredAt"`
	Note          json.RawMessage `json:"note"`
}

func Checkout(args []string) error {
	if err := gitwrap.Run(append([]string{"checkout"}, args...)...); err != nil {
		return err
	}

	restored, err := RestoreHEADContext()
	if err != nil {
		return err
	}

	if restored {
		fmt.Println("agit: restored checkout context from HEAD")
	} else {
		fmt.Println("agit: no AGIT context note found on HEAD")
	}

	return nil
}

func RestoreHEADContext() (bool, error) {
	head, err := gitwrap.OutputQuiet("rev-parse", "HEAD")
	if err != nil {
		return false, err
	}

	note, err := gitwrap.OutputQuiet("notes", "--ref=agit", "show", head)
	if err != nil || note == "" {
		return false, nil
	}

	var rawNote json.RawMessage
	if err := json.Unmarshal([]byte(note), &rawNote); err != nil {
		return false, fmt.Errorf("invalid AGIT note on HEAD: %w", err)
	}

	branch, _ := gitwrap.OutputQuiet("branch", "--show-current")

	restored := RestoredCheckoutContext{
		SchemaVersion: "agit.checkout-context.v0",
		Commit:        head,
		Branch:        branch,
		RestoredAt:    time.Now().UTC().Format(time.RFC3339),
		Note:          rawNote,
	}

	raw, err := json.MarshalIndent(restored, "", "  ")
	if err != nil {
		return false, err
	}

	if err := os.MkdirAll(filepath.Join(".agit", "state"), 0755); err != nil {
		return false, err
	}

	path := filepath.Join(".agit", "state", "checkout.json")
	if err := os.WriteFile(path, raw, 0644); err != nil {
		return false, err
	}

	return true, nil
}

func LoadCheckoutContextRaw() ([]byte, bool, error) {
	path := filepath.Join(".agit", "state", "checkout.json")

	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	return raw, true, nil
}

func PrintCheckoutContext() error {
	raw, ok, err := LoadCheckoutContextRaw()
	if err != nil {
		return err
	}

	if !ok {
		fmt.Println("No checkout context has been restored. Run `agit checkout <branch>` on a commit with AGIT notes.")
		return nil
	}

	fmt.Println(string(raw))
	return nil
}
