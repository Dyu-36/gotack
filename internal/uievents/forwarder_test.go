package uievents

import (
	"encoding/json"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/Dyu-36/gotack/internal/crushapi"
)

func textParts(text string) json.RawMessage {
	return json.RawMessage(`[{"type":"text","data":{"text":` + quote(text) + `}}]`)
}

func quote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func messageUpdate(id, sessionID, text string, parts json.RawMessage) crushapi.StreamEvent {
	payload, _ := json.Marshal(map[string]any{
		"id": id, "session_id": sessionID, "role": "assistant", "parts": parts,
	})
	return crushapi.StreamEvent{Kind: "message", Event: "updated", Payload: payload}
}

type collector struct {
	mu     sync.Mutex
	events []namedEvent
}

type namedEvent struct {
	name string
	data any
}

func (c *collector) emit(name string, data any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, namedEvent{name, data})
}

func (c *collector) of(name string) []namedEvent {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []namedEvent
	for _, e := range c.events {
		if e.name == name {
			out = append(out, e)
		}
	}
	return out
}

func TestForwarderCoalescesDeltas(t *testing.T) {
	var c collector
	f := NewForwarder(slog.Default(), c.emit, nil)
	f.setDelay(30 * time.Millisecond)

	ch := make(chan crushapi.StreamEvent, 8)
	done := make(chan struct{})
	go func() {
		f.Consume(ch)
		close(done)
	}()

	ch <- messageUpdate("m1", "s1", "Hel", nil)
	ch <- messageUpdate("m1", "s1", "Hello", nil)
	ch <- messageUpdate("m1", "s1", "Hello wor", textParts("Hello world"))
	ch <- messageUpdate("m1", "s1", "", textParts("Hello world"))
	close(ch)
	<-done

	time.Sleep(60 * time.Millisecond)
	deltas := c.of(SessionDelta)
	if len(deltas) == 0 {
		t.Fatal("expected at least one coalesced delta")
	}
	if len(deltas) > 3 {
		t.Fatalf("coalescing failed: %d deltas for 4 updates", len(deltas))
	}
	last := deltas[len(deltas)-1].data.(SessionDeltaPayload)
	if last.Text != "Hello world" || last.MessageID != "m1" || last.SessionID != "s1" {
		t.Fatalf("final delta = %+v", last)
	}
	// Append always equals the full text for the first flush of a message
	// because the UI has not seen anything yet. Older clients that ignore
	// the field keep working; new clients can swap to append-only.
	if last.Append != "Hello world" {
		t.Fatalf("first-flush append = %q want %q", last.Append, "Hello world")
	}
}

func TestForwarderRunCompleteFlushesAndOrders(t *testing.T) {
	var c collector
	f := NewForwarder(slog.Default(), c.emit, nil)
	f.setDelay(time.Second) // only the run_complete flush path fires it

	ch := make(chan crushapi.StreamEvent, 4)
	done := make(chan struct{})
	go func() {
		f.Consume(ch)
		close(done)
	}()

	ch <- messageUpdate("m9", "s2", "", textParts("final words"))
	rc, _ := json.Marshal(crushapi.RunComplete{SessionID: "s2", MessageID: "m9", Text: "final words"})
	ch <- crushapi.StreamEvent{Kind: "run_complete", Payload: rc}
	close(ch)
	<-done

	deltas := c.of(SessionDelta)
	if len(deltas) != 1 || deltas[0].data.(SessionDeltaPayload).Text != "final words" {
		t.Fatalf("run_complete must flush pending delta first, got %+v", deltas)
	}
	if got := deltas[0].data.(SessionDeltaPayload).Append; got != "final words" {
		t.Fatalf("drain append = %q want %q", got, "final words")
	}

	dones := c.of(SessionDone)
	if len(dones) != 1 {
		t.Fatalf("expected exactly one done event, got %d", len(dones))
	}
}

func TestForwarderToolActivityImmediate(t *testing.T) {
	var c collector
	f := NewForwarder(slog.Default(), c.emit, nil)
	f.setDelay(time.Hour)

	parts := json.RawMessage(`[{"type":"tool_call","data":{"id":"t1","name":"bash","input":"ls","finished":true}}]`)
	ch := make(chan crushapi.StreamEvent, 1)
	go f.Consume(ch)
	ch <- messageUpdate("m", "s3", "", parts)
	time.Sleep(50 * time.Millisecond)

	tools := c.of(ToolActivity)
	if len(tools) != 1 {
		t.Fatalf("expected immediate tool activity, got %d", len(tools))
	}
	got := tools[0].data.(ToolActivityPayload)
	if got.Name != "bash" || got.ToolCallID != "t1" || !got.Finished {
		t.Fatalf("tool payload = %+v", got)
	}
}

// TestForwarderAppendIsSuffix verifies that consecutive flushes for the same
// message emit a suffix, not a full re-send. This is the contract the
// frontend relies on to render the append-only model without re-walking the
// whole text on every tick.
func TestForwarderAppendIsSuffix(t *testing.T) {
	var c collector
	f := NewForwarder(slog.Default(), c.emit, nil)
	f.setDelay(20 * time.Millisecond)

	ch := make(chan crushapi.StreamEvent, 4)
	done := make(chan struct{})
	go func() {
		f.Consume(ch)
		close(done)
	}()

	// Two distinct full-text updates; the first flush should carry the full
	// text, the second should carry only the suffix appended since flush 1.
	ch <- messageUpdate("mA", "sA", "", textParts("Hello"))
	time.Sleep(40 * time.Millisecond)
	ch <- messageUpdate("mA", "sA", "", textParts("Hello world"))
	rc, _ := json.Marshal(crushapi.RunComplete{SessionID: "sA", MessageID: "mA"})
	ch <- crushapi.StreamEvent{Kind: "run_complete", Payload: rc}
	close(ch)
	<-done

	deltas := c.of(SessionDelta)
	if len(deltas) < 2 {
		t.Fatalf("expected at least 2 deltas, got %d: %+v", len(deltas), deltas)
	}
	first := deltas[0].data.(SessionDeltaPayload)
	if first.Append != "Hello" {
		t.Fatalf("first append = %q want %q", first.Append, "Hello")
	}
	last := deltas[len(deltas)-1].data.(SessionDeltaPayload)
	if last.Append != " world" {
		t.Fatalf("last append = %q want %q", last.Append, " world")
	}
	if last.Text != "Hello world" {
		t.Fatalf("last full text = %q want %q", last.Text, "Hello world")
	}
}
