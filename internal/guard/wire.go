package guard

import (
	"encoding/json"
	"errors"
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
	f, _ := in.fields()
	return f.Command
}

func (in Input) FilePath() string {
	f, _ := in.fields()
	return f.FilePath
}

func (in Input) ToolInputErr() error {
	_, err := in.fields()
	return err
}

func (in Input) fields() (toolInputFields, error) {
	var f toolInputFields
	if len(in.ToolInput) == 0 {
		return f, nil
	}

	if err := json.Unmarshal(in.ToolInput, &f); err != nil {
		return f, err
	}
	return f, nil
}

func ParseInput(data []byte) (Input, error) {
	var in Input
	if len(data) == 0 {
		return in, errors.New("empty hook payload")
	}
	if err := json.Unmarshal(data, &in); err != nil {
		return in, err
	}
	return in, nil
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
