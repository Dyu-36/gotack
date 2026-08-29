package main

import (
	"context"
	"log/slog"
	"strings"
	"sync/atomic"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/changes"
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
// status, cancelStream, zaloChats, zaloCancel, zalo) is swapped atomically
// through a *conn pointer so read paths need no lock at all.

// conn holds the dynamic connection state. The host reads it with
// a.conn.Load() and mutates it through a.swapConn(...); the previous
// sync.RWMutex is gone.
type conn struct {
	api   *crushapi.Client
	fwd   *uievents.Forwarder
	ws    *workspace.Service
	sess  *session.Service
	perms *permission.Relay
	diffs *changes.Service
	term  *terminal.Service
	zalo  *zalo.Bridge

	ep        crushapi.Endpoint
	version   string
	lastError string
	status    engine.Status

	attachCtx context.Context

	zaloChats    map[string]string // zalo chat id -> agent session id
	zaloCancel   context.CancelFunc
	cancelStream context.CancelFunc
}

type App struct {
	ctx context.Context

	cfg *appconfig.Config
	log *slog.Logger
	// sup is the concrete engine supervisor, typed as the narrow EngineAPI
	// seam so app.go depends on the interface, not the implementation.
	sup engine.EngineAPI

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
		perms:     permission.NewRelay(permissionTTL),
		zaloChats: make(map[string]string),
		status:    engine.StatusStopped,
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

	// The terminal service lives in the conn so a stop/start cycle can
	// re-attach to a fresh engine; constructing it performs no I/O and the
	// PTYs stay lazy until the UI explicitly opens the panel.
	a.swapConn(func(c *conn) *conn {
		c.term = terminal.New(a.log, a.emit)
		return c
	})

	if cfg.AutostartEngine {
		a.tryConnect()
	}
}

// shutdown tears down every resource owned by the desktop host. Adopted Crush
// servers are deliberately left running; only a child launched by this
// Supervisor can be terminated by Supervisor.Stop.
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
	a.stopZaloBridge()
	if c.term != nil {
		c.term.CloseAll()
	}
	if a.sup != nil {
		_ = a.sup.Stop() // adopted servers are never killed
	}
	if a.cfg != nil {
		_ = appconfig.Save(a.cfg)
	}
}

// startZaloBridge launches the Zalo poll loop when the bridge is enabled and
// the engine is attached. The bridge survives engine restarts on its own
// context; Starter reports unavailability while the engine is down.
func (a *App) startZaloBridge() {
	c := a.getConn()
	if c == nil {
		return
	}
	if c.zaloCancel != nil {
		return
	}
	if a.cfg == nil || !a.cfg.Zalo.Enabled || strings.TrimSpace(a.cfg.Zalo.Token) == "" {
		return
	}
	if a.ctx == nil {
		return
	}

	bridge := zalo.NewBridge(a.cfg.Zalo.Token, a.cfg.Zalo.AllowedChats, a.startZaloRequest, a.log)
	ctx, cancel := context.WithCancel(context.Background())

	var won bool
	a.swapConn(func(c *conn) *conn {
		if c.zaloCancel != nil { // raced with a concurrent start
			return c
		}
		c.zalo = bridge
		c.zaloCancel = cancel
		won = true
		return c
	})
	if !won {
		// someone else won the race; drop our bridge and context.
		cancel()
		return
	}

	go func() {
		if err := bridge.Run(ctx); err != nil && ctx.Err() == nil {
			a.log.Warn("zalo bridge stopped", "err", err)
		}
	}()
}

// stopZaloBridge cancels the poll loop and drops the bridge reference.
func (a *App) stopZaloBridge() {
	if a.getConn() == nil {
		return
	}
	var cancel context.CancelFunc
	a.swapConn(func(c *conn) *conn {
		cancel = c.zaloCancel
		c.zalo = nil
		c.zaloCancel = nil
		return c
	})
	if cancel != nil {
		cancel()
	}
}

// startZaloRequest maps one inbound Zalo message onto the agent: the chat
// reuses its session so remote conversations keep their context. A failed
// send drops the stale mapping so the next message starts a fresh session.
func (a *App) startZaloRequest(ctx context.Context, chatID, senderName, text string) (string, error) {
	svc, err := a.services()
	if err != nil {
		return "", err
	}

	var sessionID string
	var reused bool
	a.swapConn(func(c *conn) *conn {
		sessionID, reused = c.zaloChats[chatID]
		return c
	})

	if !reused {
		sess, err := svc.sess.Create(ctx, "Zalo: "+senderName)
		if err != nil {
			return "", err
		}
		sessionID = sess.ID
		a.swapConn(func(c *conn) *conn {
			c.zaloChats[chatID] = sessionID
			return c
		})
	}
	if _, err := svc.sess.Send(ctx, sessionID, text); err != nil {
		if reused {
			a.swapConn(func(c *conn) *conn {
				delete(c.zaloChats, chatID)
				return c
			})
		}
		return "", err
	}
	return sessionID, nil
}

// RunDone implements uievents.DoneSink: completed agent runs are forwarded to
// the bridge, which ignores sessions it did not start.
func (a *App) RunDone(done uievents.SessionDonePayload) {
	if a.getConn() == nil {
		return
	}
	var bridge *zalo.Bridge
	a.swapConn(func(c *conn) *conn {
		bridge = c.zalo
		return c
	})
	if bridge != nil {
		bridge.Done(done.SessionID, done.Text)
	}
}

// resetZaloSessions drops chat-to-session mappings; sessions belong to one
// workspace, so they must not leak across workspace switches.
func (a *App) resetZaloSessions() {
	if a.getConn() == nil {
		return
	}
	a.swapConn(func(c *conn) *conn {
		c.zaloChats = make(map[string]string)
		return c
	})
}
