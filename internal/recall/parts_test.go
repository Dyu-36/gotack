package recall

import (
	"encoding/json"
	"strings"
	"testing"
)

func partFixture(t *testing.T, partType string, data any) string {
	t.Helper()
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal part data: %v", err)
	}
	envelope, err := json.Marshal(map[string]json.RawMessage{
		"type": mustJSON(t, partType),
		"data": raw,
	})
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	return string(envelope)
}

func mustJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal %v: %v", value, err)
	}
	return raw
}

func partsArray(parts ...string) string {
	return "[" + strings.Join(parts, ",") + "]"
}

func TestExtractPartsText(t *testing.T) {
	cases := []struct {
		name  string
		build func(t *testing.T) string
		want  []string
	}{
		{
			name: "text part",
			build: func(t *testing.T) string {
				return partsArray(partFixture(t, "text", map[string]string{"text": "fix the deploy pipeline"}))
			},
			want: []string{"fix the deploy pipeline"},
		},
		{
			name: "reasoning is not indexed",
			build: func(t *testing.T) string {
				return partsArray(
					partFixture(t, "reasoning", map[string]string{"thinking": "consider the rollback"}),
					partFixture(t, "text", map[string]string{"text": "rolled back the release"}),
				)
			},
			want: []string{"rolled back the release"},
		},
		{
			name: "assistant tool call fields",
			build: func(t *testing.T) string {
				return partsArray(
					partFixture(t, "tool_call", map[string]string{"name": "exec", "input": `{"command":"kubectl get pods"}`}),
					partFixture(t, "tool_result", map[string]string{"name": "exec", "content": "three pods running"}),
				)
			},
			want: []string{"exec", "kubectl get pods"},
		},
		{
			name: "unsupported machine parts are skipped",
			build: func(t *testing.T) string {
				return partsArray(
					partFixture(t, "shell_command", map[string]any{"command": "pnpm build", "output": "built ok", "exit_code": 0}),
					partFixture(t, "finish", map[string]any{"reason": "error", "message": "provider rate limited"}),
				)
			},
			want: nil,
		},
		{
			name: "unknown part type is skipped not fatal",
			build: func(t *testing.T) string {
				return partsArray(
					partFixture(t, "future_part", map[string]string{"whatever": "x"}),
					partFixture(t, "text", map[string]string{"text": "still indexed"}),
				)
			},
			want: []string{"still indexed"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			role := "user"
			if tc.name == "assistant tool call fields" || tc.name == "reasoning is not indexed" {
				role = "assistant"
			}
			got := extractPartsText(tc.build(t), role)
			offset := 0
			for _, want := range tc.want {
				idx := strings.Index(got[offset:], want)
				if idx < 0 {
					t.Fatalf("extractPartsText() = %q, missing %q", got, want)
				}
				offset += idx + len(want)
			}
		})
	}
}

func TestExtractPartsTextDefensive(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"empty string", ""},
		{"empty array", "[]"},
		{"malformed json", `[{"type":"text","data":`},
		{"object instead of array", `{"type":"text"}`},
		{"null", "null"},
		{"part data wrong shape", `[{"type":"text","data":"not an object"}]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := extractPartsText(tc.raw, "user"); got != "" {
				t.Fatalf("extractPartsText(%q) = %q, want empty", tc.raw, got)
			}
		})
	}
}

func TestExtractPartsTextTruncates(t *testing.T) {
	long := strings.Repeat("a", maxMessageTextBytes+4096)
	raw := partsArray(partFixture(t, "text", map[string]string{"text": long}))
	got := extractPartsText(raw, "user")
	if len(got) > maxMessageTextBytes {
		t.Fatalf("extracted %d bytes, cap is %d", len(got), maxMessageTextBytes)
	}
}

func TestExtractPartsTextIndexesToolResultsOnlyWhenRoleIsTool(t *testing.T) {
	raw := partsArray(partFixture(t, "tool_result", map[string]string{
		"name": "exec", "content": "pods healthy",
	}))
	if got := extractPartsText(raw, "assistant"); got != "" {
		t.Fatalf("assistant extraction exposed tool result: %q", got)
	}
	if got := extractPartsText(raw, "tool"); !strings.Contains(got, "pods healthy") {
		t.Fatalf("tool extraction omitted result: %q", got)
	}
}

func TestTruncateBytesKeepsRunes(t *testing.T) {
	input := strings.Repeat("ệ", 10)
	got := truncateBytes(input, 7)
	if len(got) > 6 {
		t.Fatalf("truncate split a rune: %q (%d bytes)", got, len(got))
	}
	for _, r := range got {
		if r == '\uFFFD' {
			t.Fatalf("truncate produced replacement char: %q", got)
		}
	}
}
