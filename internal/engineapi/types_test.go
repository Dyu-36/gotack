package engineapi

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestExtractPartsText(t *testing.T) {
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
			got := ExtractParts(json.RawMessage(tt.parts)).Text
			if got != tt.want {
				t.Fatalf("ExtractParts(%q).Text = %q, want %q", tt.parts, got, tt.want)
			}
		})
	}
}

func TestExtractPartsAttachments(t *testing.T) {
	parts := json.RawMessage(`[
		{"type":"text","data":{"text":"review this"}},
		{"type":"binary","data":{"Path":"photo.png","MIMEType":"image/png","Data":"iVBORw=="}}
	]`)

	got := ExtractParts(parts).Attachments
	if len(got) != 1 {
		t.Fatalf("ExtractParts().Attachments len = %d, want 1", len(got))
	}
	if got[0].FileName != "photo.png" || got[0].MimeType != "image/png" {
		t.Fatalf("ExtractParts().Attachments metadata = %#v", got[0])
	}
	if string(got[0].Content) != "\x89PNG" {
		t.Fatalf("ExtractParts().Attachments content = %q, want PNG header", got[0].Content)
	}
}

func TestExtractPartsToolCalls(t *testing.T) {
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
			got := ExtractParts(json.RawMessage(tt.parts)).ToolCalls
			if !equalToolCalls(got, tt.want) {
				t.Fatalf("ExtractParts(%q).ToolCalls = %+v, want %+v", tt.parts, got, tt.want)
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

func equalToolCalls(a, b []ToolCall) bool {
	return reflect.DeepEqual(a, b)
}
