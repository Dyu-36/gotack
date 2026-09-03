package uievents

import (
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/Dyu-36/gotack/internal/crushapi"
)

type Emitter func(name string, data any)

type SessionDeltaPayload struct {
	SessionID string `json:"session_id"`
	MessageID string `json:"message_id"`
	Text      string `json:"text"`
	Append    string `json:"append"`

	Seq int64 `json:"seq"`
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

type ChangesUpdatedPayload struct {
	SessionID string `json:"session_id"`
	Path      string `json:"path"`
}

type PermissionRequestPayload struct {
	Request   crushapi.PermissionRequest `json:"request"`
	ExpiresAt int64                      `json:"expires_at_ms"`
}

const coalesceDelay = 40 * time.Millisecond

type pendingMessage struct {
	sessionID string
	text      string
	sent      string
	timer     *time.Timer
	tools     map[string]bool

	nextSeq int64
}

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

type Callbacks struct {
	PermissionPending    func(crushapi.PermissionRequest) int64
	RunDone              func(SessionDonePayload)
	AssistantIteration   func(sessionID, messageID string, hasToolCalls bool)
	LearningToolExecuted func(sessionID, toolCallID, toolName string)
}

type Forwarder struct {
	log           *slog.Logger
	emit          Emitter
	callbacks     Callbacks
	delayOverride time.Duration

	mu       sync.Mutex
	pending  map[string]*pendingMessage
	stopOnce sync.Once
	stopped  bool
}

func NewForwarder(log *slog.Logger, emit Emitter, callbacks Callbacks) *Forwarder {
	if log == nil {
		log = slog.Default()
	}
	return &Forwarder{
		log: log, emit: emit, callbacks: callbacks,
		pending: make(map[string]*pendingMessage),
	}
}

func (f *Forwarder) setDelay(d time.Duration) {
	f.mu.Lock()
	f.delayOverride = d
	f.mu.Unlock()
}

func (f *Forwarder) nextDelay(bytesSinceLastFlush int) time.Duration {
	if f.delayOverride > 0 {
		return f.delayOverride
	}
	switch {
	case bytesSinceLastFlush > 4096:
		return 80 * time.Millisecond
	case bytesSinceLastFlush > 512:
		return 40 * time.Millisecond
	default:
		return 16 * time.Millisecond
	}
}

func (f *Forwarder) Consume(events <-chan crushapi.StreamEvent) {
	for ev := range events {
		if f.isStopped() {
			return
		}
		f.handle(ev)
	}
	f.emitDeltas(f.drain(""))
}

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
		if ev.Event == "updated" {
			f.handleMessageUpdate(ev.Payload)
		}
	case "run_complete":
		f.handleRunComplete(ev.Payload)
	case "permission_request":
		f.handlePermission(ev.Payload)
	case "file":
		f.handleFile(ev.Payload)
	default:
		if f.log != nil {
			f.log.Debug("uievents: ignoring unknown stream event", "kind", ev.Kind, "event", ev.Event)
		}
	}
}

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
	parts := crushapi.ExtractParts(msg.Parts)
	if strings.EqualFold(msg.Role, "assistant") && f.callbacks.AssistantIteration != nil {
		f.callbacks.AssistantIteration(msg.SessionID, msg.ID, len(parts.ToolCalls) > 0)
	}
	if f.callbacks.LearningToolExecuted != nil {
		for _, result := range parts.ToolResults {
			if learningResultAdmitted(result) {
				f.callbacks.LearningToolExecuted(msg.SessionID, result.ToolCallID, result.Name)
			}
		}
	}
	f.schedule(msg.SessionID, msg.ID, parts)
}

func learningResultAdmitted(result crushapi.ToolResult) bool {
	if result.ToolCallID == "" {
		return false
	}
	switch result.Name {
	case "memory", "mcp_gotack-memory_memory", "skill_manage", "mcp_gotack-skills_skill_manage":
	default:
		return false
	}
	if strings.TrimSpace(result.Content) == "User denied permission" {
		return false
	}
	var metadata struct {
		Hook *struct {
			Decision string `json:"decision"`
			Halt     bool   `json:"halt"`
		} `json:"hook"`
	}
	if json.Unmarshal([]byte(result.Metadata), &metadata) == nil && metadata.Hook != nil {
		if metadata.Hook.Halt || strings.EqualFold(metadata.Hook.Decision, "deny") {
			return false
		}
	}
	return true
}

func (f *Forwarder) schedule(sessionID, messageID string, parts crushapi.Parts) {
	f.mu.Lock()
	pm, ok := f.pending[messageID]
	if !ok {
		pm = &pendingMessage{sessionID: sessionID, nextSeq: 1}
		f.pending[messageID] = pm
	}
	pm.text = parts.Text
	if pm.text != pm.sent {
		if pm.timer != nil {
			pm.timer.Stop()
		}
		delay := f.nextDelay(len(pm.text) - len(pm.sent))
		pm.timer = time.AfterFunc(delay, func() { f.flush(messageID) })
	}
	fresh := pm.markToolStates(parts.ToolCalls)
	f.mu.Unlock()

	for _, c := range fresh {
		f.send(ToolActivity, ToolActivityPayload{SessionID: sessionID, Name: c.Name, Input: c.Input, Finished: c.Finished, ToolCallID: c.ID})
	}
}

func deltaSuffix(previous, current string) string {
	if strings.HasPrefix(current, previous) {
		return current[len(previous):]
	}
	return current
}

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
	seq := pm.nextSeq
	pm.nextSeq++
	payload := SessionDeltaPayload{SessionID: pm.sessionID, MessageID: messageID, Text: pm.text, Append: deltaSuffix(prev, pm.text), Seq: seq}
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
	if rc.SessionID != "" {
		f.emitDeltas(f.drain(rc.SessionID))
	}
	done := SessionDonePayload{SessionID: rc.SessionID, Text: rc.Text, Error: rc.Error, Cancelled: rc.Cancelled}
	if f.callbacks.RunDone != nil {
		f.callbacks.RunDone(done)
	}
	f.send(SessionDone, done)
}

func (f *Forwarder) handlePermission(payload json.RawMessage) {
	var req crushapi.PermissionRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		if f.log != nil {
			f.log.Debug("uievents: failed to decode permission_request", "err", err)
		}
		return
	}
	var expiresAt int64
	if f.callbacks.PermissionPending != nil {
		expiresAt = f.callbacks.PermissionPending(req)
	}
	f.send(PermissionRequest, PermissionRequestPayload{Request: req, ExpiresAt: expiresAt})
}

func (f *Forwarder) handleFile(payload json.RawMessage) {
	var file crushapi.File
	if err := json.Unmarshal(payload, &file); err != nil {
		if f.log != nil {
			f.log.Debug("uievents: failed to decode file event", "err", err)
		}
		return
	}
	if file.SessionID == "" {
		return
	}
	f.send(ChangesUpdated, ChangesUpdatedPayload{SessionID: file.SessionID, Path: file.Path})
}

func (f *Forwarder) send(name string, data any) {
	if f.emit != nil {
		f.emit(name, data)
	}
}

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
			seq := pm.nextSeq
			pm.nextSeq++
			out = append(out, SessionDeltaPayload{SessionID: pm.sessionID, MessageID: id, Text: pm.text, Append: deltaSuffix(pm.sent, pm.text), Seq: seq})
		}
		delete(f.pending, id)
	}
	f.mu.Unlock()
	return out
}

func (f *Forwarder) emitDeltas(deltas []SessionDeltaPayload) {
	for _, d := range deltas {
		f.send(SessionDelta, d)
	}
}
