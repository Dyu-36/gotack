package crushapi

import (
	"encoding/json"
	"testing"
)

func TestDecodeEnvelopePreservesFlatRunComplete(t *testing.T) {
	line := `{"type":"run_complete","payload":{"session_id":"session-1","run_id":"run-1","text":"done"}}`
	event, ok := decodeEnvelope(line, map[string]struct{}{"run_complete": {}})
	if !ok {
		t.Fatal("flat run_complete was dropped")
	}
	if event.Kind != "run_complete" || event.Event != "" {
		t.Fatalf("flat run_complete metadata = kind %q event %q", event.Kind, event.Event)
	}
	var terminal RunComplete
	if err := json.Unmarshal(event.Payload, &terminal); err != nil {
		t.Fatalf("decode flat run_complete payload: %v", err)
	}
	if terminal.SessionID != "session-1" || terminal.RunID != "run-1" || terminal.Text != "done" {
		t.Fatalf("flat run_complete payload = %#v", terminal)
	}
}

func TestDecodeEnvelopeUnwrapsLifecycleEvent(t *testing.T) {
	line := `{"type":"message","payload":{"type":"updated","payload":{"id":"message-1"}}}`
	event, ok := decodeEnvelope(line, map[string]struct{}{"message": {}})
	if !ok {
		t.Fatal("wrapped message event was dropped")
	}
	if event.Kind != "message" || event.Event != "updated" {
		t.Fatalf("wrapped event metadata = kind %q event %q", event.Kind, event.Event)
	}
	var payload struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		t.Fatalf("decode wrapped payload: %v", err)
	}
	if payload.ID != "message-1" {
		t.Fatalf("wrapped payload id = %q", payload.ID)
	}
}

func TestDecodeEnvelopeHonorsAllowList(t *testing.T) {
	line := `{"type":"message","payload":{"type":"updated","payload":{"id":"message-1"}}}`
	if _, ok := decodeEnvelope(line, map[string]struct{}{"run_complete": {}}); ok {
		t.Fatal("disallowed event was emitted")
	}
}
