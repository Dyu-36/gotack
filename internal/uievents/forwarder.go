package uievents

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/Dyu-36/gotack/internal/crushapi"
)

// forwarder.go -- role: engine stream in, runtime.EventsEmit out.
//
// Coalesce high frequency token deltas before emitting to keep the webview light.

// Emitter is the surface the Wails runtime exposes. The signature mirrors
// runtime.EventsEmit so the bind layer can pass the function value in directly.
type Emitter func(name string, data any)

// Payload shapes shared with the Svelte layer. Field names are JSON tags; the
// front-end reads them verbatim.
type SessionDeltaPayload struct {
	SessionID string `json:"session_id"`
	MessageID string `json:"message_id"`
	Text      string `json:"text"`
	// Append is the suffix the UI has not seen yet: pm.text[len(pm.sent):]
	// computed at emit time. It is "" when nothing moved (acks after coalesce)
	// and equals Text on the very first delta for a message. Frontends that
	// want append-only behavior can ignore Text and concat Append; the field
	// stays present so older clients keep working unchanged.
	Append string `json:"append"`
}

type SessionDonePayload struct {
	SessionID string `json:"session_id"`
	Text      string `json:"text"`
	Error     string `json:"error,omitempty"`
	Cancelled bool   `json:"cancelled"`
}

type ToolActivityPayload struct {
	SessionID  string          `json:"session_id"`
	Name       string          `json:"name"`
	Input      json.RawMessage `json:"input"`
	Finished   bool            `json:"finished"`
	ToolCallID string          `json:"tool_call_id"`
}

// coalesceDelay bounds how long the forwarder waits after the last text
// update before flushing the coalesced text. 40ms keeps the UI responsive
// while collapsing the per-token burst into a single paint.
const coalesceDelay = 40 * time.Millisecond

// pendingMessage is the per-message coalescing state: the newest extracted
// text, the text the UI last received, and the tool-call states already
// announced. Tracking "sent" and "tools" here is what turns a burst of token
// updates into one delta per coalesce window and one tool:activity per real
// tool-state change, instead of one of each per SSE event.
type pendingMessage struct {
	sessionID string
	text      string
	sent      string
	timer     *time.Timer
	tools     map[string]bool // tool call id -> Finished, as last emitted
}

// markToolStates returns the calls whose state the UI has not seen yet and
// records them as sent. Calls without an ID cannot be deduplicated and are
// always returned. The caller must hold Forwarder.mu.
func (pm *pendingMessage) markToolStates(calls []crushapi.ToolCall) []crushapi.ToolCall {
	var out []crushapi.ToolCall
	for _, c := range calls {
		if c.ID != "" {
			if was, ok := pm.tools[c.ID]; ok && was == c.Finished {
				continue
			}
			if pm.tools == nil {
				pm.tools = make(map[string]bool, len(calls))
			}
			pm.tools[c.ID] = c.Finished
		}
		out = append(out, c)
	}
	return out
}

// PermissionSink registers an inbound permission request before the UI sees
// it, so the answer that comes back through the bound call can be matched to
// the request by ID. internal/permission.Relay implements it.
//
// The interface is declared here, on the consumer side, so uievents keeps no
// dependency on the permission package. It exists because the alternative was
// a permission-specific branch in the shared emit path that every session
// delta and every terminal chunk had to walk past.
type PermissionSink interface {
	Pending(req crushapi.PermissionRequest)
}

// Forwarder consumes crushapi stream events and dispatches them to the webview
// through the injected Emitter. Each Forwarder owns a single Consume loop; call
// Stop to flush in-flight deltas and tear down.
type Forwarder struct {
	log   *slog.Logger
	emit  Emitter
	perms PermissionSink
	delay time.Duration

	mu      sync.Mutex
	pending map[string]*pendingMessage

	stopOnce sync.Once
	stopped  bool
}

// NewForwarder returns a Forwarder ready to Consume. Both emit and perms may
// be nil, in which case those calls are skipped; the tests rely on that.
func NewForwarder(log *slog.Logger, emit Emitter, perms PermissionSink) *Forwarder {
	if log == nil {
		log = slog.Default()
	}
	return &Forwarder{
		log:     log,
		emit:    emit,
		perms:   perms,
		delay:   coalesceDelay,
		pending: make(map[string]*pendingMessage),
	}
}

