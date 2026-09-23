package main

import "github.com/Dyu-36/gotack/internal/attachments"

func decodePromptAttachments(input []PromptAttachment, supportsVision bool) []attachments.Prepared {
	items := make([]attachments.Input, len(input))
	for i, item := range input {
		items[i] = attachments.Input{
			FileName: item.FileName,
			MimeType: item.MimeType,
			Content:  item.Content,
			Path:     item.Path,
		}
	}
	return attachments.PrepareInputs(items, supportsVision)
}
