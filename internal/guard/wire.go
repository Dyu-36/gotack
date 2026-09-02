package guard

import (
	"encoding/json"
)

const (
	DecisionAllow = "allow"
	DecisionDeny  = "deny"
	DecisionNone  = ""
)

const outputVersion = 1

type Input struct {
	Event     string          `json:"event"`
	SessionID string          `json:"session_id"`
	CWD       string          `json:"cwd"`
	ToolName  string          `json:"tool_name"`
	ToolInput json.RawMessage `json:"tool_input"`
}

type toolInputFields struct {
	Command  string `json:"command"`
	FilePath string `json:"file_path"`
}

func (in Input) Command() string {
	return in.fields().Command
}

func (in Input) FilePath() string {
	return in.fields().FilePath
}

func (in Input) fields() toolInputFields {
	var f toolInputFields
	if len(in.ToolInput) == 0 {
		return f
	}

	_ = json.Unmarshal(in.ToolInput, &f)
	return f
}

func ParseInput(data []byte) Input {
	var in Input
	if len(data) == 0 {
		return in
	}
	_ = json.Unmarshal(data, &in)
	return in
}

type Output struct {
	Version      int             `json:"version,omitempty"`
	Decision     string          `json:"decision,omitempty"`
	Halt         bool            `json:"halt,omitempty"`
	Reason       string          `json:"reason,omitempty"`
	Context      string          `json:"context,omitempty"`
	UpdatedInput json.RawMessage `json:"updated_input,omitempty"`
}

func (o Output) IsNone() bool {
	return o.Decision == DecisionNone && len(o.UpdatedInput) == 0
}

func None() Output { return Output{} }

func Deny(reason string, halt bool) Output {
	return Output{Version: outputVersion, Decision: DecisionDeny, Halt: halt, Reason: reason}
}

func Allow() Output {
	return Output{Version: outputVersion, Decision: DecisionAllow}
}

func MarshalOutput(o Output) ([]byte, error) {
	if o.IsNone() {
		return nil, nil
	}
	return json.Marshal(o)
}
