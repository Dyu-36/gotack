package zalo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// recordingServer captures every sendMessage call the bridge makes.
type recordingServer struct {
	mu     sync.Mutex
	texts  []string
	server *httptest.Server
}

func newRecordingServer(t *testing.T) *recordingServer {
	t.Helper()
	rec := &recordingServer{}
	rec.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ChatID string `json:"chat_id"`
			Text   string `json:"text"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		rec.mu.Lock()
		rec.texts = append(rec.texts, body.Text)
		rec.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(rec.server.Close)
	return rec
}

func (r *recordingServer) sent() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.texts...)
}

func newTestBridge(t *testing.T, rec *recordingServer, allowed []string, start Starter) *Bridge {
	t.Helper()
	bridge := NewBridge("tok", allowed, start, nil)
	bridge.client.base = rec.server.URL
	return bridge
}

func textUpdate(id, chatID, text string) *Update {
	return &Update{
		EventName: "message.text.received",
		Message: Message{
			MessageID: id,
			From:      Sender{ID: "user-1", Name: "An"},
			Chat:      Chat{ID: chatID, ChatType: "PRIVATE"},
			Text:      text,
		},
	}
}

func TestBridgeIgnoresChatOutsideAllowList(t *testing.T) {
	rec := newRecordingServer(t)
	var calls int
	bridge := newTestBridge(t, rec, []string{"allowed"}, func(context.Context, string, string, string) (string, error) {
		calls++
		return "s1", nil
	})

	bridge.dispatch(context.Background(), textUpdate("m1", "stranger", "hi"))
	if calls != 0 || len(rec.sent()) != 0 {
		t.Fatalf("non-allowed chat must be ignored, calls=%d sent=%v", calls, rec.sent())
	}
}

func TestBridgeAnswersEachMessageOnce(t *testing.T) {
	rec := newRecordingServer(t)
	var calls int
	bridge := newTestBridge(t, rec, []string{"c1"}, func(context.Context, string, string, string) (string, error) {
		calls++
		return "s1", nil
	})

	bridge.dispatch(context.Background(), textUpdate("m1", "c1", "hello"))
	bridge.dispatch(context.Background(), textUpdate("m1", "c1", "hello"))

	if calls != 1 {
		t.Fatalf("duplicate delivery must be deduped, calls=%d", calls)
	}
}

func TestBridgeStartsTurnAndDeliversAnswer(t *testing.T) {
	rec := newRecordingServer(t)
	bridge := newTestBridge(t, rec, []string{"c1"}, func(_ context.Context, chatID, sender, text string) (string, error) {
		if chatID != "c1" || sender != "An" || text != "build me a report" {
			t.Fatalf("unexpected starter args %q %q %q", chatID, sender, text)
		}
		return "s1", nil
	})

	bridge.dispatch(context.Background(), textUpdate("m1", "c1", "build me a report"))
	bridge.Done("s1", "  all finished  ")

	sent := rec.sent()
	if len(sent) != 1 || sent[0] != "all finished" {
		t.Fatalf("unexpected replies %v", sent)
	}
}

func TestBridgeRejectsSecondMessageWhileBusy(t *testing.T) {
	rec := newRecordingServer(t)
	var calls int
	bridge := newTestBridge(t, rec, []string{"c1"}, func(context.Context, string, string, string) (string, error) {
		calls++
		return "s1", nil
	})

	bridge.dispatch(context.Background(), textUpdate("m1", "c1", "first"))
	bridge.dispatch(context.Background(), textUpdate("m2", "c1", "second"))

	if calls != 1 {
		t.Fatalf("busy chat must not start another turn, calls=%d", calls)
	}
	sent := rec.sent()
	if len(sent) != 1 || sent[0] != replyBusy {
		t.Fatalf("expected busy reply, got %v", sent)
	}

	bridge.Done("s1", "done")
	bridge.dispatch(context.Background(), textUpdate("m3", "c1", "third"))
	if calls != 2 {
		t.Fatalf("chat must unblock after Done, calls=%d", calls)
	}
}

func TestBridgeRepliesUnavailableWhenStarterFails(t *testing.T) {
	rec := newRecordingServer(t)
	bridge := newTestBridge(t, rec, []string{"c1"}, func(context.Context, string, string, string) (string, error) {
		return "", context.Canceled
	})

	bridge.dispatch(context.Background(), textUpdate("m1", "c1", "hi"))

	sent := rec.sent()
	if len(sent) != 1 || sent[0] != replyUnavailable {
		t.Fatalf("expected unavailable reply, got %v", sent)
	}
}

func TestBridgeTruncatesLongAnswer(t *testing.T) {
	rec := newRecordingServer(t)
	bridge := newTestBridge(t, rec, []string{"c1"}, func(context.Context, string, string, string) (string, error) {
		return "s1", nil
	})
	bridge.dispatch(context.Background(), textUpdate("m1", "c1", "hi"))

	bridge.Done("s1", longText(maxReplyChars+100))

	sent := rec.sent()
	if len(sent) != 1 || len(sent[0]) > maxReplyChars+len(replyTruncated) {
		t.Fatalf("reply must be truncated, got %d chars", len(sent[0]))
	}
}

func longText(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'a'
	}
	return string(b)
}
