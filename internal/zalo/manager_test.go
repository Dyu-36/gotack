package zalo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeServer struct {
	mu        sync.Mutex
	updates   [][]Update
	delivered []map[string]any
	server    *httptest.Server
}

func newFakeServer(t *testing.T, initial []Update) *fakeServer {
	t.Helper()
	rec := &fakeServer{updates: [][]Update{initial}}
	rec.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "getMe"):
			_, _ = w.Write([]byte(`{"ok":true,"result":{"id":"bot-1","display_name":"Gotack Bot"}}`))
		case strings.HasSuffix(r.URL.Path, "deleteWebhook"):
			_, _ = w.Write([]byte(`{"ok":true}`))
		case strings.HasSuffix(r.URL.Path, "getUpdates"):
			rec.mu.Lock()
			defer rec.mu.Unlock()
			if len(rec.updates) == 0 {
				_, _ = w.Write([]byte(`{"ok":true,"result":[]}`))
				return
			}
			next := rec.updates[0]
			rec.updates = rec.updates[1:]
			payload, _ := json.Marshal(next)
			_, _ = w.Write([]byte(`{"ok":true,"result":` + string(payload) + `}`))
		case strings.HasSuffix(r.URL.Path, "sendMessage"):
			rec.mu.Lock()
			rec.delivered = append(rec.delivered, map[string]any{
				"chat_id": body["chat_id"],
				"text":    body["text"],
			})
			rec.mu.Unlock()
			_, _ = w.Write([]byte(`{"ok":true}`))
		default:
			_, _ = w.Write([]byte(`{"ok":true}`))
		}
	}))
	t.Cleanup(rec.server.Close)
	return rec
}

func (f *fakeServer) deliveredMessages(t *testing.T) []string {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, 0, len(f.delivered))
	for _, entry := range f.delivered {
		if value, ok := entry["text"].(string); ok {
			out = append(out, value)
		}
	}
	return out
}

func tempPath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "zalo.json")
}

func newManagerForTest(t *testing.T, token string) (*Manager, *fakeServer) {
	t.Helper()
	server := newFakeServer(t, nil)
	m := NewManager(tempPath(t), Runtime{
		Start: func(_ context.Context, existing, chatID, text string) (string, error) {
			return "session-" + chatID + "-" + strings.ReplaceAll(text, " ", "_"), nil
		},
		Session: func(_ context.Context, id string) (string, error) { return "title:" + id, nil },
		Model:   func(_ context.Context) (string, error) { return "mock/large", nil },
	}, nil)
	m.clientFactory = func(tok string) (*Client, error) {
		c, err := NewClient(tok)
		if err != nil {
			return nil, err
		}
		c.base = server.server.URL
		return c, nil
	}
	if token != "" {
		status, err := m.SetToken(context.Background(), token)
		if err != nil {
			t.Fatalf("SetToken: %v", err)
		}
		if !status.Configured {
			t.Fatalf("status not configured: %+v", status)
		}
	}
	return m, server
}

func TestManagerSetTokenPersistsState(t *testing.T) {
	m, _ := newManagerForTest(t, "token-1")
	status := m.Status()
	if !status.Configured || status.Running == false {
		t.Fatalf("expected running configured channel, got %+v", status)
	}
	m.Stop()
	if m.Status().Running {
		t.Fatalf("Stop must mark manager idle")
	}
	data, err := os.ReadFile(m.path)
	if err != nil {
		t.Fatalf("state not persisted: %v", err)
	}
	var saved StoredChannel
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatalf("state parse: %v", err)
	}
	if saved.Token != "token-1" || saved.PairingCode == "" {
		t.Fatalf("persisted state missing fields: %+v", saved)
	}
}
func TestManagerPairingAndTurnRoundTrip(t *testing.T) {

	server := newFakeServer(t, nil)
	manager := NewManager(tempPath(t), Runtime{
		Start:   func(_ context.Context, _, chatID, _ string) (string, error) { return "session-" + chatID, nil },
		Session: func(_ context.Context, id string) (string, error) { return "title:" + id, nil },
		Model:   func(_ context.Context) (string, error) { return "mock/large", nil },
	}, nil)
	manager.clientFactory = func(tok string) (*Client, error) {
		c, _ := NewClient(tok)
		c.base = server.server.URL
		return c, nil
	}
	if _, err := manager.SetToken(context.Background(), "token-1"); err != nil {
		t.Fatalf("SetToken: %v", err)
	}
	defer manager.Stop()
	if !waitFor(t, 5*time.Second, func() bool { return manager.Status().BotName == "Gotack Bot" }) {
		t.Fatalf("set token did not populate bot name: %+v", manager.Status())
	}

	manager.mu.Lock()
	manager.state.PairedChatIDs = []string{"c1"}
	manager.state.ChatSessions = map[string]string{"c1": "session-c1"}
	manager.active["c1"] = "session-c1"
	manager.mu.Unlock()
	id := int64(1)
	manager.dispatch(context.Background(), mustClient(t, manager, "token-1"), Update{UpdateID: &id, MessageID: "m-1", ChatID: "c1", SenderName: "An", Text: "build me a report"})
	manager.Done("session-c1", "all finished")
	if !waitFor(t, 5*time.Second, func() bool {
		for _, message := range server.deliveredMessages(t) {
			if strings.Contains(message, "all finished") {
				return true
			}
		}
		return false
	}) {
		t.Fatalf("agent answer never delivered: %v", server.deliveredMessages(t))
	}
}

