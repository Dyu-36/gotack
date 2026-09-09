package session

import (
	"strings"
	"unicode"

	"github.com/Dyu-36/gotack/internal/attachments"
	"github.com/Dyu-36/gotack/internal/engineapi"
)

func isDefaultTitle(title string) bool {
	switch strings.ToLower(strings.TrimSpace(title)) {
	case "", "new session", "new conversation", "new chat", "untitled session", "hội thoại mới":
		return true
	default:
		return false
	}
}

func titleFromMessages(messages []engineapi.Message) string {
	for _, message := range messages {
		if message.Role != "user" {
			continue
		}
		parts := engineapi.ExtractParts(message.Parts)
		text, refs := attachments.ParseAttachmentBlocks(parts.Text)
		if text == "" {
			var names []string
			for _, ref := range refs {
				names = append(names, attachments.BaseName(ref.FileName))
			}
			for _, attachment := range parts.Attachments {
				names = append(names, attachments.BaseName(attachment.FileName))
			}
			text = strings.Join(names, ", ")
		}
		text = strings.Map(func(r rune) rune {
			if unicode.IsControl(r) {
				return ' '
			}
			return r
		}, text)
		title := []rune(strings.Join(strings.Fields(text), " "))
		if len(title) == 0 {
			continue
		}
		if len(title) > 80 {
			return strings.TrimSpace(string(title[:79])) + "…"
		}
		return string(title)
	}
	return ""
}
