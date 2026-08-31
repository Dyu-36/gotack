package recall

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Dyu-36/gotack/internal/mcp"
)

func TestToolHandlerReturnsCitedResults(t *testing.T) {
	dataDir := standardFixture(t, t.TempDir())
	store := newTestStore(t, dataDir)
	tool := Tool(store)

	if tool.Name != ToolName {
		t.Fatalf("tool name = %q, want %q", tool.Name, ToolName)
	}
	var schema map[string]any
	if err := json.Unmarshal(tool.Schema, &schema); err != nil {
		t.Fatalf("tool schema is not valid JSON: %v", err)
	}

	text, err := tool.Handler(context.Background(), json.RawMessage(`{"query":"kubernetes","limit":5}`))
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	var payload struct {
		Count   int      `json:"count"`
		Results []Result `json:"results"`
	}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf("handler output is not JSON: %v\n%s", err, text)
	}
	if payload.Count != 2 || len(payload.Results) != 2 {
		t.Fatalf("payload = %+v", payload)
	}
	if payload.Results[0].SessionID == "" || payload.Results[0].CreatedAt == "" {
		t.Fatalf("result lacks citation fields: %+v", payload.Results[0])
	}
}

func TestToolHandlerArgumentErrors(t *testing.T) {
	dataDir := standardFixture(t, t.TempDir())
	store := newTestStore(t, dataDir)
	tool := Tool(store)

	cases := []struct {
		name string
		args string
	}{
		{"no arguments", ""},
		{"invalid json", "{not json"},
		{"wrong type", `{"query":42}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := tool.Handler(context.Background(), json.RawMessage(tc.args)); err == nil {
				t.Fatal("expected an argument error")
			}
		})
	}
}

// TestToolOverMCPServer drives the tool through the stdio server, the way
// Crush launches it after registration.
func TestToolOverMCPServer(t *testing.T) {
	dataDir := standardFixture(t, t.TempDir())
	store := newTestStore(t, dataDir)
	server := &mcp.Server{
		Name:    "gotack-recall",
		Version: "0.1.0",
		Tools:   []mcp.Tool{Tool(store)},
	}

	lines := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"session_search","arguments":{"query":"kubernetes"}}}`,
	}, "\n") + "\n"

	var out bytes.Buffer
	if err := server.Serve(context.Background(), strings.NewReader(lines), &out); err != nil {
		t.Fatalf("serve: %v", err)
	}
	responses := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(responses) != 3 {
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

	var call struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(responses[2]), &call); err != nil {
		t.Fatalf("decode tools/call: %v", err)
	}
	if call.Result.IsError || len(call.Result.Content) != 1 {
		t.Fatalf("tools/call = %+v", call)
	}
	if !strings.Contains(call.Result.Content[0].Text, `"session_id": "sess-deploy"`) {
		t.Fatalf("tool result lacks session citation: %s", call.Result.Content[0].Text)
	}
}

// TestToolSurfacesSchemaMismatch proves a drifted engine database becomes an
// isError tool result rather than an empty success.
func TestToolSurfacesSchemaMismatch(t *testing.T) {
	dir := t.TempDir()
	createFixture(t, dir, func(db *sql.DB) {
		seedSession(t, db, "s", "S", 1)
		seedMessage(t, db, "m", "s", "user", "[]", 1, 1)
		if _, err := db.Exec("ALTER TABLE messages DROP COLUMN parts"); err != nil {
			t.Fatalf("drop parts: %v", err)
		}
	})
	store := newTestStore(t, dir)
	tool := Tool(store)

	_, err := tool.Handler(context.Background(), json.RawMessage(`{"query":"anything"}`))
	if err == nil {
		t.Fatal("schema mismatch must fail the tool call")
	}
	if !strings.Contains(err.Error(), "recall:") {
		t.Fatalf("error is not typed: %v", err)
	}
}
