package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Dyu-36/gotack/internal/changes"
	"github.com/Dyu-36/gotack/internal/crushapi"
	"github.com/Dyu-36/gotack/internal/engine"
	"github.com/Dyu-36/gotack/internal/session"
	"github.com/Dyu-36/gotack/internal/uievents"
	"github.com/Dyu-36/gotack/internal/workspace"
)

// bind_engine.go -- role: Wails-bound API for the Crush engine lifecycle.
//
// Start/stop/reconnect return an immediate snapshot and do the long work in
// the background, reporting progress through the engine:status event.

const handshakeTimeout = 15 * time.Second

// EngineInfo is the JSON shape of every engine status report.
type EngineInfo struct {
	Status   string `json:"status"` // stopped | starting | running | error
	Running  bool   `json:"running"`
	Endpoint string `json:"endpoint"`
	Version  string `json:"version"`
	Owned    bool   `json:"owned"`
	Error    string `json:"error,omitempty"`
}

func (a *App) engineInfoLocked() EngineInfo {
	info := EngineInfo{
		Status:   string(a.status),
		Running:  a.status == engine.StatusRunning,
		Endpoint: a.ep.Address,
		Version:  a.version,
		Error:    a.lastError,
	}
	if a.sup != nil {
		info.Owned = a.sup.Owned()
	}
	return info
}

// EngineStatus returns the current engine state without side effects.
func (a *App) EngineStatus() EngineInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.engineInfoLocked()
}

// StartEngine attaches to a running server or launches one, then reports the
// outcome asynchronously via engine:status. Already running and already
// connecting are both no-ops.
func (a *App) StartEngine() EngineInfo {
	a.tryConnect()
	return a.EngineStatus()
}

// StopEngine stops the transport and kills the child process only when this
// host launched it. stopTransport owns the status transition, so there is no
// second setStatus here that could disagree with it.
func (a *App) StopEngine() EngineInfo {
	a.stopTransport()
	if a.sup != nil {
		_ = a.sup.Stop() // adopted servers are never terminated
	}
	return a.EngineStatus()
}

// ReconnectEngine re-dials after transport loss without killing the agent.
// Dropping the transport first is what lets it reuse tryConnect unchanged:
// the engine reads as stopped by then, so reconnect needs no special case.
func (a *App) ReconnectEngine() error {
	a.stopTransport()
	if !a.tryConnect() {
		return errors.New("engine connect already in progress")
	}
	return nil
}

// tryConnect claims the attach slot and runs the attach in the background. The
// claim and the StatusStarting publication happen under a single lock, so two
// fast clicks can no longer launch two attach goroutines, and with them two
// engines and two event streams.
//
// The context created here is the one cancellation scope for the whole attempt:
// locate, dial, handshake and the event stream that follows. Storing it as
// a.cancelStream is what makes stopTransport cancel the attach itself rather
// than only the stream. Previously connect ran on context.Background(), so a
// Stop during the handshake let the attach run to completion and then
// overwrite the status back to running.
func (a *App) tryConnect() bool {
	a.mu.Lock()
	if a.status == engine.StatusRunning || a.status == engine.StatusStarting {
		a.mu.Unlock()
		return false
	}
	prev := a.cancelStream
	ctx, cancel := context.WithCancel(context.Background())
	a.cancelStream = cancel
	a.status = engine.StatusStarting
	info := a.engineInfoLocked()
	a.mu.Unlock()

	if prev != nil {
		prev()
	}
	a.emit(uievents.EngineStatus, info)
	go a.connect(ctx)
	return true
}

// connect performs the full attach sequence for one attach scope:
// locate or launch -> dial -> handshake -> build services -> stream events.
func (a *App) connect(ctx context.Context) {
	ep, found := a.sup.Locate(ctx)
	if !found {
		var err error
		// Not ctx-bound: the child must outlive this attach attempt, so its
		// lifetime is Supervisor.Stop's business.
		ep, err = a.sup.Start()
		if err != nil {
			a.failConnect(fmt.Sprintf("launch engine: %v", err))
			return
		}
	}

	hc, err := crushapi.Dial(ctx, ep)
	if err != nil {
		a.failConnect(fmt.Sprintf("dial %s %s: %v", ep.Network, ep.Address, err))
		return
	}

	api := crushapi.NewClient(hc)
	if err := engine.WaitForHealthy(ctx, api, handshakeTimeout); err != nil {
		a.failConnect(fmt.Sprintf("handshake: %v", err))
		return
	}
	vi, err := api.Version(ctx)
	if err != nil {
		a.failConnect(fmt.Sprintf("version: %v", err))
		return
	}

	fwd := uievents.NewForwarder(a.log, a.emit, a.perms)
	ws := workspace.NewService(api)
	sess := session.NewService(api, ws)
	diffs := changes.NewService(api, ws)

	a.mu.Lock()
	// The attach scope may have been cancelled while we were dialling. Publish
	// nothing in that case: stopTransport has already reported the stop, and
	// overwriting it with running is precisely the stale-status bug.
	if ctx.Err() != nil {
		a.mu.Unlock()
		return
	}
	a.api = api
	a.fwd = fwd
	a.ws = ws
	a.sess = sess
	a.diffs = diffs
	a.ep = ep
	a.version = vi.Version
	a.mu.Unlock()

	// Re-attach the previous workspace so its sessions survive UI restarts.
	if desc, ok := ws.Current(); ok {
		a.startStream(ctx, desc.WorkspaceID)
	}

	a.log.Info("engine connected", "endpoint", ep.Address, "version", vi.Version, "owned", a.sup.Owned())
	a.setStatus(engine.StatusRunning)
}

func (a *App) failConnect(msg string) {
	a.log.Error("engine connect failed", "reason", msg)
	a.mu.Lock()
	a.lastError = msg
	a.status = engine.StatusError
	info := a.engineInfoLocked()
	a.mu.Unlock()
	a.emit(uievents.EngineStatus, info)
}

// startStream subscribes to the workspace SSE channel and forwards it to the
// UI. One stream per active workspace.
func (a *App) startStream(ctx context.Context, workspaceID string) {
	events, stop, err := a.api.Stream(ctx, workspaceID)
	if err != nil {
		a.log.Error("stream attach failed", "workspace", workspaceID, "err", err)
		return
	}
	go a.fwd.Consume(events)
	go func() {
		<-ctx.Done()
		stop()
	}()
}

// stopTransport drops the wired services, cancels the attach scope and then
// reports the engine as stopped. Owning the status transition here is the
// reason ReconnectEngine needs no special case, and it removes the old
// StopEngine two-step where the status could be published separately from the
// teardown that justified it.
func (a *App) stopTransport() {
	a.mu.Lock()
	cancel := a.cancelStream
	a.cancelStream = nil
	fwd := a.fwd
	a.fwd = nil
	a.api = nil
	a.ws = nil
	a.sess = nil
	a.diffs = nil
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if fwd != nil {
		fwd.Stop()
	}
	a.setStatus(engine.StatusStopped)
}

// bridgeServices groups the wired services a bound call needs. Nil when the
// engine is not attached; binds translate that into a user-facing error.
type bridgeServices struct {
	api   *crushapi.Client
	ws    *workspace.Service
	sess  *session.Service
	diffs *changes.Service
}

func (a *App) services() (*bridgeServices, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.api == nil || a.ws == nil || a.sess == nil || a.status != engine.StatusRunning {
		return nil, errors.New("engine is not running")
	}
	return &bridgeServices{api: a.api, ws: a.ws, sess: a.sess, diffs: a.diffs}, nil
}
