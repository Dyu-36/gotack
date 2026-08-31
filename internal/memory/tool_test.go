package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Dyu-36/gotack/internal/mcp"
)

// TestToolValidationMatrix pins every accepted and rejected argument shape:
// unknown actions and targets are typed errors, empty content on add is
// rejected, and replace/remove on a missing section are explicit errors.
func TestToolValidationMatrix(t *testing.T) {
	ctx := context.Background()

	seed := func(t *testing.T, tool mcp.Tool) {
		t.Helper()
		if _, err := tool.Handler(ctx, json.RawMessage(`{"action":"add","section":"Facts","content":"known fact"}`)); err != nil {
			t.Fatalf("seed add: %v", err)
		}
	}

	tests := []struct {
		name    string
		args    string
		seeded  bool
		wantErr error // nil expects success
	}{
		{name: "view default target", args: `{"action":"view"}`},
		{name: "view user target", args: `{"action":"view","target":"user"}`},
		{name: "add creates section", args: `{"action":"add","section":"Facts","content":"the sky is blue"}`},
		{name: "add user target", args: `{"action":"add","target":"user","section":"Prefs","content":"concise"}`},
		{name: "add defaults target to memory", args: `{"action":"add","target":"","section":"Facts","content":"x"}`},
		{name: "replace existing section", args: `{"action":"replace","section":"Facts","content":"new"}`, seeded: true},
		{name: "remove existing section", args: `{"action":"remove","section":"Facts"}`, seeded: true},
		{name: "unknown action", args: `{"action":"destroy"}`, wantErr: ErrUnknownAction},
		{name: "empty action", args: `{"action":""}`, wantErr: ErrUnknownAction},
		{name: "unknown target", args: `{"action":"view","target":"project"}`, wantErr: ErrUnknownTarget},
		{name: "add without section", args: `{"action":"add","content":"x"}`, wantErr: ErrMissingSection},
		{name: "add with empty content", args: `{"action":"add","section":"Facts","content":""}`, wantErr: ErrEmptyContent},
		{name: "add with whitespace content", args: `{"action":"add","section":"Facts","content":"   "}`, wantErr: ErrEmptyContent},
		{name: "replace without section", args: `{"action":"replace","content":"x"}`, wantErr: ErrMissingSection},
		{name: "replace with empty content", args: `{"action":"replace","section":"Facts","content":""}`, wantErr: ErrEmptyContent},
		{name: "replace missing section", args: `{"action":"replace","section":"Ghost","content":"x"}`, wantErr: ErrSectionNotFound},
		{name: "remove without section", args: `{"action":"remove"}`, wantErr: ErrMissingSection},
		{name: "remove missing section", args: `{"action":"remove","section":"Ghost"}`, wantErr: ErrSectionNotFound},
		{name: "no arguments", args: "", wantErr: ErrArguments},
		{name: "invalid json", args: "{not json", wantErr: ErrArguments},
		{name: "wrong field type", args: `{"action":42}`, wantErr: ErrArguments},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tool := Tool(newTestStore(t, "sess-1"))
			if test.seeded {
				seed(t, tool)
			}
			out, err := tool.Handler(ctx, json.RawMessage(test.args))
			if test.wantErr == nil {
				if err != nil {
					t.Fatalf("expected success, got %v", err)
				}
				var payload Result
				if err := json.Unmarshal([]byte(out), &payload); err != nil {
					t.Fatalf("result is not a Result payload: %v\n%s", err, out)
				}
				if payload.Cap == 0 || payload.Remaining != payload.Cap-payload.Size {
					t.Fatalf("budget fields wrong: %+v", payload)
				}
				return
			}
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("err = %v, want errors.Is(%v)", err, test.wantErr)
			}
		})
	}
}

func TestToolNameAndSchema(t *testing.T) {
	tool := Tool(newTestStore(t, "sess-1"))
	if tool.Name != ToolName {
		t.Fatalf("tool name = %q, want %q", tool.Name, ToolName)
	}
	var schema map[string]any
	if err := json.Unmarshal(tool.Schema, &schema); err != nil {
		t.Fatalf("tool schema is not valid JSON: %v", err)
	}
}

// TestToolOverMCPServer drives the tool through the stdio server, the way
// Crush launches it after registration: initialize, tools/list, then calls.
func TestToolOverMCPServer(t *testing.T) {
	store := newTestStore(t, "sess-live")
	server := &mcp.Server{
		Name:    "gotack-memory",
		Version: "0.1.0",
		Tools:   []mcp.Tool{Tool(store)},
	}

	lines := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"memory","arguments":{"action":"add","section":"Facts","content":"the sky is blue"}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"memory","arguments":{"action":"view"}}}`,
		`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"memory","arguments":{"action":"destroy"}}}`,
	}, "\n") + "\n"

	var out bytes.Buffer
	if err := server.Serve(context.Background(), strings.NewReader(lines), &out); err != nil {
		t.Fatalf("serve: %v", err)
	}
	responses := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(responses) != 5 {
		t.Fatalf("got %d responses: %q", len(responses), out.String())
	}

	var listing struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(responses[1]), &listing); err != nil {
		t.Fatalf("decode tools/list: %v", err)
	}
	if len(listing.Result.Tools) != 1 || listing.Result.Tools[0].Name != ToolName {
		t.Fatalf("tools/list = %+v", listing)
	}

	decode := func(t *testing.T, line string) (text string, isError bool) {
		t.Helper()
		var call struct {
			Result struct {
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
				IsError bool `json:"isError"`
			} `json:"result"`
		}
		if err := json.Unmarshal([]byte(line), &call); err != nil {
			t.Fatalf("decode tools/call: %v", err)
		}
		if len(call.Result.Content) != 1 {
			t.Fatalf("tools/call content = %+v", call)
		}
		return call.Result.Content[0].Text, call.Result.IsError
	}

	addText, isError := decode(t, responses[2])
	if isError {
		t.Fatalf("add call errored: %s", addText)
	}
	var added Result
	if err := json.Unmarshal([]byte(addText), &added); err != nil {
		t.Fatalf("add result is not a Result payload: %v\n%s", err, addText)
	}
	if added.Evicted != 0 || added.Size == 0 {
		t.Fatalf("add result = %+v", added)
	}

	viewText, isError := decode(t, responses[3])
	if isError {
		t.Fatalf("view call errored: %s", viewText)
	}
	var viewed Result
	if err := json.Unmarshal([]byte(viewText), &viewed); err != nil {
		t.Fatalf("view result is not a Result payload: %v\n%s", err, viewText)
	}
	if !strings.Contains(viewed.Content, "the sky is blue") {
		t.Fatalf("memory did not round-trip through stdio: %s", viewed.Content)
	}
	// Provenance must survive the round trip with the configured session.
	stamped := false
	for _, section := range parseFile(viewed.Content).Sections {
		for _, entry := range section.Entries {
			if entry.Session == "sess-live" && entry.At != "" {
				stamped = true
			}
		}
	}
	if !stamped {
		t.Fatalf("viewed content lacks sess-live provenance: %s", viewed.Content)
	}

	errText, isError := decode(t, responses[4])
	if !isError || !strings.Contains(errText, "unknown action") {
		t.Fatalf("unknown action must surface as isError: %q (isError=%v)", errText, isError)
	}
}