// setDelay overrides the coalesce delay. Test-only.
func (f *Forwarder) setDelay(d time.Duration) {
	f.mu.Lock()
	f.delay = d
	f.mu.Unlock()
}

// Consume reads events from ch until the channel closes, dispatching each one
// to the appropriate webview event. It blocks for the lifetime of the stream;
// cancel via Stop.
func (f *Forwarder) Consume(events <-chan crushapi.StreamEvent) {
	for ev := range events {
		if f.isStopped() {
			return
		}
		f.handle(ev)
	}
	// Channel closed: flush any debounced text so the UI doesn't miss the tail
	// of a run.
	f.emitDeltas(f.drain(""))
}

// Stop signals the consumer to exit and flushes in-flight deltas. Safe to
// call multiple times.
func (f *Forwarder) Stop() {
	f.stopOnce.Do(func() {
		f.mu.Lock()
		f.stopped = true
		f.mu.Unlock()
	})
	f.emitDeltas(f.drain(""))
}

func (f *Forwarder) isStopped() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.stopped
}

func (f *Forwarder) handle(ev crushapi.StreamEvent) {
	switch ev.Kind {
	case "message":
		// Only the updated event carries incremental content; created carries
		// the seed message and deleted is not currently consumed by the UI.
		if ev.Event != "updated" {
			return
		}
		f.handleMessageUpdate(ev.Payload)
	case "run_complete":
		f.handleRunComplete(ev.Payload)
	case "permission_request":
		f.handlePermission(ev.Payload)
	case "question_batch_request":
		f.handleQuestion(ev.Payload)
	default:
		// Unknown kind: record it for debug visibility but never panic. New
		// event types from the engine should be reviewed and either added or
		// explicitly ignored here.
		if f.log != nil {
			f.log.Debug("uievents: ignoring unknown stream event", "kind", ev.Kind, "event", ev.Event)
		}
	}
}

// messageWire mirrors proto.Message on the wire. The Crush server marshals
// Message.Parts as a wrapped JSON array; we re-decode only the fields we need
// for the UI and let crushapi.ExtractParts do the heavy lifting on Parts.
type messageWire struct {
	ID        string          `json:"id"`
	SessionID string          `json:"session_id"`
	Role      string          `json:"role"`
	Parts     json.RawMessage `json:"parts"`
}

func (f *Forwarder) handleMessageUpdate(payload json.RawMessage) {
	var msg messageWire
	if err := json.Unmarshal(payload, &msg); err != nil {
		if f.log != nil {
			f.log.Debug("uievents: failed to decode message update", "err", err)
		}
		return
	}
	if msg.ID == "" {
		return
	}
	// One decode per event. The parts array is re-sent in full on every token
	// update, so decoding it twice was the hot path in this loop.
	f.schedule(msg.SessionID, msg.ID, crushapi.ExtractParts(msg.Parts))
}

// schedule records the newest text for messageID, arms the coalesce timer only
// when the text actually moved, and emits tool activity for the calls whose
// state changed. Tool activity stays undebounced because every real transition
// flips a "running" / "done" badge, but re-announcing an unchanged call on
// every token update is pure waste.
func (f *Forwarder) schedule(sessionID, messageID string, parts crushapi.Parts) {
	f.mu.Lock()
	pm, ok := f.pending[messageID]
	if !ok {
		pm = &pendingMessage{sessionID: sessionID}
		f.pending[messageID] = pm
	}
	pm.text = parts.Text
	if pm.text != pm.sent {
		if pm.timer != nil {
			// Coalesce: a newer update arrived before the previous timer fired.
			pm.timer.Stop()
		}
		pm.timer = time.AfterFunc(f.delay, func() { f.flush(messageID) })
	}
	fresh := pm.markToolStates(parts.ToolCalls)
	f.mu.Unlock()

	for _, c := range fresh {
		f.send(ToolActivity, ToolActivityPayload{
			SessionID:  sessionID,
			Name:       c.Name,
			Input:      c.Input,
			Finished:   c.Finished,
			ToolCallID: c.ID,
		})
	}
}

