package recall

import (
	"encoding/json"
	"strings"

	"github.com/Dyu-36/gotack/internal/crushapi"
)

// maxMessageTextBytes caps the searchable text extracted from one message.
// Tool outputs can carry megabytes; indexing them whole would bloat recall.db
// without improving recall quality.
const maxMessageTextBytes = 64 * 1024

// extractPartsText returns the searchable fields from one Crush message.
// Crush keeps reasoning and tool results in the same parts blob, while
// Hermes' default discovery indexes ordinary user/assistant content and only
// searches tool output when the caller explicitly asks for role=tool.
func extractPartsText(raw, role string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "[]" {
		return ""
	}
	parts := crushapi.ExtractParts(json.RawMessage(trimmed))
	var sb strings.Builder
	appendField(&sb, parts.Text)
	if strings.EqualFold(strings.TrimSpace(role), "assistant") {
		for _, call := range parts.ToolCalls {
			appendField(&sb, call.Name)
			appendField(&sb, string(call.Input))
		}
	}
	if strings.EqualFold(strings.TrimSpace(role), "tool") {
		for _, result := range parts.ToolResults {
			appendField(&sb, result.Name)
			appendField(&sb, result.Content)
		}
	}
	return strings.TrimSpace(truncateBytes(sb.String(), maxMessageTextBytes))
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
