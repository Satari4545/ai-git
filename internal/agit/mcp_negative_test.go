package agit

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Satari4545/ai-git/internal/testutil"
)

type mcpWireResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func runMCPExchange(t *testing.T, input string) []mcpWireResponse {
	t.Helper()

	done := make(chan struct{})
	var serveErr error
	var out bytes.Buffer
	go func() {
		defer close(done)
		serveErr = serveMCP(strings.NewReader(input), &out)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("serveMCP did not terminate: possible hang on input")
	}
	if serveErr != nil {
		t.Fatalf("serveMCP returned error: %v", serveErr)
	}

	raw := bytes.TrimSpace(out.Bytes())
	if len(raw) == 0 {
		return nil
	}
	var responses []mcpWireResponse
	for _, line := range bytes.Split(raw, []byte("\n")) {
		var resp mcpWireResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			t.Fatalf("server emitted non-JSON line %q: %v", line, err)
		}
		if resp.JSONRPC != "2.0" {
			t.Errorf("line %q: want jsonrpc 2.0, got %q", line, resp.JSONRPC)
		}
		if resp.Result == nil && resp.Error == nil {
			t.Errorf("line %q: response has neither result nor error", line)
		}
		if resp.Result != nil && resp.Error != nil {
			t.Errorf("line %q: response has both result and error", line)
		}
		responses = append(responses, resp)
	}
	return responses
}

func mcpErrorCode(t *testing.T, resp mcpWireResponse) int {
	t.Helper()
	if resp.Error == nil {
		t.Fatalf("want error response, got result %s", resp.Result)
	}
	return resp.Error.Code
}

func TestMCPMalformedInput(t *testing.T) {
	testutil.InitRepo(t)

	responses := runMCPExchange(t,
		"this is not json\n"+
			"{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/\n"+
			"[1,2,3]\n"+
			"\"bare string\"\n"+
			"42\n"+
			"\n",
	)

	if len(responses) != 6 {
		t.Fatalf("want 6 parse-error responses, got %d", len(responses))
	}
	for i, resp := range responses {
		if code := mcpErrorCode(t, resp); code != -32700 {
			t.Errorf("input %d: want parse error -32700, got %d", i, code)
		}
		if resp.ID != nil {
			t.Errorf("input %d: parse error must carry null id, got %v", i, resp.ID)
		}
	}
}

func TestMCPServerContinuesAfterErrors(t *testing.T) {
	testutil.InitRepo(t)

	responses := runMCPExchange(t,
		"garbage\n"+
			"{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"initialize\"}\n"+
			"{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"nope\"}\n"+
			"{\"jsonrpc\":\"2.0\",\"id\":3,\"method\":\"tools/list\"}\n",
	)

	if len(responses) != 4 {
		t.Fatalf("want 4 responses, got %d", len(responses))
	}
	if code := mcpErrorCode(t, responses[0]); code != -32700 {
		t.Errorf("garbage: want -32700, got %d", code)
	}
	if responses[1].Error != nil {
		t.Errorf("initialize after garbage: want result, got error %v", responses[1].Error)
	}
	if code := mcpErrorCode(t, responses[2]); code != -32601 {
		t.Errorf("unknown method: want -32601, got %d", code)
	}
	if responses[3].Error != nil {
		t.Errorf("tools/list after errors: want result, got error %v", responses[3].Error)
	}
}

func TestMCPNotificationsGetNoResponse(t *testing.T) {
	testutil.InitRepo(t)

	responses := runMCPExchange(t,
		"{\"jsonrpc\":\"2.0\",\"method\":\"notifications/initialized\"}\n"+
			"{\"jsonrpc\":\"2.0\",\"method\":\"notifications/unknown\"}\n"+
			"{\"jsonrpc\":\"2.0\",\"method\":\"tools/list\"}\n",
	)

	if len(responses) != 0 {
		t.Fatalf("notifications must not be answered, got %d responses", len(responses))
	}
}

func TestMCPUnknownTool(t *testing.T) {
	testutil.InitRepo(t)

	responses := runMCPExchange(t,
		"{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\",\"params\":{\"name\":\"nope\",\"arguments\":{}}}\n",
	)

	if len(responses) != 1 {
		t.Fatalf("want 1 response, got %d", len(responses))
	}
	if code := mcpErrorCode(t, responses[0]); code != -32601 {
		t.Errorf("unknown tool: want -32601, got %d", code)
	}
}

