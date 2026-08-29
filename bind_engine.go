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
func (a *App) ReconnectEngine() error {
	a.stopTransport()
	if !a.tryConnect() {
		return errors.New("engine connect already in progress")
	}
	return nil
}

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
	a.lastError = ""
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

	fwd := uievents.NewForwarder(a.log, a.emit, a.perms, a)
	ws := workspace.NewService(api)
	sess := session.NewService(api, ws)
	diffs := changes.NewService(api, ws)

	a.mu.Lock()
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
	a.lastError = ""
	a.mu.Unlock()

	a.log.Info("engine connected", "endpoint", ep.Address, "version", vi.Version, "owned", a.sup.Owned())
	a.setStatus(engine.StatusRunning)
	a.startZaloBridge()
}

func (a *App) failConnect(msg string) {
	if a.log != nil {
		a.log.Error("engine connect failed", "reason", msg)
	}
	a.mu.Lock()
	a.lastError = msg
	a.status = engine.StatusError
	info := a.engineInfoLocked()
	a.mu.Unlock()
	a.emit(uievents.EngineStatus, info)
}

// transportLost marks a live transport unhealthy only if the attach scope is
// still current. Intentional Stop/Reconnect cancels ctx first and therefore
// never gets misreported as an error.
func (a *App) transportLost(ctx context.Context, msg string) {
	if ctx.Err() != nil {
		return
	}
	a.mu.Lock()
	if a.status != engine.StatusRunning {
		a.mu.Unlock()
		return
	}
	a.status = engine.StatusError
	a.lastError = msg
	info := a.engineInfoLocked()
	a.mu.Unlock()
	if a.log != nil {
		a.log.Warn("engine transport lost", "reason", msg)
	}
	a.emit(uievents.EngineStatus, info)
}

// startStream subscribes to the workspace SSE channel and forwards it to the
// UI. One stream per active workspace. An unexpected stream close is promoted
// to engine:error so the frontend backoff loop can reconnect instead of
// silently freezing with the last token on screen.
func (a *App) startStream(ctx context.Context, workspaceID string) {
	a.mu.RLock()
	api := a.api
	fwd := a.fwd
	a.mu.RUnlock()
	if api == nil || fwd == nil {
		a.transportLost(ctx, "event stream unavailable: transport not wired")
		return
	}

	events, stop, err := api.Stream(ctx, workspaceID)
	if err != nil {
		a.transportLost(ctx, fmt.Sprintf("event stream attach failed: %v", err))
		return
	}

	go func() {
		fwd.Consume(events)
		if ctx.Err() == nil {
			a.transportLost(ctx, "engine event stream disconnected")
		}
	}()
	go func() {
		<-ctx.Done()
		stop()
	}()
}

// stopTransport drops the wired services, cancels the attach scope and then
// reports the engine as stopped.
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
	a.stopZaloBridge()
	a.setStatus(engine.StatusStopped)
}

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
