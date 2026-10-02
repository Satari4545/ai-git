package agit

import (
	"fmt"
	"strconv"
	"strings"

	gitwrap "github.com/Satari4545/ai-git/internal/git"
)

const (
	// defaultHistoryCount is used when the caller does not specify a count.
	defaultHistoryCount = 10
	// maxHistoryCount caps history reads so a single query cannot dump an
	// unbounded number of notes into an agent's context window.
	maxHistoryCount = 50
)

// HistoryEntry is one commit's AGIT context, for agent consumption.
// Only commits that carry a readable AGIT note appear here.
type HistoryEntry struct {
	Commit    string   `json:"commit"`
	Short     string   `json:"short"`
	Date      string   `json:"date"`
	Subject   string   `json:"subject"`
	ContextID string   `json:"contextID,omitempty"`
	Intent    string   `json:"intent,omitempty"`
	Files     []string `json:"files,omitempty"`
	Decisions []string `json:"decisions,omitempty"`
	NextSteps []string `json:"nextSteps,omitempty"`
}

// GetHistory returns AGIT context entries for branch, newest first.
// Only commits carrying a readable AGIT note are included.
//
// n must be positive and is capped at maxHistoryCount. An empty branch
// means the current branch. An empty since means no date filter; otherwise
// since is passed to `git log --since` (e.g. "yesterday", "2026-09-01").
func GetHistory(n int, branch string, since string) ([]HistoryEntry, error) {
	count, err := normalizeHistoryCount(n)
	if err != nil {
		return nil, err
	}

	rev, err := resolveHistoryBranch(branch)
	if err != nil {
		return nil, err
	}

	entries, err := walkContextLogEntries(count, rev, since, nil)
	if err != nil {
		return nil, err
	}

	history := make([]HistoryEntry, 0, len(entries))
	for _, e := range entries {
		if !e.HasNote || e.Summary == nil {
			continue
		}
		s := e.Summary
		history = append(history, HistoryEntry{
			Commit:    e.Commit,
			Short:     e.Short,
			Date:      e.Date,
			Subject:   e.Subject,
			ContextID: s.ContextID,
			Intent:    s.Intent,
			Files:     s.Files,
			Decisions: s.Decisions,
			NextSteps: s.NextSteps,
		})
	}

	return history, nil
}

func normalizeHistoryCount(n int) (int, error) {
	if n <= 0 {
		return 0, fmt.Errorf("invalid history count %d: must be a positive integer", n)
	}
	if n > maxHistoryCount {
		return maxHistoryCount, nil
	}
	return n, nil
}

func resolveHistoryBranch(branch string) (string, error) {
	if branch == "" {
		out, err := gitwrap.OutputQuiet("rev-parse", "--abbrev-ref", "HEAD")
		if err != nil || out == "" || out == "HEAD" {
			return "HEAD", nil
		}
		return out, nil
	}
	if _, err := gitwrap.OutputQuiet("rev-parse", "--verify", "--quiet", branch); err != nil {
		return "", fmt.Errorf("unknown branch %q", branch)
	}
	return branch, nil
}

// walkContextLogEntries returns up to maxCount commits reachable from rev,
// each with its AGIT note summary attached. since filters via `git log
// --since`; paths optionally limits the walk to commits touching those paths.
func walkContextLogEntries(maxCount int, rev string, since string, paths []string) ([]contextLogEntry, error) {
	logArgs := []string{
		"log",
		"--format=%H%x00%h%x00%ad%x00%s",
		"--date=short",
		"-n", strconv.Itoa(maxCount),
		rev,
	}
	if since != "" {
		logArgs = append(logArgs, "--since="+since)
	}
	if len(paths) > 0 {
		logArgs = append(logArgs, "--")
		logArgs = append(logArgs, paths...)
	}

	out, err := gitwrap.OutputQuiet(logArgs...)
	if err != nil {
		if since != "" {
			return nil, fmt.Errorf("git log failed (bad --since %q?): %w", since, err)
		}
		return nil, fmt.Errorf("git log failed: %w", err)
	}
	if out == "" {
		return []contextLogEntry{}, nil
	}

	entries := make([]contextLogEntry, 0, maxCount)
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
