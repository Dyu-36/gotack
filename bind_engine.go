package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/Dyu-36/gotack/internal/changes"
	"github.com/Dyu-36/gotack/internal/crushapi"
	"github.com/Dyu-36/gotack/internal/enginelink"
	"github.com/Dyu-36/gotack/internal/permission"
	"github.com/Dyu-36/gotack/internal/session"
	"github.com/Dyu-36/gotack/internal/uievents"
	"github.com/Dyu-36/gotack/internal/workspace"
)

// bind_engine.go -- role: Wails-bound API for the Crush engine lifecycle.
//
// The connection state machine (status transitions, attach scope, dial
// sequence, event-stream scopes) lives in internal/enginelink; these bound
// methods stay thin wrappers between it and the UI. Start/stop/reconnect
// return an immediate snapshot and do the long work in the background,
// reporting progress through the engine:status event.

// The forwarder drains the SSE stream; keep the compile-time proof that it
// still satisfies the enginelink consumer seam.
var _ enginelink.EventConsumer = (*uievents.Forwarder)(nil)

// EngineInfo is the JSON shape of every engine status report.
type EngineInfo struct {
	Status   string `json:"status"` // stopped | starting | running | error
	Running  bool   `json:"running"`
	Endpoint string `json:"endpoint"`
	Version  string `json:"version"`
	Owned    bool   `json:"owned"`
	Error    string `json:"error,omitempty"`
}

// engineInfo reports the current engine state from the link and the
// supervisor. Read-only: every caller reads a post-transition snapshot.
func (a *App) engineInfo() EngineInfo {
	info := EngineInfo{
		Status: string(enginelink.StatusStopped),
	}
	if a.link != nil {
		status := a.link.Status()
		info.Status = string(status)
		info.Running = status == enginelink.StatusRunning
		info.Endpoint = a.link.Endpoint().Address
		info.Version = a.link.Version()
		info.Error = a.link.LastError()
	}
	if a.sup != nil {
		info.Owned = a.sup.Owned()
	}
	return info
}

// EngineStatus returns the current engine state without side effects.
func (a *App) EngineStatus() EngineInfo {
	return a.engineInfo()
}

// StartEngine attaches to a running server or launches one, then reports the
// outcome asynchronously via engine:status. Already running and already
// connecting are both no-ops.
func (a *App) StartEngine() EngineInfo {
	a.tryConnect()
	return a.EngineStatus()
}

// StopEngine stops the transport and kills the child process only when this
// host launched it. Disconnect owns the status transition, so there is no
// second status write here that could disagree with it.
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
	scope, started := a.link.BeginConnect(context.Background())
	if !started {
		return false
	}
	a.emit(uievents.EngineStatus, a.engineInfo())
	go a.connect(scope)
	return true
}

// connect performs the full attach sequence for one attach scope. Dial and
// handshake live in enginelink; the ready hook commits the services,
// activates the default workspace, and only then promotes the link to
// running.
func (a *App) connect(scope context.Context) {
	err := a.link.Connect(scope, func(ctx context.Context, api *crushapi.Client, ep crushapi.Endpoint, version string) error {
		fwd := uievents.NewForwarder(a.log, a.emit, a.permsFromConn(), a, a)
		ws := workspace.NewService(api)
		sess := session.NewService(api, ws)
		diffs := changes.NewService(api, ws)

		if !a.commitAttach(ctx, api, fwd, ws, sess, diffs, ep, version) {
			return enginelink.ErrAttachSuperseded
		}

		// Gotack always attaches a default workspace before reporting the engine as
		// running. On Windows that workspace is C:\, while Crush persistence stays
		// under Gotack's app-data directory. Users can chat immediately without
		// selecting a folder, and all workspaces run with permission prompts off.
		svc := &bridgeServices{api: api, ws: ws, sess: sess, diffs: diffs}
		// Runs before the workspace attach so the engine builds its agent from the
		// provider that owns the ChatGPT credential after the OpenAI/Codex split.
		a.migrateChatGPTProviderCredential(svc)
		workspaceWarning := ""
		if _, err := a.activateAssistantWorkspace(svc); err != nil {
			// Attaching the default workspace must not decide whether the engine
			// counts as reachable. Crush rejects workspace creation when a stored
			// provider credential is unusable, and Settings -- the only place to
			// repair it -- stays out of reach while a failed attach keeps the link
			// in error. Report the engine as running and carry the reason instead;
			// EnsureAssistantWorkspace retries from the UI once it is fixed.
			a.log.Warn("could not attach the default workspace", "err", err)
			workspaceWarning = fmt.Sprintf("initialize assistant workspace: %v", err)
		}

		a.log.Info("engine connected", "endpoint", ep.Address, "version", version, "owned", a.sup.Owned())
		a.link.MarkRunning()
		status := a.engineInfo()
		if status.Error == "" {
			status.Error = workspaceWarning
		}
		a.emit(uievents.EngineStatus, status)
		// Readiness is pushed, never polled: overdue scheduled runs
		// re-evaluate immediately after a (re)connect.
		a.setSchedulerReady(true)
		a.reapplySavedWorkspaceSettings()
		if a.zalo != nil && a.zalo.Status().Configured {
			a.zalo.Start()
		}
		return nil
	})
	switch {
	case err == nil:
	case errors.Is(err, enginelink.ErrAttachSuperseded):
		// A newer attach scope owns the state machine now; it reports.
	default:
		a.failConnect(err.Error())
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
		stillCurrent = true
		return c
	})
	if !stillCurrent {
		return false
	}
	return a.link.CommitAttach(ctx, ep, version)
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

