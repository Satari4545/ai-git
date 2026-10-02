package agit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/Satari4545/ai-git/internal/testutil"
)

func TestRedactStringPatterns(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		secret string
	}{
		{
			name:   "openai key",
			input:  "configured key sk-proj-abcdefghij1234567890ABCDEFGHIJ for the client",
			secret: "sk-proj-abcdefghij1234567890ABCDEFGHIJ",
		},
		{
			name:   "github pat",
			input:  "token is ghp_abcdefghijklmnopqrstuvwxyz1234567890",
			secret: "ghp_abcdefghijklmnopqrstuvwxyz1234567890",
		},
		{
			name:   "github fine-grained pat",
			input:  "using github_pat_abcdefghijklmnopqrstuvwxyz1234567890 in CI",
			secret: "github_pat_abcdefghijklmnopqrstuvwxyz1234567890",
		},
		{
			name:   "aws access key",
			input:  "deployed with AKIAIOSFODNN7EXAMPLE",
			secret: "AKIAIOSFODNN7EXAMPLE",
		},
		{
			name:   "stripe live key",
			input:  "stripe key sk_live_abcdefghijklmnopqrstuvwxyz12",
			secret: "sk_live_abcdefghijklmnopqrstuvwxyz12",
		},
		{
			name:   "slack bot token",
			input:  "notify via xoxb-123456789012-ABCDEFGHIJKL",
			secret: "xoxb-123456789012-ABCDEFGHIJKL",
		},
		{
			name:   "private key block",
			input:  "cert:\n-----BEGIN RSA PRIVATE KEY-----\nMIIBOgIBAAJBAKsecretbytes\n-----END RSA PRIVATE KEY-----\ndone",
			secret: "MIIBOgIBAAJBAKsecretbytes",
		},
		{
			name:   "password assignment",
			input:  "login with password=hunter2",
			secret: "hunter2",
		},
		{
			name:   "json token assignment",
			input:  `config: {"token": "abc123def456"}`,
			secret: "abc123def456",
		},
		{
			name:   "api key with colon",
			input:  "header api_key: xyz789",
			secret: "xyz789",
		},
		{
			name:   "env style token",
			input:  "export DB_TOKEN=s3cr3tvalue",
			secret: "s3cr3tvalue",
		},
		{
			name:   "url credentials",
			input:  "cloned https://yousif:hunter2@github.com/org/repo",
			secret: "hunter2@github.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := RedactString(tt.input)
			if !strings.Contains(out, redactToken) {
				t.Errorf("RedactString(%q) = %q, want it to contain %q", tt.input, out, redactToken)
			}
			if strings.Contains(out, tt.secret) {
				t.Errorf("RedactString(%q) = %q, still contains secret", tt.input, out)
			}
		})
	}
}

func TestRedactStringNoFalsePositives(t *testing.T) {
	inputs := []string{
		"tokenize the input stream",
		"my passport expires in 2030",
		"the secret sauce recipe",
		"a token bucket algorithm",
		"classified documents are here",
		"the password field was left blank",
		"api keyboard shortcuts",
	}

	for _, in := range inputs {
		if out := RedactString(in); out != in {
			t.Errorf("RedactString(%q) = %q, want unchanged", in, out)
		}
	}
}

func TestRedactionDefaultOn(t *testing.T) {
	testutil.InitRepo(t)
	if !redactionEnabled() {
		t.Error("redactionEnabled() = false with no config, want true (default ON)")
	}
	if got := RedactMode(); got != "enabled" {
		t.Errorf("RedactMode() = %q, want %q", got, "enabled")
	}
}

