package main

// schedule_host.go -- role: host wiring for the Phase 5 scheduler
// (internal/schedule). This phase adds no UI surface, so there are no
// Wails-bound methods here: hard rule 8 forbids bound methods nothing
// consumes. The scheduler starts and stops with the app, fires only over
// the existing REST session seam, learns engine readiness from the
// connection flow (never by polling), and consumes run completions from
// the run_complete SSE event via App.RunDone.

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/guard"
	"github.com/Dyu-36/gotack/internal/schedule"
)

// startScheduler constructs and starts the scheduled-run scheduler. Job
// definitions live in schedule.json under appconfig.Dir(); the desktop
// never executes agent logic itself, it only launches runs through the
// same REST path the UI uses (ADR 0001).
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

// stopScheduler ends the tick loop and waits for it to exit.
func (a *App) stopScheduler() {
	if a.scheduler != nil {
		a.scheduler.Stop()
	}
}

// setSchedulerReady pushes engine availability to the scheduler: true when
// the connection flow commits, false when the transport stops or is lost.
// The scheduler re-evaluates due jobs on the transition, so readiness is
// event-driven instead of polled from the link.
func (a *App) setSchedulerReady(ready bool) {
	if a.scheduler != nil {
		a.scheduler.SetEngineReady(ready)
	}
}

// scheduleCreateSession opens the firing's session through the same service
// the UI uses, inside the currently active workspace.
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

// scheduleMarkUnattended records the session in the guard roster BEFORE the
// prompt is submitted, exactly like startZaloTurn: the guard then denies
// ask-tier operations with a legible reason instead of hanging on a prompt
// nobody can answer (ADR 0002, plan 5.4). A failed mark aborts the firing.
func (a *App) scheduleMarkUnattended(_ context.Context, sessionID string) error {
	return guard.MarkUnattendedSession(
		filepath.Join(appconfig.Dir(), guard.UnattendedRosterFileName), sessionID)
}

// scheduleSendPrompt submits the job prompt through the same service the UI
// uses; the returned run id belongs to the engine.
func (a *App) scheduleSendPrompt(ctx context.Context, sessionID, prompt string) (string, error) {
	svc, err := a.services()
	if err != nil {
		return "", err
	}
	return svc.sess.Send(ctx, sessionID, prompt)
}

// schedulePreflight skips a firing instead of burning a failed run when no
// model is configured (plan 5.1), mirroring the requirement
// zaloCurrentModel applies to remote turns.
func (a *App) schedulePreflight(context.Context) error {
	if a.cfg == nil || a.cfg.Model == "" {
		return errors.New("no model configured")
	}
	return nil
}
