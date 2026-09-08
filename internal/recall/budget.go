package recall

import (
	"encoding/json"
	"fmt"
	"sort"
	"unicode/utf8"
)

// Bytes are deterministic across model tokenizers. They are not token counts.
const maxToolResponseBytes = 12 * 1024
const maxToolTextBytes = 6 * 1024

func clipUTF8(text string, limit int) string {
	if limit <= 0 {
		return ""
	}
	if len(text) <= limit {
		return text
	}
	text = text[:limit]
	for len(text) > 0 && !utf8.ValidString(text) {
		text = text[:len(text)-1]
	}
	return text
}

func encode(payload any) (string, error) {
	out, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("recall: encode results: %w", err)
	}
	if len(out) <= maxToolResponseBytes {
		return string(out), nil
	}
	var bounded map[string]any
	if err := json.Unmarshal(out, &bounded); err != nil {
		return "", err
	}
	remaining := maxToolTextBytes
	trimRecallText(bounded, &remaining)
	bounded["output_truncated"] = true
	bounded["note"] = "Output is bounded. Use a result's session_id and match_message_id as around_message_id with a smaller window to read more."
	out, err = json.Marshal(bounded)
	if err != nil {
		return "", err
	}
	if len(out) > maxToolResponseBytes {
		return "", fmt.Errorf("recall output metadata exceeds the limit; reduce limit/window or use detail=brief")
	}
	return string(out), nil
}

func trimRecallText(value any, remaining *int) {
	switch item := value.(type) {
	case []any:
		// Preserve a queried anchor before spending the budget on surrounding text.
		for _, child := range item {
			if object, ok := child.(map[string]any); ok && object["anchor"] == true {
				trimRecallText(child, remaining)
			}
		}
		for _, child := range item {
			if object, ok := child.(map[string]any); ok && object["anchor"] == true {
				continue
			}
			trimRecallText(child, remaining)
		}
	case map[string]any:
		keys := make([]string, 0, len(item))
		for key := range item {
			keys = append(keys, key)
		}
		sort.Slice(keys, func(i, j int) bool {
			// Main matches/windows come before expendable bookends.
			a, b := recallKeyPriority(keys[i]), recallKeyPriority(keys[j])
			if a != b {
				return a < b
			}
			return keys[i] < keys[j]
		})
		for _, key := range keys {
			child := item[key]
			switch key {
			case "content", "snippet", "preview", "title", "query":
				text, ok := child.(string)
				if !ok {
					continue
				}
				limit := *remaining
				if key == "title" || key == "query" {
					if limit > 256 {
						limit = 256
					}
				}
				clipped := clipUTF8(text, limit)
				item[key] = clipped
				*remaining -= len(clipped)
				if key == "content" && len(clipped) < len(text) {
					item["content_truncated"] = true
					if _, exists := item["original_content_bytes"]; !exists {
						item["original_content_bytes"] = len(text)
					}
				}
			default:
				trimRecallText(child, remaining)
			}
		}
	}
}
func recallKeyPriority(key string) int {
	switch key {
	case "results", "messages", "content", "snippet":
		return 0
	case "bookend_start", "bookend_end":
		return 2
	default:
		return 1
	}
}