func (a *App) failConnect(reason string) {
	if a.log != nil {
		a.log.Error("engine connect failed", "reason", reason)
	}
	a.link.Fail(reason)
	a.emit(uievents.EngineStatus, a.engineInfo())
}

// transportLost marks a live transport unhealthy only if the attach scope is
// still current. Intentional Stop/Reconnect cancels the scope first and
// therefore never gets misreported as an error.
func (a *App) transportLost(scope context.Context, reason string) {
	if !a.link.TransportLost(scope, reason) {
		return
	}
	a.setSchedulerReady(false)
	if a.log != nil {
		a.log.Warn("engine transport lost", "reason", reason)
	}
	a.emit(uievents.EngineStatus, a.engineInfo())
}

// attachStream subscribes to the workspace SSE channel and forwards it to
// the UI. One stream per active workspace. An unexpected stream close is
// promoted to engine:error so the frontend backoff loop can reconnect
// instead of silently freezing with the last token on screen.
func (a *App) attachStream(scope context.Context, workspaceID string) error {
	c := a.getConn()
	if c == nil || c.api == nil || c.fwd == nil {
		return enginelink.ErrTransportNotWired
	}
	return enginelink.AttachStream(scope, c.api, c.fwd, workspaceID, a.transportLost)
}

func (a *App) startStream(scope context.Context, workspaceID string) {
	if err := a.attachStream(scope, workspaceID); err != nil {
		a.transportLost(scope, err.Error())
	}
}

// replaceWorkspaceStream synchronously installs a fresh SSE subscription for
// the active workspace. Crush considers a client attached only while this
// stream is live; session-presence calls return 409 otherwise. Keeping the
// attach synchronous guarantees callers can safely retry SetCurrentSession.
// Unlike the activation paths (see rebindWorkspaceRuntime), an attach
// failure is returned to the caller and the fresh scope is rolled back.
func (a *App) replaceWorkspaceStream(workspaceID string) error {
	if workspaceID == "" {
		return enginelink.ErrWorkspaceIDRequired
	}
	if a.getConn() == nil {
		return enginelink.ErrNoConnection
	}

	scope := a.link.ReplaceStreamScope(a.ctx)
	if err := a.attachStream(scope, workspaceID); err != nil {
		a.link.CancelScope()
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
	var fwd *uievents.Forwarder
	a.swapConn(func(c *conn) *conn {
		fwd = c.fwd
		c.api = nil
		c.fwd = nil
		c.ws = nil
		c.sess = nil
		c.diffs = nil
		return c
	})
	a.link.Disconnect()
	a.setSchedulerReady(false)
	if fwd != nil {
		fwd.Stop()
	}
	if a.zalo != nil {
		a.zalo.Stop()
	}
	a.emit(uievents.EngineStatus, a.engineInfo())
}

type bridgeServices struct {
	api   *crushapi.Client
	ws    *workspace.Service
	sess  *session.Service
	diffs *changes.Service
}

func (a *App) services() (*bridgeServices, error) {
	c := a.getConn()
	if c == nil || c.api == nil || c.ws == nil || c.sess == nil || a.link.Status() != enginelink.StatusRunning {
		return nil, errors.New("engine is not running")
	}
	return &bridgeServices{api: c.api, ws: c.ws, sess: c.sess, diffs: c.diffs}, nil
}
