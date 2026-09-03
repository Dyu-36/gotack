package main

import (
	"context"
	"errors"
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
	"github.com/Dyu-36/gotack/internal/enginelink"
	"github.com/Dyu-36/gotack/internal/guard"
	"github.com/Dyu-36/gotack/internal/logging"
	"github.com/Dyu-36/gotack/internal/permission"
	"github.com/Dyu-36/gotack/internal/reflection"
	"github.com/Dyu-36/gotack/internal/schedule"
	"github.com/Dyu-36/gotack/internal/session"
	"github.com/Dyu-36/gotack/internal/terminal"
	"github.com/Dyu-36/gotack/internal/uievents"
	"github.com/Dyu-36/gotack/internal/userstrings"
	"github.com/Dyu-36/gotack/internal/workspace"
	"github.com/Dyu-36/gotack/internal/zalo"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type conn struct {
	api   *crushapi.Client
	fwd   *uievents.Forwarder
	ws    *workspace.Service
	sess  *session.Service
	perms *permission.Relay
	diffs *changes.Service
	term  *terminal.Service
}

type App struct {
	ctx context.Context

	cfg *appconfig.Config
	log *slog.Logger

	sup engine.EngineAPI

	link *enginelink.Link

	zalo          *zalo.Manager
	officeSeeder  *officeSeeder
	contextSeeder *contextseed.Seeder

	scheduler *schedule.Scheduler

	reflection *reflection.Tracker

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
	a.link = enginelink.NewLink(nil)
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
	if err == nil {
		a.log = logger
	} else {
		a.log = slog.Default()
	}
	a.sup = engine.NewSupervisor(a.log, cfg.EngineBinary)
	a.link = enginelink.NewLink(a.sup)

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
	if a.ctx == nil {
		return
	}
	wailsruntime.WindowShow(a.ctx)
}

func (a *App) shutdown(ctx context.Context) {
	c := a.getConn()
	if c == nil {

		return
	}

	a.stopReflection(ctx)

	a.link.CancelScope()
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

	if err := guard.MarkUnattendedSession(
		filepath.Join(appconfig.Dir(), guard.UnattendedRosterFileName), sessionID); err != nil {
		return "", err
	}
	cadenceReady := a.prepareReflectionTurn(sessionID)
	if _, err := svc.sess.Send(ctx, sessionID, text); err != nil {
		return "", err
	}
	a.reflectionTurnAccepted(sessionID, cadenceReady)
	return sessionID, nil
}

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
		return "", errors.New(userstrings.ErrNoModelSelected)
	}
	if a.cfg.Provider == "" {
		return a.cfg.Model, nil
	}
	return a.cfg.Provider + "/" + a.cfg.Model, nil
}

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

func (a *App) runDone(done uievents.SessionDonePayload) {
	if a.zalo != nil {
		a.zalo.Done(done.SessionID, done.Text)
	}
	scheduled := false
	if a.scheduler != nil {
		scheduled = a.scheduler.RecordOutcome(done.SessionID, done.Error, done.Cancelled)
	}
	if a.reflection != nil {

		if scheduled {
			a.reflection.Forget(done.SessionID)
			return
		}
		review, cleanupID := a.reflection.RunDone(done.SessionID, done.Text, done.Error, done.Cancelled)
		if cleanupID != "" {
			a.cleanupReflection(cleanupID)
		}
		if review.Any() {
			a.triggerReflection(done.SessionID, review)
		}
	}
}

func (a *App) resetZaloSessions() {
	if a.zalo == nil {
		return
	}
	if err := a.zalo.ResetSessions(); err != nil && a.log != nil {
		a.log.Warn("zalo session reset failed", "err", err)
	}
}
