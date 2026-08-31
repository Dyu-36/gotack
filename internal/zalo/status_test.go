package zalo

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestClientNegotiatesHTTP2(t *testing.T) {
	client, err := NewClient("test-token")
	if err != nil {
		t.Fatal(err)
	}
	transport, ok := client.http.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("unexpected transport type %T", client.http.Transport)
	}
	if !transport.ForceAttemptHTTP2 {
		t.Fatal("Zalo Bot API requires HTTP/2 negotiation")
	}
}

func TestStatusUsesFrontendPairedChatField(t *testing.T) {
	payload, err := json.Marshal(Status{PairedChatIDs: []string{"chat-1"}})
	if err != nil {
		t.Fatal(err)
	}
	text := string(payload)
	if !strings.Contains(text, `"paired_chat_ids":["chat-1"]`) {
		t.Fatalf("status JSON does not match desktop.ts: %s", text)
	}
	if strings.Contains(text, `"paired_chats"`) {
		t.Fatalf("status leaked the config-only field name: %s", text)
	}
}

func TestManagerStatusUsesEmptyPairedChatArray(t *testing.T) {
	manager := NewManager(t.TempDir()+"/zalo.json", Runtime{}, nil)
	payload, err := json.Marshal(manager.Status())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), `"paired_chat_ids":[]`) {
		t.Fatalf("empty paired chats must serialize as an array: %s", payload)
	}
}
