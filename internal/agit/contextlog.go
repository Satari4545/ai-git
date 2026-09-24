package agit

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	gitwrap "github.com/Satari4545/ai-git/internal/git"
)

// contextLogDefaults holds the default options for `agit context log`.
const defaultContextLogCount = 20

type contextLogOptions struct {
	maxCount int
	asJSON   bool
	paths    []string
}

// ContextLog implements `agit context log`: walk recent commits and show the
// AGIT context note attached to each one, if any.
func ContextLog(args []string) error {
	opts, err := parseContextLogArgs(args)
	if err != nil {
		return err
	}

	entries, err := buildContextLog(opts)
	if err != nil {
		return err
	}

	if opts.asJSON {
		raw, err := json.MarshalIndent(entries, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(raw))
		return nil
	}

	printContextLog(entries)
	return nil
}

func parseContextLogArgs(args []string) (contextLogOptions, error) {
	opts := contextLogOptions{maxCount: defaultContextLogCount}

	pathsOnly := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		if pathsOnly {
			opts.paths = append(opts.paths, a)
			continue
		}
		switch {
		case a == "--":
			pathsOnly = true
		case a == "--json":
			opts.asJSON = true
		case a == "-n" || a == "--max-count":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("flag %s requires a value", a)
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil || n < 1 {
				return opts, fmt.Errorf("invalid max count %q: must be a positive integer", args[i])
			}
			opts.maxCount = n
		case strings.HasPrefix(a, "--max-count="):
			n, err := strconv.Atoi(strings.TrimPrefix(a, "--max-count="))
			if err != nil || n < 1 {
				return opts, fmt.Errorf("invalid max count %q: must be a positive integer", a)
			}
			opts.maxCount = n
		case strings.HasPrefix(a, "-") && a != "-":
			return opts, fmt.Errorf("unknown flag %q (usage: agit context log [-n N] [--json] [-- <path>...])", a)
		default:
			opts.paths = append(opts.paths, a)
		}
	}

	return opts, nil
}

// contextLogEntry is one commit plus its optional AGIT context summary.
type contextLogEntry struct {
	Commit    string       `json:"commit"`
	Short     string       `json:"short"`
	Date      string       `json:"date"`
	Subject   string       `json:"subject"`
	HasNote   bool         `json:"hasNote"`
	NoteError string       `json:"noteError,omitempty"`
	Summary   *noteSummary `json:"summary,omitempty"`
}

// noteSummary is the compact, human-useful slice of an AGIT note.
// It is parsed defensively: unknown or missing fields never fail the parse.
type noteSummary struct {
	ContextID string   `json:"contextID"`
	Intent    string   `json:"intent,omitempty"`
	Provider  string   `json:"provider,omitempty"`
	Files     []string `json:"files,omitempty"`
	Decisions []string `json:"decisions,omitempty"`
	Risks     []string `json:"risks,omitempty"`
	NextSteps []string `json:"nextSteps,omitempty"`
	CreatedAt string   `json:"createdAt,omitempty"`
}

// agitNote mirrors only the fields of a commit note that context log needs.
// Everything else in the note is ignored so schema additions stay compatible.
type agitNote struct {
	ContextID  string `json:"contextID"`
	LogicState *struct {
		AgentThinking *struct {
			IntentSummary string   `json:"intentSummary"`
			RiskFlags     []string `json:"riskFlags"`
		} `json:"agentThinking"`
		CodeReality *struct {
			ActiveFiles []string `json:"activeFiles"`
		} `json:"codeReality"`
		CreatedAt string `json:"createdAt"`
	} `json:"logicState"`
	AgentContext *struct {
		Provider    string   `json:"provider"`
		Intent      string   `json:"intentSummary"`
		Decisions   []string `json:"decisions"`
		Risks       []string `json:"risks"`
		NextSteps   []string `json:"nextSteps"`
		ActiveFiles []string `json:"activeFiles"`
		CreatedAt   string   `json:"createdAt"`
	} `json:"agentContext"`
}

