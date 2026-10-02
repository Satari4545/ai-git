package agit

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	gitwrap "github.com/Satari4545/ai-git/internal/git"
)

type AgentContext struct {
	SchemaVersion           string   `json:"schemaVersion"`
	Provider                string   `json:"provider"`
	TaskID                  string   `json:"taskID,omitempty"`
	Repo                    string   `json:"repo,omitempty"`
	Branch                  string   `json:"branch,omitempty"`
	BaseCommit              string   `json:"baseCommit,omitempty"`
	IntentSummary           string   `json:"intentSummary"`
	CommitMessageSuggestion string   `json:"commitMessageSuggestion"`
	ActiveFiles             []string `json:"activeFiles"`
	PlannedDelta            string   `json:"plannedDelta"`
	ActualDelta             string   `json:"actualDelta"`
	Decisions               []string `json:"decisions,omitempty"`
	Risks                   []string `json:"risks,omitempty"`
	NextSteps               []string `json:"nextSteps,omitempty"`
	CreatedAt               string   `json:"createdAt"`
}

type agentContextInput struct {
	SchemaVersion           string   `json:"schemaVersion"`
	Provider                string   `json:"provider"`
	TaskID                  string   `json:"taskID"`
	Repo                    string   `json:"repo"`
	Branch                  string   `json:"branch"`
	BaseCommit              string   `json:"baseCommit"`
	Intent                  string   `json:"intent"`
	IntentSummary           string   `json:"intentSummary"`
	Message                 string   `json:"message"`
	CommitMessageSuggestion string   `json:"commitMessageSuggestion"`
	Files                   []string `json:"files"`
	ActiveFiles             []string `json:"activeFiles"`
	Planned                 string   `json:"planned"`
	PlannedDelta            string   `json:"plannedDelta"`
	Actual                  string   `json:"actual"`
	ActualDelta             string   `json:"actualDelta"`
	Decisions               []string `json:"decisions"`
	Risks                   []string `json:"risks"`
	NextSteps               []string `json:"nextSteps"`
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

		case "--task-id":
			if i+1 >= len(args) {
				return errors.New("--task-id requires a value")
			}
			ctx.TaskID = args[i+1]
			i++

		case "--repo":
			if i+1 >= len(args) {
				return errors.New("--repo requires a value")
			}
			ctx.Repo = args[i+1]
			i++

		case "--branch":
			if i+1 >= len(args) {
				return errors.New("--branch requires a value")
			}
			ctx.Branch = args[i+1]
			i++

		case "--base-commit":
			if i+1 >= len(args) {
				return errors.New("--base-commit requires a value")
			}
			ctx.BaseCommit = args[i+1]
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

		case "--decisions":
			if i+1 >= len(args) {
				return errors.New("--decisions requires a value")
			}
			ctx.Decisions = splitCSV(args[i+1])
			i++

		case "--risks":
			if i+1 >= len(args) {
				return errors.New("--risks requires a value")
			}
			ctx.Risks = splitCSV(args[i+1])
			i++

		case "--next-steps":
			if i+1 >= len(args) {
				return errors.New("--next-steps requires a value")
			}
			ctx.NextSteps = splitCSV(args[i+1])
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

	populateGitContext(&ctx)

	if err := SaveAgentContext(&ctx); err != nil {
		return err
	}

	if os.Getenv("AGIT_MCP_SILENT") != "1" {
		fmt.Println("Agent context written to", filepath.Join(".agit", "state", "current.json"))
		fmt.Println("Suggested commit message:", ctx.CommitMessageSuggestion)
	}

	return nil
}

func SaveAgentContext(ctx *AgentContext) error {
	if ctx.SchemaVersion == "" {
		ctx.SchemaVersion = "agit.agent-context.v0"
	}

	if ctx.CreatedAt == "" {
		ctx.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}

	if ctx.Provider == "" {
		ctx.Provider = "unknown-agent"
	}

	if ctx.CommitMessageSuggestion == "" {
		ctx.CommitMessageSuggestion = ctx.IntentSummary
	}

	populateGitContext(ctx)

	// Sanitize before persisting: covers the state file and, via
	// LoadAgentContext, the git note commit.go attaches.
	SanitizeAgentContext(ctx)

	if err := os.MkdirAll(filepath.Join(".agit", "state"), 0755); err != nil {
		return err
	}

	raw, err := json.MarshalIndent(ctx, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(".agit", "state", "current.json")
	return os.WriteFile(path, raw, 0644)
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

func ParseAgentContextJSON(raw []byte) (*AgentContext, error) {
	var input agentContextInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return nil, err
	}

	ctx := &AgentContext{
		SchemaVersion:           input.SchemaVersion,
		Provider:                input.Provider,
		TaskID:                  input.TaskID,
		Repo:                    input.Repo,
		Branch:                  input.Branch,
		BaseCommit:              input.BaseCommit,
		IntentSummary:           firstNonEmpty(input.IntentSummary, input.Intent),
		CommitMessageSuggestion: firstNonEmpty(input.CommitMessageSuggestion, input.Message),
		ActiveFiles:             firstNonEmptySlice(input.ActiveFiles, input.Files),
		PlannedDelta:            firstNonEmpty(input.PlannedDelta, input.Planned),
		ActualDelta:             firstNonEmpty(input.ActualDelta, input.Actual),
		Decisions:               input.Decisions,
		Risks:                   input.Risks,
		NextSteps:               input.NextSteps,
		CreatedAt:               time.Now().UTC().Format(time.RFC3339),
	}

	if ctx.IntentSummary == "" {
		return nil, errors.New("provider context missing required intent or intentSummary")
	}

	return ctx, nil
}

func populateGitContext(ctx *AgentContext) {
	if ctx.Repo == "" {
		ctx.Repo, _ = gitwrap.OutputQuiet("config", "--get", "remote.origin.url")
	}

	if ctx.Branch == "" {
		ctx.Branch, _ = gitwrap.OutputQuiet("branch", "--show-current")
	}

	if ctx.BaseCommit == "" {
		ctx.BaseCommit, _ = gitwrap.OutputQuiet("rev-parse", "HEAD")
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func firstNonEmptySlice(values ...[]string) []string {
	for _, value := range values {
		if len(value) > 0 {
			return value
		}
	}
	return nil
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
