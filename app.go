package main

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync/atomic"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/attachments"
	"github.com/Dyu-36/gotack/internal/changes"
	"github.com/Dyu-36/gotack/internal/contextseed"
	"github.com/Dyu-36/gotack/internal/crushapi"
	"github.com/Dyu-36/gotack/internal/engine"
	"github.com/Dyu-36/gotack/internal/logging"
	"github.com/Dyu-36/gotack/internal/permission"
	"github.com/Dyu-36/gotack/internal/session"
	"github.com/Dyu-36/gotack/internal/terminal"
	"github.com/Dyu-36/gotack/internal/uievents"
	"github.com/Dyu-36/gotack/internal/workspace"
	"github.com/Dyu-36/gotack/internal/zalo"
)

// app.go -- role: Wails application object, lifecycle and service wiring.
// App is the only struct bound into the UI, reachable as window.go.main.App.*
//
// Static configuration (cfg, log, ctx, sup) lives directly on App because it is
// assigned once in startup and read by every bind call. The dynamic connection
// state (api, fwd, ws, sess, perms, diffs, term, ep, version, lastError,
// status, cancelStream) is swapped atomically through a *conn pointer so read
// paths need no lock at all. Zalo state is deliberately NOT part of conn: the
// manager owns its own lifecycle and outlives engine reconnects.

// conn holds the dynamic connection state. The host reads it with
// a.conn.Load() and mutates it through a.swapConn(...), so read paths need no
// mutex; see docs/plans/completed/d1-conn-pointer.md for the rationale.
type conn struct {
	api   *crushapi.Client
	fwd   *uievents.Forwarder
	ws    *workspace.Service
	sess  *session.Service
	perms *permission.Relay
	diffs *changes.Service
	term  *terminal.Service

	ep        crushapi.Endpoint
	version   string
	lastError string
	status    engine.Status

	// cancelStream cancels the current attach scope. The attach context
	// itself is passed to connect()/attachStream() as an argument rather than
	// stored here, so each scope has exactly one owner.
	cancelStream context.CancelFunc
}

type App struct {
	ctx context.Context

	cfg *appconfig.Config
	log *slog.Logger
	// sup is the concrete engine supervisor, typed as the narrow EngineAPI
	// seam so app.go depends on the interface, not the implementation.
	sup engine.EngineAPI

	zalo          *zalo.Manager
	officeSeeder  *officeSeeder
	contextSeeder *contextseed.Seeder

	conn atomic.Pointer[conn]
}

// swapConn copies the current *conn, hands it to mutate, and stores the
// returned pointer back atomically. The mutate callback may also return a
// brand-new pointer when there is no existing conn to copy. After the swap,
// the previous pointer must not be read or written by any other goroutine;
// callers therefore read with getConn() to obtain a fresh pointer for the
// current generation.
func (a *App) swapConn(mutate func(*conn) *conn) *conn {
	for {
		cur := a.conn.Load()
		next := cur
		if next == nil {
			next = &conn{}
		} else {
			clone := *cur
			next = &clone
		}
		updated := mutate(next)
		if a.conn.CompareAndSwap(cur, updated) {
			return updated
		}
	}
}

// getConn returns the current *conn or nil when no connection has been
// initialized. Read paths must guard on a nil result before dereferencing.
func (a *App) getConn() *conn {
	return a.conn.Load()
}

func NewApp() *App {
	a := &App{}
	a.conn.Store(&conn{
		perms:  permission.NewRelay(permissionTTL),
		status: engine.StatusStopped,
	})
	return a
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	cfg, err := appconfig.Load()
	if err != nil {
		cfg = appconfig.Defaults()
	}
	a.cfg = cfg

	logger, err := logging.Setup(appconfig.LogDir(), cfg.Debug)
	if err == nil {
		a.log = logger
	} else {
		a.log = slog.Default()
	}
	a.sup = engine.NewSupervisor(a.log, cfg.EngineBinary)

	a.officeSeeder = newOfficeSeeder(a.log)
	a.ensureOfficeSeed()

	a.contextSeeder = contextseed.New(appconfig.Dir(), a.log)
	a.ensureContextSeed()

	a.zalo = zalo.NewManager(filepath.Join(appconfig.Dir(), "zalo.json"), zalo.Runtime{
		Workspace: a.workspacePath,
	}, a.log)
	if err := a.zalo.ImportLegacy(cfg.Zalo.Token, cfg.Zalo.AllowedChats); err != nil && a.log != nil {
		a.log.Warn("zalo legacy import failed", "err", err)
	}
	a.wireZaloRuntime()

	// The terminal service lives in the conn so a stop/start cycle can
	// re-attach to a fresh engine; constructing it performs no I/O and the
	// PTYs stay lazy until the UI explicitly opens the panel.
	a.swapConn(func(c *conn) *conn {
		c.term = terminal.New(a.log, a.emit)
		return c
	})

	// Dropped files reach the host as absolute paths, never as base64 through
	// the webview; the composer renders chips from the emitted metadata.
	a.registerFileDrop()

	// One-shot trim, not a loop: every send used to copy its upload into the
	// attachment cache and nothing ever removed it again.
	go attachments.PruneCache()

	// The Crush engine is part of Gotack's runtime, not an optional project
	// feature. Start/attach it whenever the desktop app opens so chat, provider
	// config, Zalo, and local tools are available before a folder is selected.
	a.tryConnect()
	if a.zalo.Status().Configured {
		a.zalo.Start()
	}
}

