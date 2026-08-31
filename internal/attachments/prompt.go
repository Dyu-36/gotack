package attachments

import (
	"fmt"
	"strings"

	"github.com/Dyu-36/gotack/internal/crushapi"
)

// ComposePrompt builds the effective prompt text for an agent turn.
// It attaches @[filepath] tags for all uploaded files so the model
// knows their exact locations on disk, and provides a clear default
// instruction when the user only sends files without text.
func ComposePrompt(text string, attachments []crushapi.Attachment) string {
	var tags []string
	for _, att := range attachments {
		path := att.FilePath
		if path == "" {
			path = att.FileName
		}
		if path != "" {
			tags = append(tags, fmt.Sprintf("@[%s]", path))
		}
	}

	trimmed := strings.TrimSpace(text)
	tagBlock := strings.Join(tags, "\n")

	if len(tags) == 0 {
		return trimmed
	}

	if trimmed == "" {
		if len(tags) == 1 {
			return fmt.Sprintf("Hãy xem và xử lý tệp đính kèm sau:\n\n%s", tagBlock)
		}
		return fmt.Sprintf("Hãy xem và xử lý các tệp đính kèm sau:\n\n%s", tagBlock)
	}

	return fmt.Sprintf("%s\n\n%s", trimmed, tagBlock)
}
