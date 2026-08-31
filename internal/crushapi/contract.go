// contract.go -- role: data transfer types mirroring the Crush proto payloads.
//
// Source of truth: third_party/crush/internal/server/proto.go and the spec in
// third_party/crush/internal/swagger. Both sides change together.
package crushapi

import (
	"encoding/json"
	"strings"
)

// Endpoint identifies a Crush server reachable on a named pipe (windows) or
// unix socket. Network is "npipe" or "unix"; Address is the pipe path or
// socket path. Cross-platform code only uses these two strings.
type Endpoint struct {
	Network string
	Address string
}

// VersionInfo mirrors proto.VersionInfo from the Crush server.
type VersionInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildID   string `json:"build_id"`
	GoVersion string `json:"go_version"`
	Platform  string `json:"platform"`
}

// Workspace mirrors proto.Workspace. Config, Env, Channels, Skills and
// Version are intentionally omitted: the bridge does not need them and
// decoding them requires importing upstream types.
type Workspace struct {
	ID       string `json:"id"`
	Path     string `json:"path"`
	YOLO     bool   `json:"yolo,omitempty"`
	Debug    bool   `json:"debug,omitempty"`
	DataDir  string `json:"data_dir,omitempty"`
	Version  string `json:"version,omitempty"`
	ClientID string `json:"client_id,omitempty"`
}