func TestDispatchAttachmentOnlyUsesDefaultPrompt(t *testing.T) {
	server := newFakeServer(t, nil)
	var gotText string
	manager := NewManager(tempPath(t), Runtime{
		Start: func(_ context.Context, _, _, text string) (string, error) {
			gotText = text
			return "session-c1", nil
		},
	}, nil)
	manager.mu.Lock()
	manager.state.PairedChatIDs = []string{"c1"}
	manager.mu.Unlock()

	client, err := NewClient("token-1")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	client.base = server.server.URL
	fileName := strings.ReplaceAll(t.Name(), "/", "-") + ".png"
	downloadedPath := filepath.Join(os.TempDir(), "gotack-zalo-inbox", fileName)
	t.Cleanup(func() { _ = os.Remove(downloadedPath) })

	manager.dispatch(context.Background(), client, Update{
		ChatID:        "c1",
		AttachmentURL: server.server.URL + "/" + fileName,
	})

	if !strings.Contains(gotText, defaultFilePrompt) {
		t.Fatalf("attachment-only prompt = %q, want default file prompt", gotText)
	}
	if !strings.Contains(gotText, downloadedPath) {
		t.Fatalf("attachment-only prompt = %q, want downloaded path %q", gotText, downloadedPath)
	}
}

func mustClient(t *testing.T, m *Manager, token string) *Client {
	t.Helper()
	client, err := m.newClient(token)
	if err != nil {
		t.Fatalf("newClient: %v", err)
	}
	return client
}

func TestManagerSendFileEnforcesAllowList(t *testing.T) {
	server := newFakeServer(t, nil)
	manager := NewManager(tempPath(t), Runtime{}, nil)
	manager.clientFactory = func(tok string) (*Client, error) {
		c, _ := NewClient(tok)
		c.base = server.server.URL
		return c, nil
	}
	if _, err := manager.SetToken(context.Background(), "token-1"); err != nil {
		t.Fatalf("SetToken: %v", err)
	}
	defer manager.Stop()
	image := filepath.Join(t.TempDir(), "report.png")
	if err := os.WriteFile(image, []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.SendFile(context.Background(), image, "c-allow"); err == nil {
		t.Fatalf("SendFile must reject unpaired chat")
	}
}

func TestChunkTextSplitsOnWhitespace(t *testing.T) {
	parts := chunkText("a b c d e", 3)
	if len(parts) < 2 || parts[0] == "" {
		t.Fatalf("expected at least 2 non-empty parts, got %v", parts)
	}
	if joined := strings.Join(parts, " "); strings.Count(joined, " ") < 4 {
		t.Fatalf("chunkText dropped characters: %q", joined)
	}
}

func TestSanitizeReplyStripsLinksAndPaths(t *testing.T) {
	got := sanitizeReply("Xin chào ![](C:/users/me/report.png) xem tại [link](https://example.com)")
	if strings.Contains(got, ".png") || strings.Contains(got, "https://example.com") {
		t.Fatalf("sanitize did not strip media references: %q", got)
	}
	if !strings.Contains(got, "link") {
		t.Fatalf("sanitize dropped visible label: %q", got)
	}
}

func waitFor(t *testing.T, deadline time.Duration, predicate func() bool) bool {
	t.Helper()
	timeout := time.NewTimer(deadline)
	defer timeout.Stop()
	for {
		if predicate() {
			return true
		}
		select {
		case <-timeout.C:
			return false
		case <-time.After(50 * time.Millisecond):
		}
	}
}
