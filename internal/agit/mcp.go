package agit

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

type MCPRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type MCPResponse struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id,omitempty"`
	Result  any       `json:"result,omitempty"`
	Error   *MCPError `json:"error,omitempty"`
}

type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func MCPServe() error {
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		line := scanner.Bytes()

		var req MCPRequest
		if err := json.Unmarshal(line, &req); err != nil {
			writeMCPError(nil, -32700, "parse error")
			continue
		}

		switch req.Method {
		case "initialize":
			writeMCPResult(req.ID, map[string]any{
				"protocolVersion": "2025-06-18",
				"capabilities": map[string]any{
					"tools": map[string]any{},
				},
				"serverInfo": map[string]any{
					"name":    "agit",
					"version": "0.1.0",
				},
			})

		case "notifications/initialized":
			// Notification: no response.

		case "tools/list":
			writeMCPResult(req.ID, map[string]any{
				"tools": []map[string]any{
					{
						"name":        "record_context",
						"description": "Record the active AI coding agent context for the current Git repository.",
						"inputSchema": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"provider": map[string]any{
									"type":        "string",
									"description": "Agent provider, e.g. codex, claude, cursor.",
								},
								"intent": map[string]any{
									"type":        "string",
									"description": "Short summary of what the agent is trying to do.",
								},
								"message": map[string]any{
									"type":        "string",
									"description": "Suggested Git commit message.",
								},
								"files": map[string]any{
									"type":        "string",
									"description": "Comma-separated active file list.",
								},
								"planned": map[string]any{
									"type":        "string",
									"description": "Planned logic delta.",
								},
								"actual": map[string]any{
									"type":        "string",
									"description": "Actual code delta.",
								},
							},
							"required": []string{"intent"},
						},
					},
					{
						"name":        "get_context",
						"description": "Read AGIT context restored from the currently checked-out commit.",
						"inputSchema": map[string]any{
							"type":       "object",
							"properties": map[string]any{},
						},
					},
				},
			})

		case "tools/call":
			handleToolCall(req)

		default:
			writeMCPError(req.ID, -32601, "method not found")
		}
	}

	return scanner.Err()
}

func handleToolCall(req MCPRequest) {
	var params struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}

	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeMCPError(req.ID, -32602, "invalid params")
		return
	}

	switch params.Name {
	case "record_context":
		handleRecordContext(req.ID, params.Arguments)
	case "get_context":
		handleGetContext(req.ID)
	default:
		writeMCPError(req.ID, -32601, "unknown tool")
	}
}

func handleRecordContext(id any, arguments map[string]any) {
	args := []string{}

	addArg := func(flag string, value any) {
		if value == nil {
			return
		}
		s, ok := value.(string)
		if !ok || s == "" {
			return
		}
		args = append(args, flag, s)
	}

	addArg("--provider", arguments["provider"])
	addArg("--intent", arguments["intent"])
	addArg("--message", arguments["message"])
	addArg("--files", arguments["files"])
	addArg("--planned", arguments["planned"])
	addArg("--actual", arguments["actual"])

	os.Setenv("AGIT_MCP_SILENT", "1")
	defer os.Unsetenv("AGIT_MCP_SILENT")

	if err := AgentWrite(args); err != nil {
		writeMCPError(id, -32000, err.Error())
		return
	}

	writeMCPResult(id, map[string]any{
		"content": []map[string]any{
			{
				"type": "text",
				"text": "agit recorded agent context successfully.",
			},
		},
	})
}

func handleGetContext(id any) {
	raw, ok, err := LoadCheckoutContextRaw()
	if err != nil {
		writeMCPError(id, -32000, err.Error())
		return
	}

	if !ok {
		writeMCPResult(id, map[string]any{
			"content": []map[string]any{
				{
					"type": "text",
					"text": "No checkout context has been restored. Run `agit checkout <branch>` on a commit with AGIT notes.",
				},
			},
		})
		return
	}

	writeMCPResult(id, map[string]any{
		"content": []map[string]any{
			{
				"type": "text",
				"text": string(raw),
			},
		},
	})
}

func writeMCPResult(id any, result any) {
	resp := MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}

	writeJSON(resp)
}

func writeMCPError(id any, code int, message string) {
	resp := MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &MCPError{
			Code:    code,
			Message: message,
		},
	}

	writeJSON(resp)
}

func writeJSON(v any) {
	raw, _ := json.Marshal(v)
	fmt.Println(string(raw))
}
