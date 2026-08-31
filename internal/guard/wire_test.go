package guard

import (
	"encoding/json"
	"testing"
)

// wire_test.go -- role: prove the hook JSON round-trip against the exact wire
// shape Crush uses (hooks.Payload in, parseStdout envelope out).

func TestParseInputExtractsToolFields(t *testing.T) {
	payload := []byte(`{
		"event": "PreToolUse",
		"session_id": "sess-1",
		"cwd": "C:/work/project",
		"tool_name": "bash",
		"tool_input": {"command": "rm -rf /", "timeout": 30}
	}`)
	in := ParseInput(payload)
	if in.Event != "PreToolUse" || in.SessionID != "sess-1" || in.ToolName != "bash" {
		t.Fatalf("parsed envelope mismatch: %+v", in)
	}
	if got := in.Command(); got != "rm -rf /" {
		t.Fatalf("Command() = %q, want %q", got, "rm -rf /")
	}
	if got := in.FilePath(); got != "" {
		t.Fatalf("FilePath() = %q, want empty for a bash payload", got)
	}
}

func TestParseInputMalformedFailsOpen(t *testing.T) {
	for _, payload := range [][]byte{nil, {}, []byte("not-json"), []byte(`{"tool_input": "oops"}`)} {
		in := ParseInput(payload)
		if in.Command() != "" || in.FilePath() != "" {
			t.Fatalf("ParseInput(%s) should yield empty fields, got %+v", payload, in)
		}
	}
}

// TestDenyRoundTrip checks that a destructive payload yields the exact deny
// envelope Crush's parseStdout understands, and that it survives re-decoding.
func TestDenyRoundTrip(t *testing.T) {
	payload := []byte(`{"event":"PreToolUse","session_id":"s","cwd":"/w","tool_name":"bash","tool_input":{"command":"format C:"}}`)
	out := Evaluate(ParseInput(payload))
	if out.Decision != DecisionDeny {
		t.Fatalf("decision = %q, want deny", out.Decision)
	}
	if !out.Halt {
		t.Fatal("format C: is unrecoverable; halt must be true")
	}

	raw, err := MarshalOutput(out)
	if err != nil {
		t.Fatalf("MarshalOutput: %v", err)
	}
	var decoded struct {
		Version  int    `json:"version"`
		Decision string `json:"decision"`
		Halt     bool   `json:"halt"`
		Reason   string `json:"reason"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("re-decode envelope: %v", err)
	}
	if decoded.Version != outputVersion || decoded.Decision != "deny" || !decoded.Halt {
		t.Fatalf("decoded envelope = %+v, want version=%d decision=deny halt=true", decoded, outputVersion)
	}
	if decoded.Reason == "" {
		t.Fatal("deny envelope must carry a legible reason")
	}
}

// TestPassThroughEmitsNothing checks that a benign tool call produces no bytes,
// which is how a hook expresses "no opinion" and lets Crush fall through.
func TestPassThroughEmitsNothing(t *testing.T) {
	payload := []byte(`{"event":"PreToolUse","session_id":"s","cwd":"/w","tool_name":"bash","tool_input":{"command":"go build ./..."}}`)
	out := Evaluate(ParseInput(payload))
	if !out.IsNone() {
		t.Fatalf("benign command should pass through, got %+v", out)
	}
	raw, err := MarshalOutput(out)
	if err != nil {
		t.Fatalf("MarshalOutput: %v", err)
	}
	if len(raw) != 0 {
		t.Fatalf("pass-through must emit no bytes, got %s", raw)
	}
}

// TestNonCommandToolsPassThrough pins stage-1 behaviour for tools that carry no
// command field (file reads, edits): the blocklist does not apply and the call
// is left untouched.
func TestNonCommandToolsPassThrough(t *testing.T) {
	payload := []byte(`{"event":"PreToolUse","session_id":"s","cwd":"/w","tool_name":"write","tool_input":{"file_path":"/etc/hosts","content":"x"}}`)
	out := Evaluate(ParseInput(payload))
	if !out.IsNone() {
		t.Fatalf("stage-1 guard must pass non-command tools through, got %+v", out)
	}
}
