package main

import (
	"context"
	"log/slog"
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
)

// app.go -- role: Wails application object, lifecycle and service wiring.
//
// App is the only struct bound into the UI, reachable as window.go.main.App.*
// It must stay in package main because frontend/src/platform/desktop.ts targets
// the main binding namespace.
//
// App owns wiring only. Behaviour lives in internal/:
//
//	internal/engine    Crush server process lifecycle
//	internal/crushapi  REST + SSE client to the Crush server
//	internal/uievents  engine events forwarded to the UI
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

	ep        crushapi.Endpoint
	version   string
	lastError string
	status    engine.Status

	cancelStream context.CancelFunc
}

// NewApp wires the parts that need nothing from the runtime. The permission
// relay is one of them, so it belongs here rather than in startup: a.perms is
// then non-nil for the whole life of the App and no caller needs a nil case.
func NewApp() *App {
	return &App{
		status: engine.StatusStopped,
		perms:  permission.NewRelay(permissionTTL),
	}
}

// startup runs once the Wails runtime is ready.
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
	a.term = terminal.New(a.log, a.emit)

	a.setStatus(engine.StatusStopped)
	if cfg.AutostartEngine {
		a.tryConnect()
	}
}

// shutdown tears down owned resources when the window closes.
func (a *App) shutdown(ctx context.Context) {
	a.mu.Lock()
	cancel := a.cancelStream
	sup := a.sup
	cfg := a.cfg
	a.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if sup != nil {
		_ = sup.Stop() // adopted servers are never killed
	}
	if cfg != nil {
		_ = appconfig.Save(cfg)
	}
}