func TestMCPToolCallBadParams(t *testing.T) {
	testutil.InitRepo(t)

	responses := runMCPExchange(t,
		"{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\",\"params\":[1,2]}\n"+
			"{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/call\"}\n"+
			"{\"jsonrpc\":\"2.0\",\"id\":3,\"method\":\"tools/call\",\"params\":null}\n"+
			"{\"jsonrpc\":\"2.0\",\"id\":4,\"method\":\"tools/call\",\"params\":{\"arguments\":{}}}\n",
	)

	if len(responses) != 4 {
		t.Fatalf("want 4 responses, got %d", len(responses))
	}
	for i, resp := range responses {
		if code := mcpErrorCode(t, resp); code != -32602 && code != -32601 {
			t.Errorf("case %d: want invalid params -32602 or unknown tool -32601, got %d", i, code)
		}
	}
}

func TestMCPGetHistoryBadArgs(t *testing.T) {
	dir := testutil.InitRepo(t)
	commitEmpty(t, dir, "seed")

	responses := runMCPExchange(t,
		"{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\",\"params\":{\"name\":\"get_history\",\"arguments\":{\"n\":\"ten\"}}}\n"+
			"{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/call\",\"params\":{\"name\":\"get_history\",\"arguments\":{\"n\":-3}}}\n"+
			"{\"jsonrpc\":\"2.0\",\"id\":3,\"method\":\"tools/call\",\"params\":{\"name\":\"get_history\",\"arguments\":{\"n\":1000000}}}\n"+
			"{\"jsonrpc\":\"2.0\",\"id\":4,\"method\":\"tools/call\",\"params\":{\"name\":\"get_history\",\"arguments\":{\"branch\":\"no-such-branch\"}}}\n",
	)

	if len(responses) != 4 {
		t.Fatalf("want 4 responses, got %d", len(responses))
	}
	if code := mcpErrorCode(t, responses[0]); code != -32602 {
		t.Errorf("string n: want -32602, got %d", code)
	}
	if code := mcpErrorCode(t, responses[1]); code != -32000 {
		t.Errorf("negative n: want -32000, got %d", code)
	}
	if responses[2].Error != nil {
		t.Errorf("huge n: want capped result, got error %v", responses[2].Error)
	}
	if code := mcpErrorCode(t, responses[3]); code != -32000 {
		t.Errorf("bad branch: want -32000, got %d", code)
	}
}

func TestMCPRecordContextMissingIntent(t *testing.T) {
	testutil.InitRepo(t)

	responses := runMCPExchange(t,
		"{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\",\"params\":{\"name\":\"record_context\",\"arguments\":{\"provider\":\"codex\"}}}\n"+
			"{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/call\",\"params\":{\"name\":\"record_context\",\"arguments\":{\"intent\":42}}}\n",
	)

	if len(responses) != 2 {
		t.Fatalf("want 2 responses, got %d", len(responses))
	}
	for i, resp := range responses {
		if code := mcpErrorCode(t, resp); code != -32000 {
			t.Errorf("case %d: want -32000, got %d", i, code)
		}
		if !strings.Contains(resp.Error.Message, "intent") {
			t.Errorf("case %d: error should mention intent, got %q", i, resp.Error.Message)
		}
	}
}

func TestMCPGetContextFreshRepo(t *testing.T) {
	testutil.InitRepo(t)

	responses := runMCPExchange(t,
		"{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\",\"params\":{\"name\":\"get_context\",\"arguments\":{}}}\n"+
			"{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/call\",\"params\":{\"name\":\"get_git_state\",\"arguments\":{}}}\n"+
			"{\"jsonrpc\":\"2.0\",\"id\":0,\"method\":\"tools/list\"}\n",
	)

	if len(responses) != 3 {
		t.Fatalf("want 3 responses, got %d", len(responses))
	}
	for i, resp := range responses {
		if resp.Error != nil {
			t.Errorf("case %d: want result, got error %v", i, resp.Error)
		}
	}
	if responses[2].ID != float64(0) {
		t.Errorf("zero id must round-trip, got %v", responses[2].ID)
	}
	var getContext struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(responses[0].Result, &getContext); err != nil {
		t.Fatalf("get_context result: %v", err)
	}
	if len(getContext.Content) != 1 || !strings.Contains(getContext.Content[0].Text, "No checkout context") {
		t.Errorf("get_context on fresh repo: unexpected text %q", getContext.Content[0].Text)
	}
}

func TestMCPLongLineSurvives(t *testing.T) {
	testutil.InitRepo(t)

	padding := strings.Repeat("x", 300*1024)
	line := "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"initialize\",\"params\":{\"note\":\"" + padding + "\"}}\n"

	responses := runMCPExchange(t, line)

	if len(responses) != 1 {
		t.Fatalf("want 1 response to 300KB line, got %d", len(responses))
	}
	if responses[0].Error != nil {
		t.Errorf("300KB line: want result, got error %v", responses[0].Error)
	}
}