// flush emits the coalesced text for messageID when it differs from what the UI
// last received. The entry stays in the map so the tool-call dedupe state
// survives until the run completes or the stream closes.
func (f *Forwarder) flush(messageID string) {
	f.mu.Lock()
	pm, ok := f.pending[messageID]
	if !ok || pm.text == pm.sent {
		f.mu.Unlock()
		return
	}
	prev := pm.sent
	pm.timer = nil
	pm.sent = pm.text
	// Append is the suffix the UI has not seen yet: text[len(prev):]. It
	// equals the full text on the first delta for a message, and shrinks to
	// the new suffix as more tokens arrive. The full Text stays in the
	// payload so older clients keep their working behavior.
	append := pm.text[len(prev):]
	payload := SessionDeltaPayload{
		SessionID: pm.sessionID,
		MessageID: messageID,
		Text:      pm.text,
		Append:    append,
	}
	f.mu.Unlock()
	f.send(SessionDelta, payload)
}

func (f *Forwarder) handleRunComplete(payload json.RawMessage) {
	var rc crushapi.RunComplete
	if err := json.Unmarshal(payload, &rc); err != nil {
		if f.log != nil {
			f.log.Debug("uievents: failed to decode run_complete", "err", err)
		}
		return
	}
	// Flush any debounced text for this session before announcing done so the
	// final word is in the delta that the UI already showed. An empty session id
	// is malformed and must not drain every other session's pending text.
	if rc.SessionID != "" {
		f.emitDeltas(f.drain(rc.SessionID))
	}
	f.send(SessionDone, SessionDonePayload{
		SessionID: rc.SessionID,
		Text:      rc.Text,
		Error:     rc.Error,
		Cancelled: rc.Cancelled,
	})
}

func (f *Forwarder) handlePermission(payload json.RawMessage) {
	var req crushapi.PermissionRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		if f.log != nil {
			f.log.Debug("uievents: failed to decode permission_request", "err", err)
		}
		return
	}
	// Register before emitting: the UI can answer the moment it renders, and
	// AnswerPermission has to find the request by ID. This is the only code
	// that decodes the typed request, which is why the pairing belongs here
	// and not in the emit path shared with deltas and terminal output.
	if f.perms != nil {
		f.perms.Pending(req)
	}
	f.send(PermissionRequest, req)
}

func (f *Forwarder) handleQuestion(payload json.RawMessage) {
	var q crushapi.QuestionRequest
	if err := json.Unmarshal(payload, &q); err != nil {
		if f.log != nil {
			f.log.Debug("uievents: failed to decode question_batch_request", "err", err)
		}
		return
	}
	f.send(QuestionRequest, q)
}

func (f *Forwarder) send(name string, data any) {
	if f.emit == nil {
		return
	}
	f.emit(name, data)
}

// drain removes the pending messages for sessionID, or every pending message
// when sessionID is empty, and returns the deltas the UI has not received yet.
// The per-session and drain-everything paths were near-identical, so they share
// one implementation and one place that stops timers.
func (f *Forwarder) drain(sessionID string) []SessionDeltaPayload {
	f.mu.Lock()
	var out []SessionDeltaPayload
	for id, pm := range f.pending {
		if sessionID != "" && pm.sessionID != sessionID {
			continue
		}
		if pm.timer != nil {
			pm.timer.Stop()
		}
		if pm.text != pm.sent {
			out = append(out, SessionDeltaPayload{
				SessionID: pm.sessionID,
				MessageID: id,
				Text:      pm.text,
				Append:    pm.text[len(pm.sent):],
			})
		}
		delete(f.pending, id)
	}
	f.mu.Unlock()
	return out
}

// emitDeltas sends the payloads drain collected, outside the lock.
func (f *Forwarder) emitDeltas(deltas []SessionDeltaPayload) {
	for _, d := range deltas {
		f.send(SessionDelta, d)
	}
}
