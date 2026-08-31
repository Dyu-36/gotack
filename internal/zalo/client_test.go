package zalo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestParseUpdatesAcceptsInboundImageShapes(t *testing.T) {
	raw := json.RawMessage(`[
		{
			"event_name": "message.image.received",
			"message": {
				"message_id": "img-1",
				"chat": {"id": "777"},
				"photo": "https://cdn.zalo.me/anh-bao-cao.jpg",
				"caption": "Đọc giúp tôi bảng này"
			}
		},
		{
			"event_name": "message.image.received",
			"message": {
				"message_id": "img-2",
				"chat": {"id": "778"},
				"photo_url": "https://cdn.zalo.me/hoa-don.png"
			}
		}
	]`)

	updates := parseUpdates(raw)
	if len(updates) != 2 {
		t.Fatalf("expected 2 image updates, got %d", len(updates))
	}
	if updates[0].AttachmentURL != "https://cdn.zalo.me/anh-bao-cao.jpg" {
		t.Fatalf("unexpected nested photo URL: %q", updates[0].AttachmentURL)
	}
	if updates[0].Text != "Đọc giúp tôi bảng này" {
		t.Fatalf("unexpected image caption: %q", updates[0].Text)
	}
	if updates[1].AttachmentURL != "https://cdn.zalo.me/hoa-don.png" {
		t.Fatalf("unexpected direct photo URL: %q", updates[1].AttachmentURL)
	}
}

func TestParseUpdatesAcceptsDataWrapper(t *testing.T) {
	raw := json.RawMessage(`{
		"data": [{
			"message": {
				"message_id": "img-3",
				"chat_id": "779",
				"image": {"download_url": "https://cdn.zalo.me/image.webp"}
			}
		}]
	}`)

	updates := parseUpdates(raw)
	if len(updates) != 1 {
		t.Fatalf("expected one wrapped update, got %d", len(updates))
	}
	if updates[0].AttachmentURL != "https://cdn.zalo.me/image.webp" {
		t.Fatalf("unexpected wrapped image URL: %q", updates[0].AttachmentURL)
	}
}

func TestInboundImageReachesAgentTurn(t *testing.T) {
	const imageBody = "fake image payload"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/image" {
			writer.Header().Set("Content-Type", "image/webp")
			_, _ = writer.Write([]byte(imageBody))
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"ok":true,"result":{}}`))
	}))
	defer server.Close()
	t.Setenv("ZALO_BOT_API_BASE", server.URL)

	client, err := NewClient("test-token")
	if err != nil {
		t.Fatal(err)
	}
	var prompt string
	manager := NewManager(t.TempDir()+"/zalo.json", Runtime{
		Start: func(_ context.Context, _, _, content string) (string, error) {
			prompt = content
			return "session-1", nil
		},
	}, nil)
	updates := parseUpdates(json.RawMessage(`{
		"message": {
			"message_id": "img-4",
			"chat": {"id": "780"},
			"photo_url": "` + server.URL + `/image"
		}
	}`))
	if len(updates) != 1 {
		t.Fatalf("expected one image update, got %d", len(updates))
	}

	manager.startTurn(context.Background(), client, updates[0])
	const pathMarker = "Tệp Zalo đã tải về máy tại: "
	markerAt := strings.Index(prompt, pathMarker)
	if markerAt < 0 {
		t.Fatalf("agent prompt does not contain the downloaded image path: %q", prompt)
	}
	path := strings.TrimSpace(prompt[markerAt+len(pathMarker):])
	t.Cleanup(func() { _ = os.Remove(path) })
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("downloaded image is unavailable: %v", err)
	}
	if string(data) != imageBody {
		t.Fatalf("downloaded image body = %q, want %q", data, imageBody)
	}
	if !strings.HasSuffix(path, ".webp") {
		t.Fatalf("downloaded image lost its content-type extension: %q", path)
	}
}

func TestAttachmentFileNamePreservesImageContentType(t *testing.T) {
	tests := map[string]string{
		"image/gif":  ".gif",
		"image/webp": ".webp",
		"image/bmp":  ".bmp",
		"image/heic": ".jpg",
	}
	for contentType, suffix := range tests {
		t.Run(contentType, func(t *testing.T) {
			got := attachmentFileName("https://cdn.zalo.me/photo/asset", contentType)
			if got != "asset"+suffix {
				t.Fatalf("attachmentFileName() = %q, want %q", got, "asset"+suffix)
			}
		})
	}
}
