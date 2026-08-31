package main

// reflection_host.go -- role: host wiring for the Phase 6 reflection loop
// (internal/reflection). This phase adds no UI surface, so there are no
// Wails-bound methods here: hard rule 8 forbids bound methods nothing
// consumes. The tracker fires only over the existing REST session seam,
// consumes run completions from the run_complete SSE event via App.RunDone,
// and is triggered from session deletion (the only session-end signal the
// host has). Reflection orchestration stays inside internal/reflection; the
// desktop layer only starts bounded engine runs (ADR 0001).

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/guard"
	"github.com/Dyu-36/gotack/internal/reflection"
)

// startReflection constructs the reflection tracker. It starts no goroutine
// of its own: gates open on run_complete and session deletion, and the host
// launches the bounded run when one opens.
func (a *App) startReflection() {
	a.reflection = reflection.New(reflection.Runtime{
		CreateSession:  a.reflectionCreateSession,
		MarkUnattended: a.reflectionMarkUnattended,
		SendPrompt:     a.reflectionSendPrompt,
		Preflight:      a.reflectionPreflight,
	}, a.log)
}

// reflectionCreateSession opens the reflection session through the same
// service the UI uses, inside the currently active workspace.
func (a *App) reflectionCreateSession(ctx context.Context, title string) (string, error) {
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

// reflectionMarkUnattended records the reflection session in the guard
// roster BEFORE the prompt is submitted, exactly like scheduled and Zalo
// runs: ask-tier operations are denied with a legible reason instead of
// hanging on a prompt nobody can answer (ADR 0002). A failed mark aborts
// the firing inside the tracker.
func (a *App) reflectionMarkUnattended(_ context.Context, sessionID string) error {
	return guard.MarkUnattendedSession(
		filepath.Join(appconfig.Dir(), guard.UnattendedRosterFileName), sessionID)
}

// reflectionSendPrompt submits the reflection prompt through the same
// service the UI uses; the returned run id belongs to the engine.
func (a *App) reflectionSendPrompt(ctx context.Context, sessionID, prompt string) (string, error) {
	svc, err := a.services()
	if err != nil {
		return "", err
	}
	return svc.sess.Send(ctx, sessionID, prompt)
}

// reflectionPreflight skips a firing instead of burning a failed run when
// no model is configured, mirroring the scheduler's preflight.
func (a *App) reflectionPreflight(context.Context) error {
	if a.cfg == nil || a.cfg.Model == "" {
		return errors.New("no model configured")
	}
	return nil
}

// triggerReflection starts a bounded reflection run for sourceSessionID in
// the background after a gate opened on run_complete. A refused or failed
// launch is logged, never surfaced: the reflection loop must not disturb
// the event stream that feeds it.
func (a *App) triggerReflection(sourceSessionID string) {
	if a.reflection == nil {
		return
	}
	go func() {
		if err := a.reflection.Fire(a.ctx, sourceSessionID); err != nil && a.log != nil {
			a.log.Debug("reflection run not started", "session", sourceSessionID, "err", err)
		}
	}()
}

// sessionEnded is the session-end gate: it fires synchronously so the
// reflection prompt is submitted while the source session still exists, and
// a refused or failed launch never blocks the delete that follows.
func (a *App) sessionEnded(sessionID string) {
	if a.reflection == nil || !a.reflection.SessionEnded(sessionID) {
		return
	}
	ctx, cancel := context.WithTimeout(a.ctx, 15*time.Second)
	defer cancel()
	if err := a.reflection.Fire(ctx, sessionID); err != nil && a.log != nil {
		a.log.Debug("reflection run not started", "session", sessionID, "err", err)
	}
}