// summarizeNote extracts the compact summary from a raw AGIT note.
// A malformed or empty note returns an error; the caller decides how to show it.
func summarizeNote(raw []byte) (*noteSummary, error) {
	var note agitNote
	if err := json.Unmarshal(raw, &note); err != nil {
		return nil, fmt.Errorf("unreadable note: %w", err)
	}

	summary := &noteSummary{ContextID: note.ContextID}

	if note.LogicState != nil {
		if note.LogicState.AgentThinking != nil {
			summary.Intent = note.LogicState.AgentThinking.IntentSummary
			summary.Risks = append(summary.Risks, note.LogicState.AgentThinking.RiskFlags...)
		}
		if note.LogicState.CodeReality != nil {
			summary.Files = append(summary.Files, note.LogicState.CodeReality.ActiveFiles...)
		}
		if summary.CreatedAt == "" {
			summary.CreatedAt = note.LogicState.CreatedAt
		}
	}

	if note.AgentContext != nil {
		ac := note.AgentContext
		summary.Provider = ac.Provider
		if summary.Intent == "" {
			summary.Intent = ac.Intent
		}
		if len(summary.Files) == 0 {
			summary.Files = append(summary.Files, ac.ActiveFiles...)
		}
		summary.Decisions = append(summary.Decisions, ac.Decisions...)
		summary.Risks = append(summary.Risks, ac.Risks...)
		summary.NextSteps = append(summary.NextSteps, ac.NextSteps...)
		if summary.CreatedAt == "" {
			summary.CreatedAt = ac.CreatedAt
		}
	}

	return summary, nil
}

func buildContextLog(opts contextLogOptions) ([]contextLogEntry, error) {
	logArgs := []string{
		"log",
		"--format=%H%x00%h%x00%ad%x00%s",
		"--date=short",
		"-n", strconv.Itoa(opts.maxCount),
	}
	if len(opts.paths) > 0 {
		logArgs = append(logArgs, "--")
		logArgs = append(logArgs, opts.paths...)
	}

	out, err := gitwrap.OutputQuiet(logArgs...)
	if err != nil {
		return nil, fmt.Errorf("git log failed: %w", err)
	}
	if out == "" {
		return []contextLogEntry{}, nil
	}

	entries := make([]contextLogEntry, 0, opts.maxCount)
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Split(line, "\x00")
		if len(fields) != 4 {
			continue
		}
		entry := contextLogEntry{
			Commit:  fields[0],
			Short:   fields[1],
			Date:    fields[2],
			Subject: fields[3],
		}

		noteRaw, noteErr := gitwrap.OutputQuiet("notes", "--ref=agit", "show", entry.Commit)
		if noteErr != nil || noteRaw == "" {
			entries = append(entries, entry)
			continue
		}

		entry.HasNote = true
		summary, sumErr := summarizeNote([]byte(noteRaw))
		if sumErr != nil {
			entry.NoteError = sumErr.Error()
		} else {
			entry.Summary = summary
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

func printContextLog(entries []contextLogEntry) {
	if len(entries) == 0 {
		fmt.Println("No commits found.")
		return
	}

	for _, e := range entries {
		fmt.Printf("* %s %s %s\n", e.Short, e.Date, e.Subject)
		if !e.HasNote {
			fmt.Println("  no agit context")
			continue
		}
		if e.NoteError != "" {
			fmt.Println("  agit note unreadable:", e.NoteError)
			continue
		}
		s := e.Summary
		if s.ContextID != "" {
			fmt.Println("  context:", shortContextID(s.ContextID))
		}
		if s.Provider != "" {
			fmt.Println("  provider:", s.Provider)
		}
		if s.Intent != "" {
			fmt.Println("  intent:", truncateLine(s.Intent, 100))
		}
		if len(s.Files) > 0 {
			fmt.Println("  files:", strings.Join(truncateList(s.Files, 5), ", "))
		}
		if len(s.Decisions) > 0 {
			fmt.Println("  decisions:", strings.Join(truncateList(s.Decisions, 3), "; "))
		}
		if len(s.Risks) > 0 {
			fmt.Println("  risks:", strings.Join(truncateList(s.Risks, 3), "; "))
		}
		if len(s.NextSteps) > 0 {
			fmt.Println("  next:", strings.Join(truncateList(s.NextSteps, 3), "; "))
		}
	}
}

func shortContextID(id string) string {
	const prefix = "ctx_sha256:"
	if strings.HasPrefix(id, prefix) && len(id) > len(prefix)+12 {
		return id[:len(prefix)+12] + "..."
	}
	return id
}

func truncateLine(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func truncateList(items []string, max int) []string {
	if len(items) <= max {
		return items
	}
	out := append([]string{}, items[:max]...)
	out = append(out, fmt.Sprintf("+%d more", len(items)-max))
	return out
}