// shutdown tears down resources owned by the desktop host but deliberately
// leaves the engine process running. A later Gotack launch adopts that process,
// avoiding a cold engine, MCP, and LSP startup on every UI restart.
func (a *App) shutdown(ctx context.Context) {
	c := a.getConn()
	if c == nil {
		// nothing to tear down; either startup never finished or the conn
		// was already cleared.
		return
	}
	if c.cancelStream != nil {
		c.cancelStream()
	}
	if a.zalo != nil {
		a.zalo.Stop()
	}
	if c.term != nil {
		c.term.CloseAll()
	}
	if a.cfg != nil {
		_ = appconfig.Save(a.cfg)
	}
}

// wireZaloRuntime installs the agent and chat hooks on the Zalo manager. The
// manager owns the polling loop, the pairing state and the chat-to-session
// map; the host just translates into Crush engine calls.
func (a *App) wireZaloRuntime() {
	if a.zalo == nil {
		return
	}
	a.zalo.SetRuntime(zalo.Runtime{
		Start:     a.startZaloTurn,
		Stop:      a.stopZaloTurn,
		Session:   a.zaloSessionTitle,
		Model:     a.zaloCurrentModel,
		Workspace: a.workspacePath,
	})
}

// startZaloTurn submits one inbound chat message to Crush. The manager decides
// whether to reuse a session; the host simply forwards the text.
func (a *App) startZaloTurn(ctx context.Context, existingSession, chatID, text string) (string, error) {
	svc, err := a.services()
	if err != nil {
		return "", err
	}
	sessionID := existingSession
	if sessionID == "" {
		sess, err := svc.sess.Create(ctx, "Zalo: "+chatID)
		if err != nil {
			return "", err
		}
		sessionID = sess.ID
	}
	if _, err := svc.sess.Send(ctx, sessionID, text); err != nil {
		return "", err
	}
	return sessionID, nil
}

// stopZaloTurn asks Crush to abort the in-flight turn for the chat's session.
func (a *App) stopZaloTurn(ctx context.Context, sessionID string) error {
	svc, err := a.services()
	if err != nil {
		return err
	}
	return svc.sess.Cancel(ctx, sessionID)
}

func (a *App) zaloSessionTitle(ctx context.Context, sessionID string) (string, error) {
	svc, err := a.services()
	if err != nil {
		return "", err
	}
	sessions, err := svc.sess.List(ctx)
	if err != nil {
		return "", err
	}
	for _, candidate := range sessions {
		if candidate.ID == sessionID {
			return candidate.Title, nil
		}
	}
	return "", nil
}

func (a *App) zaloCurrentModel(ctx context.Context) (string, error) {
	if a.cfg == nil {
		return "", fmt.Errorf("desktop config not loaded")
	}
	if a.cfg.Model == "" {
		return "", fmt.Errorf("chưa chọn mô hình")
	}
	if a.cfg.Provider == "" {
		return a.cfg.Model, nil
	}
	return a.cfg.Provider + "/" + a.cfg.Model, nil
}

// workspacePath returns the currently open workspace root, used by the Zalo
// manager to scope /files and outbound file resolution.
func (a *App) workspacePath() string {
	c := a.getConn()
	if c == nil || c.ws == nil {
		return ""
	}
	desc, ok := c.ws.Current()
	if !ok {
		return ""
	}
	return desc.Path
}

// RunDone implements uievents.DoneSink: completed agent runs are routed to the
// Zalo manager, which decides which chat (if any) receives the answer.
func (a *App) RunDone(done uievents.SessionDonePayload) {
	if a.zalo != nil {
		a.zalo.Done(done.SessionID, done.Text)
	}
}

// resetZaloSessions drops chat-to-session mappings; sessions belong to one
// workspace, so they must not leak across workspace switches.
func (a *App) resetZaloSessions() {
	if a.zalo == nil {
		return
	}
	if err := a.zalo.ResetSessions(); err != nil && a.log != nil {
		a.log.Warn("zalo session reset failed", "err", err)
	}
}
