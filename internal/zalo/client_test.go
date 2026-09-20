package zalo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseUpdatesAcceptsInboundImageShapes(t *testing.T) {
	raw := json.RawMessage(`[
		{"event_name":"message.image.received","message":{"message_id":"img-1","chat":{"id":"777"},"photo":"https://cdn.zalo.me/anh-bao-cao.jpg","caption":"Đọc giúp tôi bảng này"}},
		{"event_name":"message.image.received","message":{"message_id":"img-2","chat":{"id":"778"},"photo_url":"https://cdn.zalo.me/hoa-don.png"}}
	]`)
	updates := parseUpdates(raw)
	if len(updates) != 2 {
		t.Fatalf("expected 2 image updates, got %d", len(updates))
	}
	if updates[0].AttachmentURL != "https://cdn.zalo.me/anh-bao-cao.jpg" || updates[0].Text != "Đọc giúp tôi bảng này" {
		t.Fatalf("unexpected first image: %+v", updates[0])
	}
	if updates[1].AttachmentURL != "https://cdn.zalo.me/hoa-don.png" {
		t.Fatalf("unexpected direct photo URL: %q", updates[1].AttachmentURL)
	}
}

func TestParseUpdatesAcceptsDataWrapper(t *testing.T) {
	raw := json.RawMessage(`{"data":[{"message":{"message_id":"img-3","chat_id":"779","image":{"download_url":"https://cdn.zalo.me/image.webp"}}}]}`)
	updates := parseUpdates(raw)
	if len(updates) != 1 || updates[0].AttachmentURL != "https://cdn.zalo.me/image.webp" {
		t.Fatalf("unexpected wrapped image: %+v", updates)
	}
}

func TestInboundImageReachesAgentTurn(t *testing.T) {
	const imageBody = "fake image payload"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/image" {
			w.Header().Set("Content-Type", "image/webp")
			_, _ = w.Write([]byte(imageBody))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
	}))
	defer server.Close()
	t.Setenv("ZALO_BOT_API_BASE", server.URL)
	client, err := NewClient("test-token")
	if err != nil {
		t.Fatal(err)
	}
	client.allowPrivateHosts = true
	root := t.TempDir()
	var prompt string
	var received Turn
	manager := NewManager(filepath.Join(root, "zalo.json"), Runtime{
		Prepare: func(context.Context, string, string) (Turn, error) {
			return Turn{WorkspaceID: "workspace", WorkspacePath: root, SessionID: "session-1"}, nil
		},
		Run: func(_ context.Context, turn Turn, content string) error {
			prompt, received = content, turn
			return nil
		},
	}, nil)
	defer manager.Stop()
	manager.state.Token = "test-token"
	manager.state.PairedChatIDs = []string{"780"}
	updates := parseUpdates(json.RawMessage(`{"message":{"message_id":"img-4","chat":{"id":"780"},"photo_url":"` + server.URL + `/image"}}`))
	if len(updates) != 1 {
		t.Fatalf("expected one image update, got %d", len(updates))
	}
	manager.startTurn(context.Background(), client, updates[0])
	const pathMarker = "Zalo attachment saved at: "
	markerAt := strings.Index(prompt, pathMarker)
	if markerAt < 0 {
		t.Fatalf("agent prompt lacks downloaded image path: %q", prompt)
	}
	path := strings.TrimSpace(prompt[markerAt+len(pathMarker):])
	if received.ID == "" || !strings.HasPrefix(path, filepath.Join(root, ".tack", "zalo-inbox", received.ID)+string(filepath.Separator)) {
		t.Fatalf("attachment is not isolated to this run: %s (%+v)", path, received)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != imageBody || !strings.HasSuffix(path, ".webp") {
		t.Fatalf("downloaded image changed: %s %q", path, data)
	}
}

func TestAttachmentFileNamePreservesImageContentType(t *testing.T) {
	for contentType, suffix := range map[string]string{"image/gif": ".gif", "image/webp": ".webp", "image/bmp": ".bmp", "image/heic": ".jpg"} {
		t.Run(contentType, func(t *testing.T) {
			got := attachmentFileName("https://cdn.zalo.me/photo/asset", contentType)
			if got != "asset"+suffix {
				t.Fatalf("attachmentFileName() = %q, want %q", got, "asset"+suffix)
			}
		})
	}
}
