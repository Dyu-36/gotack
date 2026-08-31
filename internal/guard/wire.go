package guard

import (
	"encoding/json"
)

// wire.go -- role: the exact wire shapes Crush uses for the PreToolUse hook.
//
// The input mirrors hooks.Payload in the vendored engine: Crush pipes a JSON
// object to the hook's stdin. The output mirrors the envelope the engine's
// parseStdout understands (version, decision, halt, reason, context). The wire
// shapes are pinned against the Crush commit recorded in .crush-pin; see
// docs/contracts/gotack-approvals.md.

// Decision values a PreToolUse hook may return. An empty decision means the
// hook expressed no opinion and Crush falls through to its own permission
// system (the "ask" path when prompts are enabled).
const (
	DecisionDeny = "deny"
	DecisionNone = ""
)

// outputVersion is the envelope version this guard emits. Crush treats an
// omitted or version-1 envelope as current.
const outputVersion = 1

// Input is the JSON structure piped to the hook command's stdin.
type Input struct {
	Event     string          `json:"event"`
	SessionID string          `json:"session_id"`
	CWD       string          `json:"cwd"`
	ToolName  string          `json:"tool_name"`
	ToolInput json.RawMessage `json:"tool_input"`
}

// toolInputFields is the subset of tool_input the policy inspects. Crush sets
// command for the bash tool and file_path for the file tools; both are also
// surfaced as CRUSH_TOOL_INPUT_COMMAND / CRUSH_TOOL_INPUT_FILE_PATH.
type toolInputFields struct {
	Command  string `json:"command"`
	FilePath string `json:"file_path"`
}

// Command returns the requested shell command, empty when the tool carries no
// command field.
func (in Input) Command() string {
	return in.fields().Command
}

// FilePath returns the requested file path, empty when the tool carries no
// file_path field.
func (in Input) FilePath() string {
	return in.fields().FilePath
}

func (in Input) fields() toolInputFields {
	var f toolInputFields
	if len(in.ToolInput) == 0 {
		return f
	}
	// Malformed tool input is treated as empty rather than an error: the hook
	// must never fail closed on a shape it does not recognise, because a crash
	// here would surface as a non-blocking hook error and let the call through.
	_ = json.Unmarshal(in.ToolInput, &f)
	return f
}

// ParseInput decodes a hook stdin payload. A malformed payload yields an empty
// Input (not an error) for the same fail-open reason as fields.
func ParseInput(data []byte) Input {
	var in Input
	if len(data) == 0 {
		return in
	}
	_ = json.Unmarshal(data, &in)
	return in
}

// Output is the hook's decision envelope written to stdout.
type Output struct {
	Version  int    `json:"version,omitempty"`
	Decision string `json:"decision,omitempty"`
	Halt     bool   `json:"halt,omitempty"`
	Reason   string `json:"reason,omitempty"`
	Context  string `json:"context,omitempty"`
}

// IsNone reports whether the hook expressed no opinion (pass-through).
func (o Output) IsNone() bool { return o.Decision == DecisionNone }

// None is the pass-through result: the hook stays silent and Crush applies its
// own permission system unchanged.
func None() Output { return Output{} }

// Deny builds a deny decision carrying a legible reason. halt additionally
// stops the whole turn and is reserved for the genuinely unrecoverable rules.
func Deny(reason string, halt bool) Output {
	return Output{Version: outputVersion, Decision: DecisionDeny, Halt: halt, Reason: reason}
}

// MarshalOutput serialises the decision for stdout. A pass-through decision
// produces no bytes: writing nothing is the unambiguous way to express "no
// opinion" to the engine.
func MarshalOutput(o Output) ([]byte, error) {
	if o.IsNone() {
		return nil, nil
	}
	return json.Marshal(o)
}