// Session mirrors proto.Session. IsBusy and AttachedClients are computed on
// the server and surface in JSON; the bridge treats them as opaque fields.
type Session struct {
	ID               string  `json:"id"`
	ParentSessionID  string  `json:"parent_session_id"`
	Title            string  `json:"title"`
	MessageCount     int64   `json:"message_count"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	SummaryMessageID string  `json:"summary_message_id"`
	Cost             float64 `json:"cost"`
	Todos            []Todo  `json:"todos,omitempty"`
	CreatedAt        int64   `json:"created_at"`
	UpdatedAt        int64   `json:"updated_at"`
	IsBusy           bool    `json:"is_busy"`
	AttachedClients  int     `json:"attached_clients"`
}

// Todo mirrors proto.Todo.
type Todo struct {
	Content    string `json:"content"`
	Status     string `json:"status"`
	ActiveForm string `json:"active_form"`
}

// Message mirrors proto.Message. Parts is the wrapped JSON array
// `{"type":"text|reasoning|tool_call|...","data":{...}}` so the bridge keeps
// it as json.RawMessage and uses ExtractText / ExtractToolCalls to drill in.
type Message struct {
	ID        string          `json:"id"`
	Role      string          `json:"role"`
	SessionID string          `json:"session_id"`
	Parts     json.RawMessage `json:"parts"`
	Model     string          `json:"model"`
	Provider  string          `json:"provider"`
	CreatedAt int64           `json:"created_at"`
	UpdatedAt int64           `json:"updated_at"`
}

// Attachment mirrors proto.Attachment for prompts sent to the agent.
// Content is encoded as base64 by encoding/json, matching Crush's []byte field.
type Attachment struct {
	FilePath string `json:"file_path"`
	FileName string `json:"file_name"`
	MimeType string `json:"mime_type"`
	Content  []byte `json:"content"`
}

// TextPart is the unwrapped data of a "text" content part.
type TextPart struct {
	Text string `json:"text"`
}

// ReasoningPart is the unwrapped data of a "reasoning" content part.
type ReasoningPart struct {
	Thinking string `json:"thinking"`
}

// ToolCall is the unwrapped data of a "tool_call" content part. Input is the
// raw JSON the model emitted; callers that need typed access can decode it.
type ToolCall struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Input    json.RawMessage `json:"input"`
	Finished bool            `json:"finished,omitempty"`
}

// BinaryPart is the unwrapped data of a "binary" content part in message
// history. Crush's proto type has no JSON tags, so its field names are
// capitalized on the wire.
type BinaryPart struct {
	Path     string `json:"Path"`
	MIMEType string `json:"MIMEType"`
	Data     []byte `json:"Data"`
}

// File mirrors proto.File.
type File struct {
	ID        string `json:"id"`
	SessionID string `json:"session_id"`
	Path      string `json:"path"`
	Content   string `json:"content"`
	Version   int64  `json:"version"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

// PermissionAction mirrors proto.PermissionAction.
type PermissionAction string

// Permission action constants match upstream values exactly; the server
// round-trips them as strings.
const (
	PermissionAllow           PermissionAction = "allow"
	PermissionAllowForSession PermissionAction = "allow_session"
	PermissionDeny            PermissionAction = "deny"
)

// MarshalText implements encoding.TextMarshaler.
func (p PermissionAction) MarshalText() ([]byte, error) {
	return []byte(p), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (p *PermissionAction) UnmarshalText(text []byte) error {
	*p = PermissionAction(text)
	return nil
}

// PermissionRequest mirrors proto.PermissionRequest. Params is raw JSON
// because the server decodes it server-side based on ToolName.
type PermissionRequest struct {
	ID          string          `json:"id"`
	SessionID   string          `json:"session_id"`
	ToolCallID  string          `json:"tool_call_id"`
	ToolName    string          `json:"tool_name"`
	Description string          `json:"description"`
	Action      string          `json:"action"`
	Params      json.RawMessage `json:"params"`
	Path        string          `json:"path"`
}

// PermissionGrant is the request body for the permissions/grant endpoint.
type PermissionGrant struct {
	Permission PermissionRequest `json:"permission"`
	Action     PermissionAction  `json:"action"`
}

// QuestionRequest mirrors proto.QuestionRequest as delivered over SSE.
type QuestionRequest struct {
	ID                 string         `json:"id"`
	SessionID          string         `json:"session_id"`
	ToolCallID         string         `json:"tool_call_id"`
	Questions          []QuestionItem `json:"questions"`
	ConfirmTitle       string         `json:"confirm_title,omitempty"`
	ConfirmDescription string         `json:"confirm_description,omitempty"`
}

// QuestionItem is one question inside a batch.
type QuestionItem struct {
	ID          string           `json:"id"`
	Type        string           `json:"type"`
	Label       string           `json:"label,omitempty"`
	Question    string           `json:"question"`
	Description string           `json:"description,omitempty"`
	Choices     []QuestionChoice `json:"choices,omitempty"`
}

// QuestionChoice is a selectable option.
type QuestionChoice struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

// QuestionAnswer is the request body for the questions/answer endpoint.
type QuestionAnswer struct {
	BatchRequestID string             `json:"batch_request_id"`
	Responses      []QuestionResponse `json:"responses"`
}

// QuestionResponse is one answer inside a batch response.
type QuestionResponse struct {
	QuestionID  string            `json:"request_id"`
	SelectedIDs []string          `json:"selected_ids,omitempty"`
	FillInText  string            `json:"fill_in_text,omitempty"`
	Yes         *bool             `json:"yes,omitempty"`
	Notes       map[string]string `json:"notes,omitempty"`
}

// RunComplete mirrors proto.RunComplete.
type RunComplete struct {
	SessionID string `json:"session_id"`
	RunID     string `json:"run_id,omitempty"`
	MessageID string `json:"message_id"`
	Text      string `json:"text,omitempty"`
	Error     string `json:"error,omitempty"`
	Cancelled bool   `json:"cancelled,omitempty"`
}

// AgentEvent mirrors the minimal subset of proto.AgentEvent that the bridge
// forwards. The full proto.AgentEvent carries a Message and a Go error
// which the UI does not need here.
type AgentEvent struct {
	Type      string `json:"type"`
	SessionID string `json:"session_id,omitempty"`
	Progress  string `json:"progress,omitempty"`
	Done      bool   `json:"done,omitempty"`
}

// partWrapper is the on-the-wire shape of a single entry in Message.Parts.
// The Crush server marshals ContentPart values into
//
//	{"type": "text|reasoning|tool_call|...", "data": {...}}
//
// so the bridge decodes the same shape to reach TextPart, ReasoningPart and
// ToolCall.
type partWrapper struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// Parts is the UI-relevant content of a Message.Parts blob: concatenated text,
// tool calls, and binary attachments. It exists so the SSE forwarder can
// decode the blob once per event instead of repeatedly, which matters because
// every token delta re-sends the whole parts array.
type Parts struct {
	Text        string
	ToolCalls   []ToolCall
	Attachments []Attachment
}

// ExtractParts decodes a Message.Parts blob in a single pass. Empty or invalid
// input returns the zero value; unknown part types and parts whose data does
// not match their declared type are skipped. Text parts are joined with "\n";
// ToolCall.Input is left as json.RawMessage so callers can decode it with
// knowledge of the tool's argument schema.
func ExtractParts(parts json.RawMessage) Parts {
	if len(parts) == 0 {
		return Parts{}
	}
	var ws []partWrapper
	if err := json.Unmarshal(parts, &ws); err != nil {
		return Parts{}
	}
	var out Parts
	var b strings.Builder
	for _, w := range ws {
		switch w.Type {
		case "text":
			var t TextPart
			if err := json.Unmarshal(w.Data, &t); err != nil || t.Text == "" {
				continue
			}
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(t.Text)
		case "tool_call":
			var tc ToolCall
			if err := json.Unmarshal(w.Data, &tc); err != nil {
				continue
			}
			out.ToolCalls = append(out.ToolCalls, tc)
		case "binary":
			var binary BinaryPart
			if err := json.Unmarshal(w.Data, &binary); err != nil {
				continue
			}
			out.Attachments = append(out.Attachments, Attachment{
				FilePath: binary.Path,
				FileName: binary.Path,
				MimeType: binary.MIMEType,
				Content:  binary.Data,
			})
		}
	}
	out.Text = b.String()
	return out
}

// ExtractText concatenates all text parts of a Message.Parts blob with "\n".
func ExtractText(parts json.RawMessage) string { return ExtractParts(parts).Text }

// ExtractToolCalls returns the tool_call parts of a Message.Parts blob.
func ExtractToolCalls(parts json.RawMessage) []ToolCall { return ExtractParts(parts).ToolCalls }

// ExtractAttachments returns the binary parts of a Message.Parts blob.
func ExtractAttachments(parts json.RawMessage) []Attachment { return ExtractParts(parts).Attachments }
