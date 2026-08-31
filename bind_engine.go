package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Dyu-36/gotack/internal/changes"
	"github.com/Dyu-36/gotack/internal/crushapi"
	"github.com/Dyu-36/gotack/internal/engine"
	"github.com/Dyu-36/gotack/internal/permission"
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

// engineInfoLocked reports the current engine state from a *conn snapshot.
// Callers must hold either a direct pointer returned by getConn() (read-only)
// or be inside a swapConn mutate closure where they own the copy.
func (a *App) engineInfoLocked() EngineInfo {
	c := a.getConn()
	info := EngineInfo{
		Status: string(engine.StatusStopped),
	}
	if c != nil {
		info.Status = string(c.status)
		info.Running = c.status == engine.StatusRunning
		info.Endpoint = c.ep.Address
		info.Version = c.version
		info.Error = c.lastError
	}
	if a.sup != nil {
		info.Owned = a.sup.Owned()
	}
	return info
}

// EngineStatus returns the current engine state without side effects.
func (a *App) EngineStatus() EngineInfo {
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
	if a.getConn() == nil {
		return false
	}
	var prev context.CancelFunc
	var attachCtx context.Context
	var info EngineInfo
	var started bool
	a.swapConn(func(c *conn) *conn {
		if c.status == engine.StatusRunning || c.status == engine.StatusStarting {
			return c
		}
		prev = c.cancelStream
		ctx, cancel := context.WithCancel(context.Background())
		c.cancelStream = cancel
		c.status = engine.StatusStarting
		c.lastError = ""
		info = a.engineInfoLocked()
		attachCtx = ctx
		started = true
		return c
	})
	if !started {
		return false
	}
	if prev != nil {
		prev()
	}
	a.emit(uievents.EngineStatus, info)
	go a.connect(attachCtx)
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

	fwd := uievents.NewForwarder(a.log, a.emit, a.permsFromConn(), a)
	ws := workspace.NewService(api)
	sess := session.NewService(api, ws)
	diffs := changes.NewService(api, ws)

	if !a.commitAttach(ctx, api, fwd, ws, sess, diffs, ep, vi.Version) {
		return
	}

	// Gotack always attaches a default workspace before reporting the engine as
	// running. On Windows that workspace is C:\, while Crush persistence stays
	// under Gotack's app-data directory. Users can chat immediately without
	// selecting a folder, and all workspaces run with permission prompts off.
	svc := &bridgeServices{api: api, ws: ws, sess: sess, diffs: diffs}
	if _, err := a.activateAssistantWorkspace(svc); err != nil {
		a.failConnect(fmt.Sprintf("initialize assistant workspace: %v", err))
		return
	}

	a.log.Info("engine connected", "endpoint", ep.Address, "version", vi.Version, "owned", a.sup.Owned())
	a.setStatus(engine.StatusRunning)
	a.reapplySavedWorkspaceSettings()
	if a.zalo != nil && a.zalo.Status().Configured {
		a.zalo.Start()
	}
}

// commitAttach writes the freshly built services into the current conn, but
// only if the attach scope is still current. The ctx.Err() guard mirrors the
// original lock-window check: a stopTransport that ran while we were dialing
// must not clobber the new state with services tied to a dead context.
func (a *App) commitAttach(
	ctx context.Context,
	api *crushapi.Client,
	fwd *uievents.Forwarder,
	ws *workspace.Service,
	sess *session.Service,
	diffs *changes.Service,
	ep crushapi.Endpoint,
	version string,
) bool {
	if a.getConn() == nil {
		return false
	}
	var stillCurrent bool
	a.swapConn(func(c *conn) *conn {
		if ctx.Err() != nil {
			return c
		}
		c.api = api
		c.fwd = fwd
		c.ws = ws
		c.sess = sess
		c.diffs = diffs
		c.ep = ep
		c.version = version
		c.lastError = ""
		stillCurrent = true
		return c
	})
	return stillCurrent
}

// permsFromConn returns the permission relay from the current conn. The
// permission relay lives inside the conn pointer because the assignment moves
// the dynamic fields off App; it is built in NewApp and never replaced, so a
// nil conn here only happens before startup completed.
func (a *App) permsFromConn() *permission.Relay {
	if c := a.getConn(); c != nil {
		return c.perms
	}
	return nil
}

