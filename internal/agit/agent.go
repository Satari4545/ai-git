package agit

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type AgentContext struct {
	SchemaVersion           string   `json:"schemaVersion"`
	Provider                string   `json:"provider"`
	IntentSummary           string   `json:"intentSummary"`
	CommitMessageSuggestion string   `json:"commitMessageSuggestion"`
	ActiveFiles             []string `json:"activeFiles"`
	PlannedDelta            string   `json:"plannedDelta"`
	ActualDelta             string   `json:"actualDelta"`
	CreatedAt               string   `json:"createdAt"`
}

func AgentWrite(args []string) error {
	ctx := AgentContext{
		SchemaVersion: "agit.agent-context.v0",
		Provider:      "manual-cli",
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
	}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--provider":
			if i+1 >= len(args) {
				return errors.New("--provider requires a value")
			}
			ctx.Provider = args[i+1]
			i++

		case "--intent":
			if i+1 >= len(args) {
				return errors.New("--intent requires a value")
			}
			ctx.IntentSummary = args[i+1]
			i++

		case "--message":
			if i+1 >= len(args) {
				return errors.New("--message requires a value")
			}
			ctx.CommitMessageSuggestion = args[i+1]
			i++

		case "--files":
			if i+1 >= len(args) {
				return errors.New("--files requires a value")
			}
			ctx.ActiveFiles = splitCSV(args[i+1])
			i++

		case "--planned":
			if i+1 >= len(args) {
				return errors.New("--planned requires a value")
			}
			ctx.PlannedDelta = args[i+1]
			i++

		case "--actual":
			if i+1 >= len(args) {
				return errors.New("--actual requires a value")
			}
			ctx.ActualDelta = args[i+1]
			i++

		default:
			return fmt.Errorf("unknown agit agent write option: %s", args[i])
		}
	}

	if ctx.IntentSummary == "" {
		return errors.New("missing required --intent")
	}

	if ctx.CommitMessageSuggestion == "" {
		ctx.CommitMessageSuggestion = ctx.IntentSummary
	}

	if ctx.Provider == "" {
		ctx.Provider = "unknown-agent"
	}

	if err := os.MkdirAll(filepath.Join(".agit", "state"), 0755); err != nil {
		return err
	}

	raw, err := json.MarshalIndent(ctx, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(".agit", "state", "current.json")
	if err := os.WriteFile(path, raw, 0644); err != nil {
		return err
	}

	fmt.Println("Agent context written to", path)
	fmt.Println("Suggested commit message:", ctx.CommitMessageSuggestion)

	return nil
}

func LoadAgentContext() (*AgentContext, bool, error) {
	path := filepath.Join(".agit", "state", "current.json")

	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	var ctx AgentContext
	if err := json.Unmarshal(raw, &ctx); err != nil {
		return nil, true, err
	}

	return &ctx, true, nil
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}

	return out
}