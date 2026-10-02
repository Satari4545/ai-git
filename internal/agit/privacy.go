package agit

import (
	"encoding/json"
	"regexp"
	"strings"
	"unicode/utf8"

	gitwrap "github.com/Satari4545/ai-git/internal/git"
)

// Privacy enforcement for the context write path.
//
// SaveAgentContext is the single choke point (CLI agent write, MCP
// record_context, provider refresh), and agit commit attaches whatever was
// saved there to the git note — so sanitizing there covers both the local
// state file and the note payload.
//
// Two mechanisms: secret redaction (default ON) and size limits (always
// on). Patterns are best-effort, not exhaustive; new shapes slot into
// secretPatterns without touching the write path.

const redactToken = "[REDACTED]"

// Per-field caps in runes: large enough to stay useful, small enough that
// one runaway field cannot dominate the payload.
const (
	maxIntentRunes   = 500
	maxMessageRunes  = 500
	maxDeltaRunes    = 2000
	maxMetaRunes     = 200
	maxListItemRunes = 200
	maxListItems     = 20
)

const maxPayloadBytes = 32 * 1024

const minShrinkRunes = 100

type secretPattern struct {
	name string
	re   *regexp.Regexp
	// repl, when non-empty, is the replacement template (may reference
	// capture groups). Empty means a plain redactToken replacement.
	repl string
}

var (
	// Matches key=value, key: value, "key": "value" shapes for common
	// credential names. The value (group 4) is redacted; the key name,
	// separator, and any quoting (groups 1-3) are preserved so the structure
	// stays readable.
	// The leading ([^a-zA-Z0-9]|^) guard keeps words like "tokenize" or
	// "passport" from matching while still catching DB_TOKEN-style names.
	credentialAssignmentRe = regexp.MustCompile(`(?i)([^a-zA-Z0-9]|^)(password|passwd|pwd|secret|token|api[_-]?key)(["']?\s*[:=]\s*["']?)([^\s"'` + "`" + `,;}\]]+)`)

	// Matches https://user:password@host shapes (e.g. a credential-bearing
	// remote URL recorded in the repo field). Only the password is redacted.
	urlCredentialsRe = regexp.MustCompile(`(https?://[^\s/:]+:)([^/\s@]+)(@)`)

	secretPatterns = []secretPattern{
		{
			name: "private-key-block",
			re:   regexp.MustCompile(`(?s)-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----.*?-----END [A-Z0-9 ]*PRIVATE KEY-----`),
		},
		{
			name: "openai-key",
			re:   regexp.MustCompile(`\bsk-[A-Za-z0-9\-_]{20,}\b`),
		},
		{
			name: "github-token",
			re:   regexp.MustCompile(`\b(?:ghp_|gho_|ghu_|ghs_|ghr_|github_pat_)[A-Za-z0-9_]{20,}\b`),
		},
		{
			name: "aws-access-key-id",
			re:   regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`),
		},
		{
			name: "stripe-key",
			re:   regexp.MustCompile(`\b[rs]k_live_[A-Za-z0-9]{16,}\b`),
		},
		{
			name: "slack-token",
			re:   regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9\-]{10,}\b`),
		},
		{
			name: "url-credentials",
			re:   urlCredentialsRe,
			repl: `${1}` + redactToken + `${3}`,
		},
		{
			name: "credential-assignment",
			re:   credentialAssignmentRe,
			repl: `${1}${2}${3}` + redactToken,
		},
	}
)

// Pure function: no I/O, safe to call anywhere.
func RedactString(s string) string {
	if s == "" {
		return s
	}
	for _, p := range secretPatterns {
		if p.repl != "" {
			s = p.re.ReplaceAllString(s, p.repl)
		} else {
			s = p.re.ReplaceAllString(s, redactToken)
		}
	}
	// A value already redacted by an earlier, more specific pattern can end
	// up re-wrapped by the assignment pattern (e.g. token=[[REDACTED]]).
	s = strings.ReplaceAll(s, "["+redactToken+"]", redactToken)
	return s
}

// Default is ON; only explicit false-like values disable it.
func redactionEnabled() bool {
	value, _ := gitwrap.OutputQuiet("config", "--get", "agit.redact")
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "false", "0", "off", "no", "disable", "disabled":
		return false
	default:
		return true
	}
}

func RedactMode() string {
	if redactionEnabled() {
		return "enabled"
	}
	return "disabled"
}