func TestAgitConfigRedactToggle(t *testing.T) {
	dir := testutil.InitRepo(t)

	if err := Config([]string{"redact", "off"}); err != nil {
		t.Fatalf("Config(redact off) = %v", err)
	}
	if redactionEnabled() {
		t.Error("redactionEnabled() = true after 'agit config redact off', want false")
	}

	if err := Config([]string{"redact", "on"}); err != nil {
		t.Fatalf("Config(redact on) = %v", err)
	}
	if !redactionEnabled() {
		t.Error("redactionEnabled() = false after 'agit config redact on', want true")
	}

	value := testutil.RunGit(t, dir, "config", "--get", "agit.redact")
	if strings.TrimSpace(value) != "true" {
		t.Errorf("git config agit.redact = %q, want %q", value, "true")
	}
}

func TestSaveAgentContextRedactsSecrets(t *testing.T) {
	testutil.InitRepo(t)

	secret := "ghp_abcdefghijklmnopqrstuvwxyz1234567890"
	ctx := &AgentContext{
		IntentSummary:           "rotate the deploy token",
		CommitMessageSuggestion: "rotate token",
		PlannedDelta:            "replace " + secret + " in CI config",
		ActualDelta:             "export DB_TOKEN=" + secret,
		Decisions:               []string{"use " + secret + " for now"},
		Risks:                   []string{"password=hunter2 in the old script"},
		NextSteps:               []string{"revoke " + secret},
		ActiveFiles:             []string{"deploy.sh"},
		Repo:                    "https://bot:" + secret + "@github.com/org/repo",
	}

	if err := SaveAgentContext(ctx); err != nil {
		t.Fatalf("SaveAgentContext = %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(".agit", "state", "current.json"))
	if err != nil {
		t.Fatalf("reading state file: %v", err)
	}
	content := string(raw)

	if !strings.Contains(content, redactToken) {
		t.Error("state file has no [REDACTED] marker, want secrets redacted")
	}
	for _, leaked := range []string{secret, "hunter2", "DB_TOKEN=" + secret} {
		if strings.Contains(content, leaked) {
			t.Errorf("state file leaks secret %q", leaked)
		}
	}
}

func TestRedactionCanBeDisabled(t *testing.T) {
	dir := testutil.InitRepo(t)
	testutil.RunGit(t, dir, "config", "agit.redact", "false")

	secret := "sk-proj-abcdefghij1234567890ABCDEFGHIJ"
	ctx := &AgentContext{
		IntentSummary: "test",
		PlannedDelta:  "key " + secret,
	}
	if err := SaveAgentContext(ctx); err != nil {
		t.Fatalf("SaveAgentContext = %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(".agit", "state", "current.json"))
	if err != nil {
		t.Fatalf("reading state file: %v", err)
	}
	if !strings.Contains(string(raw), secret) {
		t.Error("state file does not contain the secret with redaction disabled, want it preserved")
	}
}

func TestNotePayloadIsRedacted(t *testing.T) {
	testutil.InitRepo(t)

	secret := "xoxb-123456789012-ABCDEFGHIJKL"
	ctx := &AgentContext{
		IntentSummary: "fix alerts",
		PlannedDelta:  "notify via " + secret,
	}
	if err := SaveAgentContext(ctx); err != nil {
		t.Fatalf("SaveAgentContext = %v", err)
	}

	loaded, ok, err := LoadAgentContext()
	if err != nil || !ok {
		t.Fatalf("LoadAgentContext = %v, %v", loaded, err)
	}

	notePayload := map[string]any{"agentContext": loaded}
	raw, err := json.Marshal(notePayload)
	if err != nil {
		t.Fatalf("marshal note payload: %v", err)
	}

	if strings.Contains(string(raw), secret) {
		t.Error("note payload leaks secret, want it redacted before commit attaches it")
	}
	if !strings.Contains(string(raw), redactToken) {
		t.Error("note payload has no [REDACTED] marker")
	}
}

func TestRedactBeforeTruncate(t *testing.T) {
	testutil.InitRepo(t)

	ctx := &AgentContext{
		IntentSummary: strings.Repeat("a", 600) + " password=hunter2",
	}
	SanitizeAgentContext(ctx)

	if strings.Contains(ctx.IntentSummary, "hunter2") {
		t.Error("secret survived because truncation ran before redaction")
	}
	if utf8.RuneCountInString(ctx.IntentSummary) > maxIntentRunes {
		t.Errorf("intent has %d runes, want <= %d", utf8.RuneCountInString(ctx.IntentSummary), maxIntentRunes)
	}
}

func TestEnforceSizeLimitsTruncatesFields(t *testing.T) {
	ctx := &AgentContext{
		Provider:                strings.Repeat("p", 300),
		IntentSummary:           strings.Repeat("日", 600),
		CommitMessageSuggestion: strings.Repeat("m", 600),
		PlannedDelta:            strings.Repeat("d", 2500),
		ActualDelta:             strings.Repeat("🚀", 2500),
		Decisions:               make([]string, 25),
		Risks:                   []string{strings.Repeat("r", 300)},
	}
	for i := range ctx.Decisions {
		ctx.Decisions[i] = "decision"
	}

	enforceSizeLimits(ctx)

	checks := []struct {
		name  string
		value string
		max   int
	}{
		{"provider", ctx.Provider, maxMetaRunes},
		{"intent", ctx.IntentSummary, maxIntentRunes},
		{"message", ctx.CommitMessageSuggestion, maxMessageRunes},
		{"planned", ctx.PlannedDelta, maxDeltaRunes},
		{"actual", ctx.ActualDelta, maxDeltaRunes},
		{"risk item", ctx.Risks[0], maxListItemRunes},
	}
	for _, c := range checks {
		n := utf8.RuneCountInString(c.value)
		if n > c.max {
			t.Errorf("%s has %d runes, want <= %d", c.name, n, c.max)
		}
		if !utf8.ValidString(c.value) {
			t.Errorf("%s is not valid UTF-8 after truncation", c.name)
		}
		if !strings.HasSuffix(c.value, "...") {
			t.Errorf("%s = %q..., want truncation marker", c.name, c.value[:20])
		}
	}

	if len(ctx.Decisions) != maxListItems+1 {
		t.Errorf("decisions has %d items, want %d", len(ctx.Decisions), maxListItems+1)
	}
	if last := ctx.Decisions[len(ctx.Decisions)-1]; last != "+5 more" {
		t.Errorf("last decision item = %q, want %q", last, "+5 more")
	}
}

func TestEnforceSizeLimitsTotalCap(t *testing.T) {
	ctx := &AgentContext{
		IntentSummary: "keep me",
		PlannedDelta:  strings.Repeat("日", 20000),
		ActualDelta:   strings.Repeat("€", 20000),
		Decisions:     []string{strings.Repeat("🚀", 5000)},
		Risks:         []string{strings.Repeat("中", 5000)},
	}

	enforceSizeLimits(ctx)

	raw, err := json.Marshal(ctx)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if len(raw) > maxPayloadBytes {
		t.Errorf("payload is %d bytes, want <= %d", len(raw), maxPayloadBytes)
	}

	var decoded AgentContext
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}
	if decoded.IntentSummary != "keep me" {
		t.Errorf("intent = %q, want small fields preserved", decoded.IntentSummary)
	}
	if !utf8.ValidString(decoded.PlannedDelta) || !utf8.ValidString(decoded.ActualDelta) {
		t.Error("shrunk fields are not valid UTF-8")
	}
}

func TestSanitizeAgentContextEndToEnd(t *testing.T) {
	testutil.InitRepo(t)

	ctx := &AgentContext{
		IntentSummary: "deploy",
		PlannedDelta:  "use AKIAIOSFODNN7EXAMPLE for the deploy role",
		ActualDelta:   strings.Repeat("x", 5000),
	}
	SanitizeAgentContext(ctx)

	if strings.Contains(ctx.PlannedDelta, "AKIAIOSFODNN7EXAMPLE") {
		t.Error("AWS key survived sanitizeAgentContext")
	}
	if utf8.RuneCountInString(ctx.ActualDelta) > maxDeltaRunes {
		t.Error("delta not truncated by sanitizeAgentContext")
	}
}
