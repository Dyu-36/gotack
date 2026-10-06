package main

import (
	"context"
	"log/slog"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/attachments"
	"github.com/Dyu-36/gotack/internal/desktop"
	"github.com/Dyu-36/gotack/internal/engine"
	"github.com/Dyu-36/gotack/internal/logging"
	"github.com/Dyu-36/gotack/internal/projecttrust"
	workspaceconfig "github.com/Dyu-36/gotack/internal/workspaceconfig"
	"github.com/Dyu-36/gotack/internal/zalo"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type conn = desktop.Connection

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
	host *desktop.Host

	zalo         *zalo.Manager
	projectTrust *projecttrust.Store

	workspaceRuntime     *workspaceconfig.Manager
	workspaceRuntimeOnce sync.Once

	vision sync.Map

	oauthMu     sync.Mutex
	oauthCancel context.CancelFunc
	oauthURL    string
}

func (a *App) swapConn(mutate func(*conn) *conn) *conn { return a.host.Update(mutate) }

func (a *App) getConn() *conn { return a.host.Get() }

func NewApp() *App {
	a := &App{host: desktop.New()}
	a.host.Emit = a.emit
	a.host.Owned = func() bool { return a.sup != nil && a.sup.Owned() }
	a.host.RunDone = a.runDone
	a.host.Hooks = desktop.Hooks{
		Prepare:    a.migrateChatGPTProviderCredential,
		Initialize: a.initializeEngineWorkspace,
		Ready:      func() { a.reapplySavedWorkspaceSettings(); a.startZaloIfEnabled() },
		Disconnect: func() {
			if a.zalo != nil {
				a.zalo.Stop()
			}
			a.vision.Clear()
		},
	}
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
	a.host.Link = engine.NewLink(sup, expectedEngineCommit(cfg.EngineBinary))
	a.host.Log = a.log

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
		a.host.Link.CancelScope()
		if a.zalo != nil {
			a.zalo.Stop()
		}
		a.host.Close()
		if a.sup != nil && a.sup.Owned() {
			if err := a.sup.Stop(); err != nil && a.log != nil {
				a.log.Error("stop owned engine on exit", "err", err)
			}
		}
		if a.cfg != nil {
			if err := appconfig.Save(a.cfg); err != nil && a.log != nil {
				a.log.Error("save desktop settings on exit", "err", err)
			}
		}
	})
}
