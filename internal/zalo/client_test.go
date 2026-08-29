package zalo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func serveEnvelope(t *testing.T, token, method string, result any) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bot"+token+"/"+method {
			t.Errorf("got path %s, want /bot%s/%s", r.URL.Path, token, method)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": result})
	}))
	t.Cleanup(server.Close)
	return server
}

func TestGetMe(t *testing.T) {
	server := serveEnvelope(t, "tok", "getMe", map[string]any{"id": "1", "account_name": "Tack Bot", "account_type": "BOT"})
	client := NewClient("tok")
	client.base = server.URL

	info, err := client.GetMe(context.Background())
	if err != nil {
		t.Fatalf("GetMe: %v", err)
	}
	if info.Name != "Tack Bot" {
		t.Fatalf("got bot name %q", info.Name)
	}
}

func TestGetUpdates(t *testing.T) {
	update := map[string]any{
		"event_name": "message.text.received",
		"message": map[string]any{
			"message_id": "m1",
			"from":       map[string]any{"id": "u1", "name": "An"},
			"chat":       map[string]any{"id": "c1", "chat_type": "PRIVATE"},
			"text":       "hello",
		},
	}
	server := serveEnvelope(t, "tok", "getUpdates", update)
	client := NewClient("tok")
	client.base = server.URL

	got, err := client.GetUpdates(context.Background(), time.Second)
	if err != nil {
		t.Fatalf("GetUpdates: %v", err)
	}
	if got == nil || got.Message.MessageID != "m1" || got.Message.Chat.ID != "c1" {
		t.Fatalf("unexpected update %+v", got)
	}
}

func TestGetUpdatesEmptyPoll(t *testing.T) {
	server := serveEnvelope(t, "tok", "getUpdates", nil)
	client := NewClient("tok")
	client.base = server.URL

	got, err := client.GetUpdates(context.Background(), time.Second)
	if err != nil || got != nil {
		t.Fatalf("want nil update, got %v, err %v", got, err)
	}
}

func TestGetUpdatesPollTimeoutIsNotAnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error_code": 408, "description": "timeout"})
	}))
	t.Cleanup(server.Close)
	client := NewClient("tok")
	client.base = server.URL

	got, err := client.GetUpdates(context.Background(), time.Second)
	if err != nil || got != nil {
		t.Fatalf("408 must read as empty poll, got %v err %v", got, err)
	}
}

func TestSendMessage(t *testing.T) {
	var mu sync.Mutex
	var body map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(server.Close)
	client := NewClient("tok")
	client.base = server.URL

	if err := client.SendMessage(context.Background(), "c1", "hi"); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if body["chat_id"] != "c1" || body["text"] != "hi" {
		t.Fatalf("unexpected body %v", body)
	}
}

func TestClientCallSurfacesAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":false,"error_code":401,"description":"bad token"}`))
	}))
	t.Cleanup(server.Close)
	client := NewClient("tok")
	client.base = server.URL

	if _, err := client.GetMe(context.Background()); err == nil {
		t.Fatal("expected error for ok=false")
	}
}
