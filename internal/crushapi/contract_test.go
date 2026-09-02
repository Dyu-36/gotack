// contract_test.go -- role: focused unit tests for the wire-shape helpers.
//
// Crush's Message.Parts is a wrapped JSON array; the bridge uses
// ExtractParts to pull typed values out. These tests
// pin the wire format we accept so future server changes are caught
// here before they reach the UI.
package crushapi

import (
	"encoding/json"
	"testing"
)

func TestExtractText(t *testing.T) {
	tests := []struct {
		name  string
		parts string
		want  string
	}{
		{
			name:  "empty",
			parts: "",
			want:  "",
		},
		{
			name:  "null",
			parts: "null",
			want:  "",
		},
		{
			name:  "invalid JSON",
			parts: "{not json",
			want:  "",
		},
		{
			name:  "single text part",
			parts: `[{"type":"text","data":{"text":"hello"}}]`,
			want:  "hello",
		},
		{
			name: "multiple text parts joined by newline",
			parts: `[
				{"type":"text","data":{"text":"first"}},
				{"type":"text","data":{"text":"second"}},
				{"type":"text","data":{"text":"third"}}
			]`,
			want: "first\nsecond\nthird",
		},
		{
			name: "text and reasoning: only text counts",
			parts: `[
				{"type":"reasoning","data":{"thinking":"..."}},
				{"type":"text","data":{"text":"answer"}}
			]`,
			want: "answer",
		},
		{
			name: "tool_call parts ignored",
			parts: `[
				{"type":"tool_call","data":{"id":"t1","name":"read","input":{}}},
				{"type":"text","data":{"text":"done"}}
			]`,
			want: "done",
		},
		{
			name: "empty text part is skipped",
			parts: `[
				{"type":"text","data":{"text":""}},
				{"type":"text","data":{"text":"only"}}
			]`,
			want: "only",
		},
		{
			name: "unrecognized type ignored",
			parts: `[
				{"type":"image_url","data":{"url":"x"}},
				{"type":"text","data":{"text":"kept"}}
			]`,
			want: "kept",
		},
		{
			name: "data field not matching the type is skipped",
			parts: `[
				{"type":"text","data":{"thinking":"oops"}}
			]`,
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractText(json.RawMessage(tt.parts))
			if got != tt.want {
				t.Fatalf("ExtractText(%q) = %q, want %q", tt.parts, got, tt.want)
			}
		})
	}
}

func TestExtractAttachments(t *testing.T) {
	parts := json.RawMessage(`[
		{"type":"text","data":{"text":"review this"}},
		{"type":"binary","data":{"Path":"photo.png","MIMEType":"image/png","Data":"iVBORw=="}}
	]`)

	got := ExtractAttachments(parts)
	if len(got) != 1 {
		t.Fatalf("ExtractAttachments() len = %d, want 1", len(got))
	}
	if got[0].FileName != "photo.png" || got[0].MimeType != "image/png" {
		t.Fatalf("ExtractAttachments() metadata = %#v", got[0])
	}
	if string(got[0].Content) != "\x89PNG" {
		t.Fatalf("ExtractAttachments() content = %q, want PNG header", got[0].Content)
	}
}

func TestExtractToolCalls(t *testing.T) {
	tests := []struct {
		name  string
		parts string
		want  []ToolCall
	}{
		{
			name:  "empty",
			parts: "",
			want:  nil,
		},
		{
			name:  "null",
			parts: "null",
			want:  nil,
		},
		{
			name:  "invalid JSON",
			parts: "{not json",
			want:  nil,
		},
		{
			name:  "no tool calls",
			parts: `[{"type":"text","data":{"text":"hi"}}]`,
			want:  nil,
		},

		{
			name: "single tool call",
			parts: `[
				{"type":"tool_call","data":{"id":"t1","name":"read","input":{"path":"/a"},"finished":true}}
			]`,
			want: []ToolCall{
				{ID: "t1", Name: "read", Input: json.RawMessage(`{"path":"/a"}`), Finished: true},
			},
		},
		{
			name: "multiple tool calls preserved in order",
			parts: `[
				{"type":"tool_call","data":{"id":"t1","name":"read","input":{}}},
				{"type":"text","data":{"text":"between"}},
				{"type":"tool_call","data":{"id":"t2","name":"write","input":{"x":1},"finished":false}}
			]`,
			want: []ToolCall{
				{ID: "t1", Name: "read", Input: json.RawMessage(`{}`), Finished: false},
				{ID: "t2", Name: "write", Input: json.RawMessage(`{"x":1}`), Finished: false},
			},
		},
		{
			name: "malformed tool call data is skipped",
			parts: `[
				{"type":"tool_call","data":"not-an-object"},
				{"type":"tool_call","data":{"id":"ok","name":"f","input":{}}}
			]`,
			want: []ToolCall{
				{ID: "ok", Name: "f", Input: json.RawMessage(`{}`), Finished: false},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractToolCalls(json.RawMessage(tt.parts))
			if !equalToolCalls(got, tt.want) {
				t.Fatalf("ExtractToolCalls(%q) = %+v, want %+v", tt.parts, got, tt.want)
			}
		})
	}
}

func TestExtractToolResults(t *testing.T) {
	parts := json.RawMessage(`[
		{"type":"tool_result","data":{"tool_call_id":"t1","name":"read","content":"file body","data":"ignored-base64","metadata":"{}","is_error":false}},
		{"type":"tool_result","data":{"tool_call_id":"t2","name":"exec","content":"exit 1","is_error":true}}
	]`)

	got := ExtractParts(parts).ToolResults
	want := []ToolResult{
		{ToolCallID: "t1", Name: "read", Content: "file body", Metadata: "{}"},
		{ToolCallID: "t2", Name: "exec", Content: "exit 1", IsError: true},
	}
	if len(got) != len(want) {
		t.Fatalf("ToolResults len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ToolResults[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// equalToolCalls compares two slices of ToolCall semantically. RawMessage
// fields are compared as JSON bytes via jsonEqual so a "{"x":1}" matches
// the same value regardless of whitespace.
func equalToolCalls(a, b []ToolCall) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ID != b[i].ID || a[i].Name != b[i].Name || a[i].Finished != b[i].Finished {
			return false
		}
		if !jsonEqual(a[i].Input, b[i].Input) {
			return false
		}
	}
	return true
}

func jsonEqual(a, b json.RawMessage) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	var av, bv any
	if err := json.Unmarshal(a, &av); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &bv); err != nil {
		return false
	}
	ra, _ := json.Marshal(av)
	rb, _ := json.Marshal(bv)
	return string(ra) == string(rb)
}
