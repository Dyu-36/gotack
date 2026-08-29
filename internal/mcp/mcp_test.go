package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func serveLine(t *testing.T, server *Server, line string) map[string]any {
	t.Helper()
	var out bytes.Buffer
	if err := server.Serve(context.Background(), strings.NewReader(line+"\n"), &out); err != nil {
		t.Fatalf("serve: %v", err)
	}
	if out.Len() == 0 {
		t.Fatalf("no response for %s", line)
	}
	var response map[string]any
	if err := json.Unmarshal(out.Bytes(), &response); err != nil {
		t.Fatalf("decode response %q: %v", out.String(), err)
	}
	return response
}

func testServer() *Server {
	return &Server{
		Name:    "test",
		Version: "0.0.1",
		Tools: []Tool{{
			Name:        "echo",
			Description: "echo the text",
			Schema:      json.RawMessage(`{"type":"object","properties":{"text":{"type":"string"}},"required":["text"]}`),
			Handler: func(ctx context.Context, args json.RawMessage) (string, error) {
				var req struct {
					Text string `json:"text"`
				}
				if err := json.Unmarshal(args, &req); err != nil {
					return "", err
				}
				return "echo:" + req.Text, nil
			},
		}},
	}
}

func TestInitialize(t *testing.T) {
	response := serveLine(t, testServer(), `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)
	result, _ := response["result"].(map[string]any)
	if result == nil {
		t.Fatalf("missing result: %v", response)
	}
	if result["protocolVersion"] != protocolVersion {
		t.Fatalf("protocol version %v", result["protocolVersion"])
	}
}

func TestToolsListAndCall(t *testing.T) {
	server := testServer()
	listing := serveLine(t, server, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	result := listing["result"].(map[string]any)
	tools := result["tools"].([]any)
	if len(tools) != 1 || tools[0].(map[string]any)["name"] != "echo" {
		t.Fatalf("unexpected tools %v", tools)
	}

	call := serveLine(t, server, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"echo","arguments":{"text":"hi"}}}`)
	callResult := call["result"].(map[string]any)
	content := callResult["content"].([]any)[0].(map[string]any)
	if content["text"] != "echo:hi" {
		t.Fatalf("tool result %v", callResult)
	}
}

func TestToolErrorBecomesIsErrorResult(t *testing.T) {
	server := testServer()
	call := serveLine(t, server, `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"echo","arguments":{"text":123}}}`)
	result := call["result"].(map[string]any)
	if result["isError"] != true {
		t.Fatalf("expected isError result: %v", call)
	}
}

func TestUnknownToolAndMethod(t *testing.T) {
	server := testServer()
	response := serveLine(t, server, `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"nope","arguments":{}}}`)
	errObj := response["error"].(map[string]any)
	if errObj["code"] != float64(-32602) {
		t.Fatalf("unknown tool error %v", errObj)
	}

	response = serveLine(t, server, `{"jsonrpc":"2.0","id":6,"method":"bogus"}`)
	errObj = response["error"].(map[string]any)
	if errObj["code"] != float64(-32601) {
		t.Fatalf("unknown method error %v", errObj)
	}
}

func TestNotificationsAreSilent(t *testing.T) {
	var out bytes.Buffer
	server := testServer()
	if err := server.Serve(context.Background(), strings.NewReader("{\"jsonrpc\":\"2.0\",\"method\":\"notifications/initialized\"}\n"), &out); err != nil {
		t.Fatalf("serve: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("notifications must not answer, got %q", out.String())
	}
}