func (a *App) failConnect(msg string) {
	if a.log != nil {
		a.log.Error("engine connect failed", "reason", msg)
	}
	if a.getConn() == nil {
		return
	}
	var info EngineInfo
	a.swapConn(func(c *conn) *conn {
		c.lastError = msg
		c.status = engine.StatusError
		info = a.engineInfoLocked()
		return c
	})
	a.emit(uievents.EngineStatus, info)
}

// transportLost marks a live transport unhealthy only if the attach scope is
// still current. Intentional Stop/Reconnect cancels ctx first and therefore
// never gets misreported as an error.
func (a *App) transportLost(ctx context.Context, msg string) {
	if ctx.Err() != nil {
		return
	}
	if a.getConn() == nil {
		return
	}
	var info EngineInfo
	var changed bool
	a.swapConn(func(c *conn) *conn {
		if c.status != engine.StatusRunning {
			return c
		}
		c.status = engine.StatusError
		c.lastError = msg
		info = a.engineInfoLocked()
		changed = true
		return c
	})
	if !changed {
		return
	}
	if a.log != nil {
		a.log.Warn("engine transport lost", "reason", msg)
	}
	a.emit(uievents.EngineStatus, info)
}

// startStream subscribes to the workspace SSE channel and forwards it to the
// UI. One stream per active workspace. An unexpected stream close is promoted
// to engine:error so the frontend backoff loop can reconnect instead of
// silently freezing with the last token on screen.
func (a *App) attachStream(ctx context.Context, workspaceID string) error {
	c := a.getConn()
	if c == nil || c.api == nil || c.fwd == nil {
		return errors.New("event stream unavailable: transport not wired")
	}
	api := c.api
	fwd := c.fwd

	events, stop, err := api.Stream(ctx, workspaceID,
		"message", "run_complete", "permission_request", "question_batch_request", "file",
	)
	if err != nil {
		return fmt.Errorf("event stream attach failed: %w", err)
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
	return nil
}

func (a *App) startStream(ctx context.Context, workspaceID string) {
	if err := a.attachStream(ctx, workspaceID); err != nil {
		a.transportLost(ctx, err.Error())
	}
}

// replaceWorkspaceStream synchronously installs a fresh SSE subscription for
// the active workspace. Crush considers a client attached only while this
// stream is live; session-presence calls return 409 otherwise. Keeping the
// attach synchronous guarantees callers can safely retry SetCurrentSession.
func (a *App) replaceWorkspaceStream(workspaceID string) error {
	if workspaceID == "" {
		return errors.New("workspace id is required for event stream")
	}
	if a.getConn() == nil {
		return errors.New("engine connection unavailable")
	}

	var previous context.CancelFunc
	var streamCtx context.Context
	a.swapConn(func(c *conn) *conn {
		previous = c.cancelStream
		streamCtx, c.cancelStream = context.WithCancel(a.ctx)
		return c
	})
	if previous != nil {
		previous()
	}
	if err := a.attachStream(streamCtx, workspaceID); err != nil {
		if cancel := a.getConn().cancelStream; cancel != nil {
			cancel()
		}
		return err
	}
	return nil
}

// stopTransport drops the wired services, cancels the attach scope and then
// reports the engine as stopped.
func (a *App) stopTransport() {
	if a.getConn() == nil {
		return
	}
	var cancel context.CancelFunc
	var fwd *uievents.Forwarder
	a.swapConn(func(c *conn) *conn {
		cancel = c.cancelStream
		c.cancelStream = nil
		fwd = c.fwd
		c.api = nil
		c.fwd = nil
		c.ws = nil
		c.sess = nil
		c.diffs = nil
		c.ep = crushapi.Endpoint{}
		c.version = ""
		c.status = engine.StatusStopped
		c.lastError = ""
		return c
	})
	if cancel != nil {
		cancel()
	}
	if fwd != nil {
		fwd.Stop()
	}
	if a.zalo != nil {
		a.zalo.Stop()
	}
	a.setStatus(engine.StatusStopped)
}

type bridgeServices struct {
	api   *crushapi.Client
	ws    *workspace.Service
	sess  *session.Service
	diffs *changes.Service
}

func (a *App) services() (*bridgeServices, error) {
	c := a.getConn()
	if c == nil || c.api == nil || c.ws == nil || c.sess == nil || c.status != engine.StatusRunning {
		return nil, errors.New("engine is not running")
	}
	return &bridgeServices{api: c.api, ws: c.ws, sess: c.sess, diffs: c.diffs}, nil
}
