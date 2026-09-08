package main

import (
	"context"
	"log/slog"
	"path/filepath"
	"sync/atomic"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/attachments"
	"github.com/Dyu-36/gotack/internal/changes"
	"github.com/Dyu-36/gotack/internal/contextseed"
	"github.com/Dyu-36/gotack/internal/engine"
	"github.com/Dyu-36/gotack/internal/engineapi"
	"github.com/Dyu-36/gotack/internal/logging"
	"github.com/Dyu-36/gotack/internal/permission"
	"github.com/Dyu-36/gotack/internal/reflection"
	"github.com/Dyu-36/gotack/internal/runmetrics"
	"github.com/Dyu-36/gotack/internal/schedule"
	"github.com/Dyu-36/gotack/internal/session"
	"github.com/Dyu-36/gotack/internal/terminal"
	"github.com/Dyu-36/gotack/internal/uievents"
	"github.com/Dyu-36/gotack/internal/workspace"
	workspaceconfig "github.com/Dyu-36/gotack/internal/workspaceconfig"
	"github.com/Dyu-36/gotack/internal/zalo"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type conn struct {
	api   *engineapi.Client
	fwd   *uievents.Forwarder
	ws    *workspace.Service
	sess  *session.Service
	perms *permission.Relay
	diffs *changes.Service
	term  *terminal.Service
}

type engineController interface {
	Owned() bool
	Stop() error
}

type App struct {
	ctx context.Context

	cfg *appconfig.Config
	log *slog.Logger

	sup  engineController
	link *engine.Link

	zalo          *zalo.Manager
	officeSeeder  *officeSeeder
	contextSeeder *contextseed.Seeder

	contextRegistrar *contextseed.Registrar
	workspaceRuntime *workspaceconfig.Manager

	scheduler  *schedule.Scheduler
	reflection *reflection.Tracker
	runMetrics *runmetrics.Writer

	conn atomic.Pointer[conn]
}

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

func (a *App) getConn() *conn {
	return a.conn.Load()
}

func NewApp() *App {
	a := &App{}
	a.link = engine.NewLink(nil)
	a.conn.Store(&conn{
		perms: permission.NewRelay(permission.DefaultTTL),
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
	if err != nil {
		a.log = slog.Default()
		a.log.Error("logging setup failed", "err", err)
	} else {
		a.log = logger
	}
	sup := engine.NewSupervisor(a.log, cfg.EngineBinary)
	a.sup = sup
	a.runMetrics = runmetrics.New(appconfig.LogDir(), a.log)
	a.link = engine.NewLink(sup)

	a.officeSeeder = newOfficeSeeder(a.log)
	a.ensureOfficeSeed()

	a.contextSeeder = contextseed.New(appconfig.Dir(), a.log)
	a.ensureContextSeed()

	a.zalo = zalo.NewManager(filepath.Join(appconfig.Dir(), "zalo.json"), zalo.Runtime{
		Workspace: a.workspacePath,
	}, a.log)

	//lint:ignore SA1019 legacy Zalo config migration remains supported until Gotack v1.0.
	if err := a.zalo.ImportLegacy(cfg.Zalo.Token, cfg.Zalo.AllowedChats); err != nil && a.log != nil {
		a.log.Warn("zalo legacy import failed", "err", err)
	}
	a.wireZaloRuntime()

	a.startScheduler()
	a.startReflection()

	a.swapConn(func(c *conn) *conn {
		c.term = terminal.New(a.log, a.emit)
		return c
	})

	a.registerFileDrop()
	go attachments.PruneCache()

	a.tryConnect()
	if a.zalo.Status().Configured {
		a.zalo.Start()
	}
	startTray(a)
}

// showMainWindow surfaces the main window from the tray. It runs on tray or
// second-instance goroutines, so it only calls the Wails runtime, which
// marshals into the main thread's message loop.
func (a *App) showMainWindow() {
	if a.ctx != nil {
		wailsruntime.WindowShow(a.ctx)
	}
}

func (a *App) shutdown(ctx context.Context) {
	c := a.getConn()
	if c == nil {
		return
	}

	a.stopReflection(ctx)
	a.link.CancelScope()
	a.releaseAllContextLeases()
	a.stopScheduler()
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
