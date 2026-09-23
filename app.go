package main

import (
	"context"
	"log/slog"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/attachments"
	"github.com/Dyu-36/gotack/internal/changes"
	"github.com/Dyu-36/gotack/internal/engine"
	"github.com/Dyu-36/gotack/internal/engineapi"
	"github.com/Dyu-36/gotack/internal/logging"
	"github.com/Dyu-36/gotack/internal/projecttrust"
	"github.com/Dyu-36/gotack/internal/session"
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
	diffs *changes.Service
}

type engineController interface {
	Owned() bool
	Stop() error
}

type App struct {
	quitting     atomic.Bool
	shutdownOnce sync.Once
	ctx          context.Context

	cfg *appconfig.Config
	log *slog.Logger

	sup  engineController
	link *engine.Link

	zalo         *zalo.Manager
	projectTrust *projecttrust.Store

	workspaceRuntime     *workspaceconfig.Manager
	workspaceRuntimeOnce sync.Once

	vision sync.Map
	conn   atomic.Pointer[conn]
	attachMu sync.Mutex

	oauthMu     sync.Mutex
	oauthCancel context.CancelFunc
	oauthURL    string
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

func (a *App) getConn() *conn { return a.conn.Load() }

func NewApp() *App {
	a := &App{link: engine.NewLink(nil)}
	a.conn.Store(&conn{})
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
	a.link = engine.NewLink(sup, expectedEngineCommit(cfg.EngineBinary))

	a.projectTrust = projecttrust.New(filepath.Join(appconfig.Dir(), "project-trust.json"))
	a.zalo = zalo.NewManager(filepath.Join(appconfig.Dir(), "zalo.json"), zalo.Runtime{
		Workspace: a.workspacePath,
	}, a.log)

	a.wireZaloRuntime()

	a.registerFileDrop()
	go attachments.PruneCache()

	a.tryConnect()
	a.startZaloIfEnabled()
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
	a.shutdownOnce.Do(func() {
		if a.link != nil {
			a.link.CancelScope()
		}
		if a.zalo != nil {
			a.zalo.Stop()
		}
		if c := a.getConn(); c != nil && c.fwd != nil {
			c.fwd.Stop()
		}
		if a.sup != nil && a.sup.Owned() {
			if err := a.sup.Stop(); err != nil && a.log != nil {
				a.log.Error("stop owned engine on exit", "err", err)
			}
		}
		a.conn.Store(nil)
		if a.cfg != nil {
			if err := appconfig.Save(a.cfg); err != nil && a.log != nil {
				a.log.Error("save desktop settings on exit", "err", err)
			}
		}
	})
}
