package recall

import (
	"encoding/json"
	"strings"

	"github.com/Dyu-36/gotack/internal/crushapi"
)

const maxMessageTextBytes = 64 * 1024

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

func truncateBytes(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && s[n]&0xC0 == 0x80 {
		n--
	}
	return s[:n]
}
