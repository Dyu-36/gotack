package crushapi

import (
	"encoding/json"
	"strings"
)

type Endpoint struct {
	Network string
	Address string
}

type VersionInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildID   string `json:"build_id"`
	GoVersion string `json:"go_version"`
	Platform  string `json:"platform"`
}

type Workspace struct {
	ID       string `json:"id"`
	Path     string `json:"path"`
	YOLO     bool   `json:"yolo,omitempty"`
	Debug    bool   `json:"debug,omitempty"`
	DataDir  string `json:"data_dir,omitempty"`
	Version  string `json:"version,omitempty"`
	ClientID string `json:"client_id,omitempty"`
}

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

type Todo struct {
	Content    string `json:"content"`
	Status     string `json:"status"`
	ActiveForm string `json:"active_form"`
}

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

type Attachment struct {
	FilePath string `json:"file_path"`
	FileName string `json:"file_name"`
	MimeType string `json:"mime_type"`
	Content  []byte `json:"content"`
}

type TextPart struct {
	Text string `json:"text"`
}

type ToolCall struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Input    json.RawMessage `json:"input"`
	Finished bool            `json:"finished,omitempty"`
}

type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Name       string `json:"name"`
	Content    string `json:"content"`
	Metadata   string `json:"metadata"`
	IsError    bool   `json:"is_error"`
}

type BinaryPart struct {
	Path     string `json:"Path"`
	MIMEType string `json:"MIMEType"`
	Data     []byte `json:"Data"`
}

type File struct {
	ID        string `json:"id"`
	SessionID string `json:"session_id"`
	Path      string `json:"path"`
	Content   string `json:"content"`
	Version   int64  `json:"version"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

type PermissionAction string

const (
	PermissionAllow           PermissionAction = "allow"
	PermissionAllowForSession PermissionAction = "allow_session"
	PermissionDeny            PermissionAction = "deny"
)

func (p PermissionAction) MarshalText() ([]byte, error) {
	return []byte(p), nil
}

func (p *PermissionAction) UnmarshalText(text []byte) error {
	*p = PermissionAction(text)
	return nil
}

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

type PermissionGrant struct {
	Permission PermissionRequest `json:"permission"`
	Action     PermissionAction  `json:"action"`
}

type RunComplete struct {
	SessionID string        `json:"session_id"`
	RunID     string        `json:"run_id,omitempty"`
	MessageID string        `json:"message_id"`
	Text      string        `json:"text,omitempty"`
	Error     string        `json:"error,omitempty"`
	Cancelled bool          `json:"cancelled,omitempty"`
	Telemetry *RunTelemetry `json:"telemetry,omitempty"`
}

type RunTelemetry struct {
	RunID               string           `json:"run_id,omitempty"`
	Provider            string           `json:"provider,omitempty"`
	Model               string           `json:"model,omitempty"`
	ReasoningEffort     string           `json:"reasoning_effort,omitempty"`
	Attempt             int              `json:"attempt"`
	RetryCount          int              `json:"retry_count"`
	RetryDelayMicros    int64            `json:"retry_delay_us,omitempty"`
	SpansMicros         map[string]int64 `json:"spans_us,omitempty"`
	TotalMicros         int64            `json:"total_us"`
	FirstSemantic       string           `json:"first_semantic,omitempty"`
	CacheStatus         string           `json:"cache_status"`
	CachedInputTokens   *int64           `json:"cached_input_tokens,omitempty"`
	UncachedInputTokens *int64           `json:"uncached_input_tokens,omitempty"`
	ServiceTier         string           `json:"service_tier,omitempty"`
	ProviderRequestID   string           `json:"provider_request_id,omitempty"`
	EstimatedUsage      bool             `json:"estimated_usage,omitempty"`
	Compacted           bool             `json:"compacted,omitempty"`
	PrefixChangedReason string           `json:"prefix_changed_reason,omitempty"`
	ChangeReasons       []string         `json:"change_reasons,omitempty"`
	StablePrefixHMAC    string           `json:"stable_prefix_hmac,omitempty"`
	StablePrefixBytes   int              `json:"stable_prefix_bytes,omitempty"`
	DynamicSuffixHMAC   string           `json:"dynamic_suffix_hmac,omitempty"`
	DynamicSuffixBytes  int              `json:"dynamic_suffix_bytes,omitempty"`
	RequestShapeHMAC    string           `json:"request_shape_hmac,omitempty"`
	RequestShapeBytes   int              `json:"request_shape_bytes,omitempty"`
}

type TaskProgress struct {
	SessionID                string `json:"session_id"`
	RunID                    string `json:"run_id,omitempty"`
	State                    string `json:"state"`
	ElapsedSeconds           int    `json:"elapsed_seconds"`
	LimitSeconds             int    `json:"limit_seconds"`
	Solutions                int    `json:"solutions,omitempty"`
	Penalty                  *int   `json:"penalty,omitempty"`
	ResultStatus             string `json:"result_status,omitempty"`
	HardConstraintsSatisfied *bool  `json:"hard_constraints_satisfied,omitempty"`
	SoftViolationCount       int    `json:"soft_violation_count,omitempty"`
}

type partWrapper struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type Parts struct {
	Text        string
	ToolCalls   []ToolCall
	ToolResults []ToolResult
	Attachments []Attachment
}

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
		case "tool_result":
			var tr ToolResult
			if err := json.Unmarshal(w.Data, &tr); err != nil {
				continue
			}
			out.ToolResults = append(out.ToolResults, tr)
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

func ExtractText(parts json.RawMessage) string { return ExtractParts(parts).Text }

func ExtractToolCalls(parts json.RawMessage) []ToolCall { return ExtractParts(parts).ToolCalls }

func ExtractAttachments(parts json.RawMessage) []Attachment { return ExtractParts(parts).Attachments }
