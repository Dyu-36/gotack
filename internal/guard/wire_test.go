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
	out := Evaluate(ParseInput(payload), Options{})
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

// TestAllowRoundTrip checks that an auto-tier call yields the exact allow
// envelope Crush treats as a pre-approval, surviving re-decoding.
func TestAllowRoundTrip(t *testing.T) {
	payload := []byte(`{"event":"PreToolUse","session_id":"s","cwd":"/w","tool_name":"view","tool_input":{"file_path":"README.md"}}`)
	out := Evaluate(ParseInput(payload), Options{})
	if out.Decision != DecisionAllow {
		t.Fatalf("decision = %q, want allow for a read-only tool", out.Decision)
	}
	raw, err := MarshalOutput(out)
	if err != nil {
		t.Fatalf("MarshalOutput: %v", err)
	}
	var decoded struct {
		Version  int    `json:"version"`
		Decision string `json:"decision"`
		Halt     bool   `json:"halt"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("re-decode envelope: %v", err)
	}
	if decoded.Version != outputVersion || decoded.Decision != "allow" || decoded.Halt {
		t.Fatalf("decoded envelope = %+v, want version=%d decision=allow halt=false", decoded, outputVersion)
	}
}

// TestPassThroughEmitsNothing checks that an ask-tier call in an interactive
// session produces no bytes, which is how a hook expresses "no opinion" and
// lets Crush fall through to its permission relay.
func TestPassThroughEmitsNothing(t *testing.T) {
	payload := []byte(`{"event":"PreToolUse","session_id":"s","cwd":"/w","tool_name":"bash","tool_input":{"command":"go build ./..."}}`)
	out := Evaluate(ParseInput(payload), Options{})
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

// TestUnknownToolPassThrough pins that a tool the guard does not recognise is
// never auto-approved: interactive sessions fall through to the ask tier.
func TestUnknownToolPassThrough(t *testing.T) {
	payload := []byte(`{"event":"PreToolUse","session_id":"s","cwd":"/w","tool_name":"future_tool","tool_input":{}}`)
	out := Evaluate(ParseInput(payload), Options{})
	if !out.IsNone() {
		t.Fatalf("unknown tools must pass through interactively, got %+v", out)
	}
}
