package recall

import (
	"encoding/json"
	"strings"
)

// maxMessageTextBytes caps the searchable text extracted from one message.
// Tool outputs can carry megabytes; indexing them whole would bloat recall.db
// without improving recall quality.
const maxMessageTextBytes = 64 * 1024

// partEnvelope mirrors the on-disk shape written by Crush's marshalParts:
// an array of {"type": ..., "data": {...}} objects. Only Type is decoded
// eagerly; Data stays raw so an unknown or reshaped part can be skipped
// without failing the whole message.
type partEnvelope struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// extractPartsText returns the human-readable text contained in a Crush
// messages.parts JSON column. It is deliberately defensive: malformed JSON,
// unknown part types and missing fields all yield partial text or an empty
// string, never an error, because ingestion must survive schema drift.
func extractPartsText(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "[]" {
		return ""
	}
	var envelopes []partEnvelope
	if err := json.Unmarshal([]byte(trimmed), &envelopes); err != nil {
		return ""
	}
	var sb strings.Builder
	for _, envelope := range envelopes {
		appendPartText(&sb, envelope.Type, envelope.Data)
		if sb.Len() >= maxMessageTextBytes {
			break
		}
	}
	return strings.TrimSpace(truncateBytes(sb.String(), maxMessageTextBytes))
}

// appendPartText writes the readable fields of one known part type into sb.
// Unknown types are skipped: a future Crush part type must degrade to "not
// indexed", not to a failed ingestion.
func appendPartText(sb *strings.Builder, partType string, data json.RawMessage) {
	switch partType {
	case "text":
		var part struct {
			Text string `json:"text"`
		}
		if json.Unmarshal(data, &part) == nil {
			appendField(sb, part.Text)
		}
	case "reasoning":
		var part struct {
			Thinking string `json:"thinking"`
		}
		if json.Unmarshal(data, &part) == nil {
			appendField(sb, part.Thinking)
		}
	case "tool_call":
		var part struct {
			Name  string `json:"name"`
			Input string `json:"input"`
		}
		if json.Unmarshal(data, &part) == nil {
			appendField(sb, part.Name)
			appendField(sb, part.Input)
		}
	case "tool_result":
		var part struct {
			Name    string `json:"name"`
			Content string `json:"content"`
		}
		if json.Unmarshal(data, &part) == nil {
			appendField(sb, part.Name)
			appendField(sb, part.Content)
		}
	case "shell_command":
		var part struct {
			Command string `json:"command"`
			Output  string `json:"output"`
		}
		if json.Unmarshal(data, &part) == nil {
			appendField(sb, part.Command)
			appendField(sb, part.Output)
		}
	case "finish":
		var part struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(data, &part) == nil {
			appendField(sb, part.Message)
		}
	}
}

// appendField adds one non-empty field separated by a newline so the FTS
// tokenizer still sees distinct terms.
func appendField(sb *strings.Builder, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	if sb.Len() > 0 {
		sb.WriteByte('\n')
	}
	sb.WriteString(value)
}

// truncateBytes cuts s to at most n bytes without splitting a UTF-8 rune,
// so the FTS tokenizer never sees invalid UTF-8.
func truncateBytes(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && s[n]&0xC0 == 0x80 {
		n--
	}
	return s[:n]
}