// redactAgentContext scrubs every free-text field of the context in place.
// SchemaVersion and CreatedAt are machine-generated and skipped.
func redactAgentContext(ctx *AgentContext) {
	ctx.Provider = RedactString(ctx.Provider)
	ctx.TaskID = RedactString(ctx.TaskID)
	ctx.Repo = RedactString(ctx.Repo)
	ctx.Branch = RedactString(ctx.Branch)
	ctx.BaseCommit = RedactString(ctx.BaseCommit)
	ctx.IntentSummary = RedactString(ctx.IntentSummary)
	ctx.CommitMessageSuggestion = RedactString(ctx.CommitMessageSuggestion)
	ctx.PlannedDelta = RedactString(ctx.PlannedDelta)
	ctx.ActualDelta = RedactString(ctx.ActualDelta)
	ctx.ActiveFiles = redactList(ctx.ActiveFiles)
	ctx.Decisions = redactList(ctx.Decisions)
	ctx.Risks = redactList(ctx.Risks)
	ctx.NextSteps = redactList(ctx.NextSteps)
}

func redactList(items []string) []string {
	for i, item := range items {
		items[i] = RedactString(item)
	}
	return items
}

// enforceSizeLimits applies rune-aware per-field caps, then the total
// payload cap. Reuses the repo's truncateLine so multi-byte runes are never
// split.
func enforceSizeLimits(ctx *AgentContext) {
	ctx.Provider = truncateLine(ctx.Provider, maxMetaRunes)
	ctx.TaskID = truncateLine(ctx.TaskID, maxMetaRunes)
	ctx.Repo = truncateLine(ctx.Repo, maxMetaRunes)
	ctx.Branch = truncateLine(ctx.Branch, maxMetaRunes)
	ctx.BaseCommit = truncateLine(ctx.BaseCommit, maxMetaRunes)
	ctx.IntentSummary = truncateLine(ctx.IntentSummary, maxIntentRunes)
	ctx.CommitMessageSuggestion = truncateLine(ctx.CommitMessageSuggestion, maxMessageRunes)
	ctx.PlannedDelta = truncateLine(ctx.PlannedDelta, maxDeltaRunes)
	ctx.ActualDelta = truncateLine(ctx.ActualDelta, maxDeltaRunes)

	ctx.ActiveFiles = truncateStringList(ctx.ActiveFiles)
	ctx.Decisions = truncateStringList(ctx.Decisions)
	ctx.Risks = truncateStringList(ctx.Risks)
	ctx.NextSteps = truncateStringList(ctx.NextSteps)

	enforceTotalCap(ctx)
}

// truncateStringList caps list length (repo truncateList style, with a
// "+N more" marker so truncation is visible) and per-item rune length.
func truncateStringList(items []string) []string {
	for i, item := range items {
		items[i] = truncateLine(item, maxListItemRunes)
	}
	return truncateList(items, maxListItems)
}

// enforceTotalCap shrinks the largest text fields until the marshaled
// payload fits. Bounded iterations; the record is never dropped.
func enforceTotalCap(ctx *AgentContext) {
	for i := 0; i < 20; i++ {
		raw, err := json.Marshal(ctx)
		if err != nil || len(raw) <= maxPayloadBytes {
			return
		}
		if !shrinkLargestTextField(ctx) {
			return
		}
	}
}

func shrinkLargestTextField(ctx *AgentContext) bool {
	fields := []*string{
		&ctx.PlannedDelta,
		&ctx.ActualDelta,
		&ctx.IntentSummary,
		&ctx.CommitMessageSuggestion,
	}
	for _, list := range []*[]string{&ctx.ActiveFiles, &ctx.Decisions, &ctx.Risks, &ctx.NextSteps} {
		for i := range *list {
			fields = append(fields, &(*list)[i])
		}
	}

	largest := -1
	largestLen := 0
	for i, f := range fields {
		if n := utf8.RuneCountInString(*f); n > minShrinkRunes && n > largestLen {
			largest, largestLen = i, n
		}
	}
	if largest == -1 {
		return false
	}

	*fields[largest] = truncateLine(*fields[largest], largestLen*3/4)
	return true
}

// SanitizeAgentContext applies redaction (unless disabled) and size limits.
// It is the single choke point called by SaveAgentContext before persisting,
// and the shared entry point for any future code that ships context off the
// machine (e.g. the team-sync service): sanitize before upload, not after.
func SanitizeAgentContext(ctx *AgentContext) {
	if redactionEnabled() {
		redactAgentContext(ctx)
	}
	enforceSizeLimits(ctx)
}
