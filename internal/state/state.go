package state

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"time"
)

type LogicDelta struct {
	SchemaVersion string `json:"schemaVersion"`
	Kind          string `json:"kind"`

	MCP MCPContext `json:"mcp"`

	AgentThinking AgentThinking `json:"agentThinking"`
	CodeReality   CodeReality   `json:"codeReality"`

	CreatedAt string `json:"createdAt"`
}

type MCPContext struct {
	SystemPrompt string        `json:"systemPrompt"`
	Resources    []MCPResource `json:"resources"`
	Tools        []MCPTool     `json:"tools,omitempty"`
}

type MCPResource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	MimeType    string `json:"mimeType,omitempty"`
	Description string `json:"description,omitempty"`
}

type MCPTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"inputSchema,omitempty"`
}

type AgentThinking struct {
	IntentSummary string   `json:"intentSummary"`
	Assumptions   []string `json:"assumptions,omitempty"`
	OpenQuestions []string `json:"openQuestions,omitempty"`
	RiskFlags     []string `json:"riskFlags,omitempty"`
}

type CodeReality struct {
	ActiveFiles []string `json:"activeFiles"`
	GitStatus   string   `json:"gitStatus"`
	DiffStat    string   `json:"diffStat"`
}

func Capture(intent string) (*LogicDelta, string, []byte, error) {
	files, _ := gitLines("diff", "--name-only", "--cached")
	status, _ := gitText("status", "--short")
	diffstat, _ := gitText("diff", "--cached", "--stat")

	prompt := os.Getenv("AGIT_SYSTEM_PROMPT")
	if prompt == "" {
		prompt = "unknown"
	}

	if intent == "" {
		intent = "No intent summary provided. Set AGIT_INTENT or use --intent."
	}

	delta := &LogicDelta{
		SchemaVersion: "agit.logic-delta.v0",
		Kind:          "logic_delta",
		MCP: MCPContext{
			SystemPrompt: prompt,
			Resources:    mapFilesToResources(files),
		},
		AgentThinking: AgentThinking{
			IntentSummary: intent,
		},
		CodeReality: CodeReality{
			ActiveFiles: files,
			GitStatus:   status,
			DiffStat:    diffstat,
		},
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	raw, err := json.Marshal(delta)
	if err != nil {
		return nil, "", nil, err
	}

	sum := sha256.Sum256(raw)
	contextID := "ctx_sha256:" + hex.EncodeToString(sum[:])

	pretty, err := json.MarshalIndent(delta, "", "  ")
	return delta, contextID, pretty, err
}

func mapFilesToResources(files []string) []MCPResource {
	res := make([]MCPResource, 0, len(files))
	for _, f := range files {
		res = append(res, MCPResource{
			URI:      "file://" + f,
			Name:     f,
			MimeType: "text/plain",
		})
	}
	return res
}

func gitText(args ...string) (string, error) {
	out, err := exec.Command("git", args...).Output()
	return strings.TrimSpace(string(out)), err
}

func gitLines(args ...string) ([]string, error) {
	txt, err := gitText(args...)
	if txt == "" {
		return []string{}, err
	}
	return strings.Split(txt, "\n"), err
}
