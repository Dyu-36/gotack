package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/guard"
	"github.com/Dyu-36/gotack/internal/userstrings"
	"github.com/Dyu-36/gotack/internal/zalo"
)

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

	if err := guard.MarkUnattendedSession(filepath.Join(appconfig.Dir(), guard.UnattendedRosterFileName), sessionID); err != nil {
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

func (a *App) zaloCurrentModel(context.Context) (string, error) {
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

func (a *App) resetZaloSessions() {
	if a.zalo == nil {
		return
	}
	if err := a.zalo.ResetSessions(); err != nil && a.log != nil {
		a.log.Warn("zalo session reset failed", "err", err)
	}
}
