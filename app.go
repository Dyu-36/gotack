package main

import (
	"context"
	"log/slog"
	"strings"
	"sync"

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
type App struct {
	ctx context.Context

	mu    sync.RWMutex
	cfg   *appconfig.Config
	log   *slog.Logger
	sup   *engine.Supervisor
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

	zaloChats   map[string]string // zalo chat id -> agent session id
	zaloCancel  context.CancelFunc
	cancelStream context.CancelFunc
}

func NewApp() *App {
	return &App{
		status:    engine.StatusStopped,
		perms:     permission.NewRelay(permissionTTL),
		zaloChats: make(map[string]string),
	}
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
	// Constructing the service performs no I/O; PTYs remain lazy until the UI
	// explicitly opens the terminal panel.
	a.term = terminal.New(a.log, a.emit)

	a.setStatus(engine.StatusStopped)
	if cfg.AutostartEngine {
		a.tryConnect()
	}
}

// shutdown tears down every resource owned by the desktop host. Adopted Crush
// servers are deliberately left running; only a child launched by this
// Supervisor can be terminated by Supervisor.Stop.
func (a *App) shutdown(ctx context.Context) {
	a.mu.Lock()
	cancel := a.cancelStream
	sup := a.sup
	term := a.term
	cfg := a.cfg
	a.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	a.stopZaloBridge()
	if term != nil {
		term.CloseAll()
	}
	if sup != nil {
		_ = sup.Stop() // adopted servers are never killed
	}
	if cfg != nil {
		_ = appconfig.Save(cfg)
	}
}

// startZaloBridge launches the Zalo poll loop when the bridge is enabled and
// the engine is attached. The bridge survives engine restarts on its own
// context; Starter reports unavailability while the engine is down.
func (a *App) startZaloBridge() {
	a.mu.Lock()
	cfg := a.cfg
	alreadyRunning := a.zaloCancel != nil
	a.mu.Unlock()

	if alreadyRunning || cfg == nil || !cfg.Zalo.Enabled || strings.TrimSpace(cfg.Zalo.Token) == "" {
		return
	}
	if a.ctx == nil {
		return
	}

	bridge := zalo.NewBridge(cfg.Zalo.Token, cfg.Zalo.AllowedChats, a.startZaloRequest, a.log)
	ctx, cancel := context.WithCancel(context.Background())

	a.mu.Lock()
	if a.zaloCancel != nil { // raced with a concurrent start
		a.mu.Unlock()
		cancel()
		return
	}
	a.zalo = bridge
	a.zaloCancel = cancel
	a.mu.Unlock()

	go func() {
		if err := bridge.Run(ctx); err != nil && ctx.Err() == nil {
			a.log.Warn("zalo bridge stopped", "err", err)
		}
	}()
}

// stopZaloBridge cancels the poll loop and drops the bridge reference.
func (a *App) stopZaloBridge() {
	a.mu.Lock()
	cancel := a.zaloCancel
	a.zalo = nil
	a.zaloCancel = nil
	a.mu.Unlock()
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

	a.mu.Lock()
	sessionID, reused := a.zaloChats[chatID]
	a.mu.Unlock()

	if !reused {
		sess, err := svc.sess.Create(ctx, "Zalo: "+senderName)
		if err != nil {
			return "", err
		}
		sessionID = sess.ID
		a.mu.Lock()
		a.zaloChats[chatID] = sessionID
		a.mu.Unlock()
	}
	if _, err := svc.sess.Send(ctx, sessionID, text); err != nil {
		if reused {
			a.mu.Lock()
			delete(a.zaloChats, chatID)
			a.mu.Unlock()
		}
		return "", err
	}
	return sessionID, nil
}

// RunDone implements uievents.DoneSink: completed agent runs are forwarded to
// the bridge, which ignores sessions it did not start.
func (a *App) RunDone(done uievents.SessionDonePayload) {
	a.mu.RLock()
	bridge := a.zalo
	a.mu.RUnlock()
	if bridge != nil {
		bridge.Done(done.SessionID, done.Text)
	}
}

// resetZaloSessions drops chat-to-session mappings; sessions belong to one
// workspace, so they must not leak across workspace switches.
func (a *App) resetZaloSessions() {
	a.mu.Lock()
	a.zaloChats = make(map[string]string)
	a.mu.Unlock()
}
