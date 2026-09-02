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

type iterationRecord struct {
	messageID string
	hasTools  bool
}

type learningRecord struct {
	sessionID string
	callID    string
	toolName  string
}

type iterationSink struct {
	records  []iterationRecord
	learning []learningRecord
}

func (s *iterationSink) assistantIteration(_ string, messageID string, hasTools bool) {
	s.records = append(s.records, iterationRecord{messageID, hasTools})
}

func (s *iterationSink) learningToolExecuted(sessionID, callID, toolName string) {
	s.learning = append(s.learning, learningRecord{sessionID, callID, toolName})
}

func (s *iterationSink) callbacks() Callbacks {
	return Callbacks{
		AssistantIteration:   s.assistantIteration,
		LearningToolExecuted: s.learningToolExecuted,
	}
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

func TestForwarderDelayPolicyAndTestOverride(t *testing.T) {
	f := NewForwarder(slog.Default(), nil, Callbacks{})
	if got := f.nextDelay(1); got != 16*time.Millisecond {
		t.Fatalf("small delta delay = %v, want 16ms", got)
	}
	if got := f.nextDelay(513); got != coalesceDelay {
		t.Fatalf("medium delta delay = %v, want %v", got, coalesceDelay)
	}
	if got := f.nextDelay(4097); got != 80*time.Millisecond {
		t.Fatalf("large delta delay = %v, want 80ms", got)
	}
	f.setDelay(123 * time.Millisecond)
	if got := f.nextDelay(1); got != 123*time.Millisecond {
		t.Fatalf("override delay = %v, want 123ms", got)
	}
}

func TestForwarderCoalescesDeltas(t *testing.T) {
	var c collector
	f := NewForwarder(slog.Default(), c.emit, Callbacks{})
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
	// Every coalesced flush of the same message must carry a strictly
	// increasing seq starting at 1, so the frontend can detect a gap.
	for i, ev := range deltas {
		p := ev.data.(SessionDeltaPayload)
		if p.Seq != int64(i+1) {
			t.Fatalf("deltas[%d].Seq = %d, want %d", i, p.Seq, i+1)
		}
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
	if last.Seq < 1 {
		t.Fatalf("final delta seq = %d, want >= 1", last.Seq)
	}
}

func TestForwarderRunCompleteFlushesAndOrders(t *testing.T) {
	var c collector
	f := NewForwarder(slog.Default(), c.emit, Callbacks{})
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
	if got := deltas[0].data.(SessionDeltaPayload).Seq; got != 1 {
		t.Fatalf("drain seq = %d, want 1 (first delta for new message)", got)
	}
	dones := c.of(SessionDone)
	if len(dones) != 1 {
		t.Fatalf("expected exactly one done event, got %d", len(dones))
	}
}

func TestForwarderToolActivityImmediate(t *testing.T) {
	var c collector
	f := NewForwarder(slog.Default(), c.emit, Callbacks{})
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

func TestForwarderReportsLateSkillManageMetadata(t *testing.T) {
	var c collector
	sink := &iterationSink{}
	f := NewForwarder(slog.Default(), c.emit, sink.callbacks())
	f.setDelay(time.Hour)

	f.handle(messageUpdate("m", "s", "", textParts("thinking")))
	parts := json.RawMessage(`[{"type":"tool_call","data":{"id":"t1","name":"mcp_gotack-skills_skill_manage","input":{},"finished":false}}]`)
	f.handle(messageUpdate("m", "s", "", parts))

	if len(sink.records) != 2 || sink.records[0].hasTools ||
		!sink.records[1].hasTools {
		t.Fatalf("iteration snapshots = %+v", sink.records)
	}
}

func TestLearningToolResultAdmissionRequiresGuardedCallID(t *testing.T) {
	cases := []struct {
		name   string
		result crushapi.ToolResult
		want   int
	}{
		{
			name:   "failed result",
			result: crushapi.ToolResult{ToolCallID: "call-1", Name: "memory", IsError: true},
			want:   1,
		},
		{
			name:   "missing call id",
			result: crushapi.ToolResult{Name: "memory"},
		},
		{
			name:   "successful result",
			result: crushapi.ToolResult{ToolCallID: "call-1", Name: "memory"},
			want:   1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sink := &iterationSink{}
			f := NewForwarder(slog.Default(), nil, sink.callbacks())
			parts, err := json.Marshal([]map[string]any{{
				"type": "tool_result",
				"data": tc.result,
			}})
			if err != nil {
				t.Fatal(err)
			}
			f.handle(messageUpdate("result-message", "session", "", parts))
			if got := len(sink.learning); got != tc.want {
				t.Fatalf("learning callbacks = %d, want %d: %+v", got, tc.want, sink.learning)
			}
		})
	}
}

func TestForwarderReportsOnlyAdmittedLearningResults(t *testing.T) {
	var c collector
	sink := &iterationSink{}
	f := NewForwarder(slog.Default(), c.emit, sink.callbacks())
	f.setDelay(time.Hour)
	message := func(id string, result map[string]any) crushapi.StreamEvent {
		payload, _ := json.Marshal(map[string]any{
			"id": id, "session_id": "s", "role": "tool",
			"parts": []map[string]any{{"type": "tool_result", "data": result}},
		})
		return crushapi.StreamEvent{Kind: "message", Event: "updated", Payload: payload}
	}
	f.handle(message("denied", map[string]any{
		"tool_call_id": "d", "name": "memory", "content": "User denied permission",
	}))
	f.handle(message("hook-denied", map[string]any{
		"tool_call_id": "h", "name": "skill_manage", "content": "blocked",
		"metadata": `{"hook":{"decision":"deny","halt":false}}`,
	}))
	f.handle(message("ok", map[string]any{
		"tool_call_id": "a", "name": "memory", "content": "failed while executing",
		"is_error": false,
	}))
	if len(sink.learning) != 1 || sink.learning[0].callID != "a" || sink.learning[0].toolName != "memory" {
		t.Fatalf("learning results = %+v", sink.learning)
	}
}

// TestForwarderAppendIsSuffix verifies that consecutive flushes for the same
// message emit a suffix, not a full re-send. This is the contract the
// frontend relies on to render the append-only model without re-walking the
// whole text on every tick.
func TestForwarderAppendIsSuffix(t *testing.T) {
	var c collector
	f := NewForwarder(slog.Default(), c.emit, Callbacks{})
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
	if first.Seq != 1 {
		t.Fatalf("first delta seq = %d, want 1", first.Seq)
	}
	// The seq must be strictly increasing across all flushes of the same
	// message; that monotonicity is what applyDelta uses to decide
	// between append and resync.
	for i := 1; i < len(deltas); i++ {
		prev := deltas[i-1].data.(SessionDeltaPayload)
		cur := deltas[i].data.(SessionDeltaPayload)
		if cur.Seq != prev.Seq+1 {
			t.Fatalf("deltas[%d].Seq = %d, want %d", i, cur.Seq, prev.Seq+1)
		}
	}
	last := deltas[len(deltas)-1].data.(SessionDeltaPayload)
	if last.Append != " world" {
		t.Fatalf("last append = %q want %q", last.Append, " world")
	}
	if last.Text != "Hello world" {
		t.Fatalf("last full text = %q want %q", last.Text, "Hello world")
	}
	if last.Seq != int64(len(deltas)) {
		t.Fatalf("last delta seq = %d, want %d", last.Seq, len(deltas))
	}
}
