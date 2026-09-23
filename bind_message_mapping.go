package main

import (
	"encoding/base64"
	"os"
	"strings"

	"github.com/Dyu-36/gotack/internal/attachments"
	"github.com/Dyu-36/gotack/internal/engineapi"
)

const maxToolInputPreview = 4096

func toolInputPreview(input string) string {
	if len(input) <= maxToolInputPreview {
		return input
	}
	runes := 0
	for offset := range input {
		if runes == maxToolInputPreview {
			return input[:offset] + "…"
		}
		runes++
	}
	return input
}

func toMessageInfo(message engineapi.Message) MessageInfo {
	parts := engineapi.ExtractParts(message.Parts)
	text, refs := attachments.ParseAttachmentBlocks(parts.Text)
	info := MessageInfo{
		ID:        message.ID,
		Role:      string(message.Role),
		Text:      text,
		Model:     message.Model,
		Provider:  message.Provider,
		CreatedAt: engineapi.TimestampMillis(message.CreatedAt),
	}
	if message.Role == "assistant" {
		info.CompletedAt = parts.CompletedAt
		if info.CompletedAt == 0 && message.UpdatedAt > message.CreatedAt {
			info.CompletedAt = engineapi.TimestampMillis(message.UpdatedAt)
		}
	}
	for _, ref := range refs {
		info.Attachments = append(info.Attachments, AttachmentInfo{
			FileName: ref.FileName,
			MimeType: ref.MimeType,
			Size:     ref.Size,
			Path:     ref.Path,
		})
	}
	for _, attachment := range parts.Attachments {
		content := ""
		if strings.HasPrefix(attachment.MimeType, "image/") {
			content = base64.StdEncoding.EncodeToString(attachment.Content)
		}
		size := len(attachment.Content)
		if stat, err := os.Stat(attachment.FilePath); err == nil {
			size = int(stat.Size())
		}
		info.Attachments = append(info.Attachments, AttachmentInfo{
			FileName: attachments.BaseName(attachment.FileName),
			MimeType: attachment.MimeType,
			Size:     size,
			Content:  content,
			Path:     attachment.FilePath,
		})
	}
	for _, call := range parts.ToolCalls {
		info.ToolCalls = append(info.ToolCalls, ToolCallInfo{
			ID:       call.ID,
			Name:     call.Name,
			Input:    toolInputPreview(string(call.Input)),
			Finished: call.Finished,
		})
	}
	return info
}
