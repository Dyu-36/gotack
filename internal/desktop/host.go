// Package desktop coordinates the engine connection and desktop services.
// Platform UI calls and product settings are supplied by the application adapter.
package desktop

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/Dyu-36/gotack/internal/engine"
	"github.com/Dyu-36/gotack/internal/engineapi"
	"github.com/Dyu-36/gotack/internal/session"
	"github.com/Dyu-36/gotack/internal/uievents"
	"github.com/Dyu-36/gotack/internal/workspace"
)

// Connection is a snapshot of the services belonging to one engine attachment.
// Change snapshots through Host.Update rather than mutating a published value.
type Connection struct {
	API       *engineapi.Client
	Forwarder *uievents.Forwarder
	Workspace *workspace.Service
	Session   *session.Service
}

type EngineInfo struct {
	Status   string `json:"status"`
	Running  bool   `json:"running"`
	Endpoint string `json:"endpoint"`
	Version  string `json:"version"`
	Owned    bool   `json:"owned"`
	Error    string `json:"error,omitempty"`
}

// Hooks keep product policy and Wails runtime calls outside the coordinator.
type Hooks struct {
	Prepare    func(*Connection)
	Initialize func(*Connection) string
	Ready      func()
	Disconnect func()
}

type Host struct {
	Link    *engine.Link
	Log     *slog.Logger
	Emit    func(string, any)
	Owned   func() bool
	RunDone func(uievents.SessionDonePayload)
	Hooks   Hooks

	connection atomic.Pointer[Connection]
	attachMu   sync.Mutex
}

func New() *Host {
	h := &Host{Link: engine.NewLink(nil), Log: slog.Default()}
	h.connection.Store(&Connection{})
	return h
}

func (h *Host) Get() *Connection { return h.connection.Load() }

func (h *Host) Update(mutate func(*Connection) *Connection) *Connection {
	for {
		cur := h.connection.Load()
		next := &Connection{}
		if cur != nil {
			*next = *cur
		}
		updated := mutate(next)
		if h.connection.CompareAndSwap(cur, updated) {
			return updated
		}
	}
}

func (h *Host) Info() EngineInfo {
	info := EngineInfo{Status: string(engine.StatusStopped)}
	if h.Link != nil {
		status := h.Link.Status()
		info.Status = string(status)
		info.Running = status == engine.StatusRunning
		info.Endpoint = h.Link.Endpoint().Address
		info.Version = h.Link.Version()
		info.Error = h.Link.LastError()
	}
	if h.Owned != nil {
		info.Owned = h.Owned()
	}
	return info
}

func (h *Host) emitStatus(warning string) {
	if h.Emit == nil {
		return
	}
	info := h.Info()
	if info.Error == "" {
		info.Error = warning
	}
	h.Emit(uievents.EngineStatus, info)
}

func (h *Host) TryConnect() bool {
	scope, started := h.Link.BeginConnect(context.Background())
	if !started {
		return false
	}
	h.emitStatus("")
	go h.Connect(scope)
	return true
}

func (h *Host) Connect(scope context.Context) {
	err := h.Link.Connect(scope, h.attach)
	if err != nil && !errors.Is(err, engine.ErrAttachSuperseded) {
		h.FailConnect(scope, err.Error())
	}
}

func (h *Host) attach(ctx context.Context, api *engineapi.Client, ep engineapi.Endpoint, version string) error {
	ws := workspace.NewService(api)
	attached := &Connection{
		API: api, Workspace: ws, Session: session.NewService(api, ws),
		Forwarder: uievents.NewForwarder(h.Log, h.Emit, uievents.Callbacks{RunDone: h.RunDone}),
	}
	if !h.CommitAttach(ctx, attached, ep, version) {
		return engine.ErrAttachSuperseded
	}
	if h.Hooks.Prepare != nil {
		h.Hooks.Prepare(attached)
	}
	h.attachMu.Lock()
	if !h.Link.IsCurrent(ctx) || h.Get() != attached {
		h.attachMu.Unlock()
		return engine.ErrAttachSuperseded
	}
	h.Link.MarkRunning()
	h.attachMu.Unlock()
	warning := ""
	if h.Hooks.Initialize != nil {
		warning = h.Hooks.Initialize(attached)
	}
	// Initializing a workspace replaces the handshake scope with its event
	// stream scope. Check attachment identity, not the cancelled handshake.
	if h.Get() != attached || h.Link.Status() != engine.StatusRunning {
		return engine.ErrAttachSuperseded
	}
	h.Log.Info("engine connected", "endpoint", ep.Address, "version", version, "owned", h.Info().Owned)
	h.emitStatus(warning)
	if h.Hooks.Ready != nil {
		h.Hooks.Ready()
	}
	return nil
}

func (h *Host) CommitAttach(ctx context.Context, connection *Connection, ep engineapi.Endpoint, version string) bool {
	h.attachMu.Lock()
	defer h.attachMu.Unlock()
	if h.Get() == nil || !h.Link.IsCurrent(ctx) || !h.Link.CommitAttach(ctx, ep, version) {
		return false
	}
	h.connection.Store(connection)
	return true
}

func (h *Host) FailConnect(scope context.Context, reason string) {
	if !h.Link.IsCurrent(scope) {
		return
	}
	h.Log.Error("engine connect failed", "reason", reason)
	h.Link.Fail(reason)
	h.emitStatus("")
}

func (h *Host) TransportLost(scope context.Context, reason string) {
	if !h.Link.TransportLost(scope, reason) {
		return
	}
	h.Log.Warn("engine transport lost", "reason", reason)
	h.emitStatus("")
}

func (h *Host) AttachStream(scope context.Context, workspaceID string) error {
	c := h.Get()
	if c == nil || c.API == nil || c.Forwarder == nil {
		return engine.ErrTransportNotWired
	}
	return engine.AttachStream(scope, c.API, c.Forwarder, workspaceID, h.TransportLost)
}

func (h *Host) StartStream(scope context.Context, workspaceID string) {
	if err := h.AttachStream(scope, workspaceID); err != nil {
		h.TransportLost(scope, err.Error())
	}
}

func (h *Host) ReplaceWorkspaceStream(ctx context.Context, workspaceID string) error {
	if workspaceID == "" {
		return engine.ErrWorkspaceIDRequired
	}
	if h.Get() == nil {
		return engine.ErrNoConnection
	}
	scope := h.Link.ReplaceStreamScope(ctx)
	if err := h.AttachStream(scope, workspaceID); err != nil {
		h.Link.CancelScope()
		return err
	}
	return nil
}

func (h *Host) Disconnect() {
	h.attachMu.Lock()
	defer h.attachMu.Unlock()
	if h.Hooks.Disconnect != nil {
		h.Hooks.Disconnect()
	}
	c := h.Get()
	if c == nil {
		return
	}
	h.connection.Store(&Connection{})
	h.Link.Disconnect()
	if c.Forwarder != nil {
		c.Forwarder.Stop()
	}
	h.emitStatus("")
}

func (h *Host) Close() {
	h.attachMu.Lock()
	defer h.attachMu.Unlock()
	h.Link.CancelScope()
	c := h.connection.Swap(nil)
	if c != nil && c.Forwarder != nil {
		c.Forwarder.Stop()
	}
}

func (h *Host) Services() (*Connection, error) {
	c := h.Get()
	if c == nil || c.API == nil || c.Workspace == nil || c.Session == nil || h.Link.Status() != engine.StatusRunning {
		return nil, errors.New("engine is not running")
	}
	return c, nil
}
