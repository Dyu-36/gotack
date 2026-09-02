package main

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/guard"
	"github.com/Dyu-36/gotack/internal/schedule"
)

func (a *App) startScheduler() {
	a.scheduler = schedule.New(
		filepath.Join(appconfig.Dir(), schedule.FileName),
		schedule.Runtime{
			CreateSession:  a.scheduleCreateSession,
			MarkUnattended: a.scheduleMarkUnattended,
			SendPrompt:     a.scheduleSendPrompt,
			Preflight:      a.schedulePreflight,
		},
		a.log,
	)
	if err := a.scheduler.Start(a.ctx); err != nil && a.log != nil {
		a.log.Error("scheduler start failed", "err", err)
	}
}

func (a *App) stopScheduler() {
	if a.scheduler != nil {
		a.scheduler.Stop()
	}
}

func (a *App) setSchedulerReady(ready bool) {
	if a.scheduler != nil {
		a.scheduler.SetEngineReady(ready)
	}
}

func (a *App) scheduleCreateSession(ctx context.Context, title string) (string, error) {
	svc, err := a.services()
	if err != nil {
		return "", err
	}
	sess, err := svc.sess.Create(ctx, title)
	if err != nil {
		return "", err
	}
	return sess.ID, nil
}

func (a *App) scheduleMarkUnattended(_ context.Context, sessionID string) error {
	return guard.MarkUnattendedSession(
		filepath.Join(appconfig.Dir(), guard.UnattendedRosterFileName), sessionID)
}

func (a *App) scheduleSendPrompt(ctx context.Context, sessionID, prompt string) error {
	svc, err := a.services()
	if err != nil {
		return err
	}
	_, err = svc.sess.Send(ctx, sessionID, prompt)
	return err
}

func (a *App) schedulePreflight(context.Context) error {
	if a.cfg == nil || a.cfg.Model == "" {
		return errors.New("no model configured")
	}
	return nil
}
