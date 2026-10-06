package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Dyu-36/gotack/internal/engineapi"
)

func TestToMessageInfoCompletion(t *testing.T) {
	tests := []struct {
		name      string
		role      string
		updatedAt int64
		parts     json.RawMessage
		completed int64
	}{
		{name: "assistant finish timestamp", role: "assistant", updatedAt: 50, parts: json.RawMessage(`[{"type":"finish","data":{"time":45}}]`), completed: 45000},
		{name: "assistant update fallback", role: "assistant", updatedAt: 50, completed: 50000},
		{name: "assistant without update", role: "assistant", updatedAt: 42},
		{name: "user ignores finish", role: "user", updatedAt: 50, parts: json.RawMessage(`[{"type":"finish","data":{"time":45}}]`)},
		{name: "malformed parts use update", role: "assistant", updatedAt: 50, parts: json.RawMessage(`{`), completed: 50000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toMessageInfo(engineapi.Message{
				ID:        "message",
				Role:      tt.role,
				CreatedAt: 42,
				UpdatedAt: tt.updatedAt,
				Parts:     tt.parts,
			})
			if got.CreatedAt != 42000 || got.CompletedAt != tt.completed {
				t.Fatalf("timestamps = %d/%d, want 42000/%d", got.CreatedAt, got.CompletedAt, tt.completed)
			}
			if got.Attachments != nil || got.ToolCalls != nil {
				t.Fatalf("empty collections changed: %#v", got)
			}
		})
	}
}

func TestToMessageInfoBinaryAttachments(t *testing.T) {
	root := t.TempDir()
	existing := filepath.Join(root, "photo.png")
	if err := os.WriteFile(existing, []byte("persisted file"), 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		path    string
		mime    string
		size    int
		content string
	}{
		{name: "image uses persisted size", path: existing, mime: "image/png", size: 14, content: "iVBORw=="},
		{name: "missing image uses payload size", path: filepath.Join(root, "missing.png"), mime: "image/png", size: 4, content: "iVBORw=="},
		{name: "text omits binary content", path: filepath.Join(root, "notes.txt"), mime: "text/plain", size: 4},
		{name: "mime prefix remains case sensitive", path: filepath.Join(root, "upper.png"), mime: "Image/png", size: 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parts, err := json.Marshal([]any{map[string]any{
				"type": "binary",
				"data": map[string]any{"Path": tt.path, "MIMEType": tt.mime, "Data": "iVBORw=="},
			}})
			if err != nil {
				t.Fatal(err)
			}
			got := toMessageInfo(engineapi.Message{Parts: parts})
			want := AttachmentInfo{FileName: filepath.Base(tt.path), MimeType: tt.mime, Size: tt.size, Content: tt.content, Path: tt.path}
			if len(got.Attachments) != 1 || got.Attachments[0] != want {
				t.Fatalf("attachments = %#v, want %#v", got.Attachments, want)
			}
		})
	}
}
