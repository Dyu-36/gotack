package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/Dyu-36/gotack/internal/userstrings"
	"github.com/Dyu-36/gotack/internal/zalo"
)

func (a *App) wireZaloRuntime() {
	if a.zalo == nil {
		return
	}
	a.zalo.SetRuntime(zalo.Runtime{
		Prepare:   a.prepareZaloTurn,
		Run:       a.runZaloTurn,
		Stop:      a.stopZaloTurn,
		Session:   a.zaloSessionTitle,
		Model:     a.zaloCurrentModel,
		Workspace: a.workspacePath,
	})
}

func (a *App) prepareZaloTurn(ctx context.Context, existingSession, chatID string) (zalo.Turn, error) {
	svc, err := a.services()
	if err != nil {
		return zalo.Turn{}, err
	}
	workspace, ok := svc.ws.Current()
	if !ok {
		return zalo.Turn{}, errors.New("no workspace is open")
	}
	turn := zalo.Turn{SessionID: existingSession, WorkspaceID: workspace.WorkspaceID, WorkspacePath: workspace.Path}
	if turn.SessionID == "" {
		created, err := svc.api.CreateSession(ctx, turn.WorkspaceID, "Zalo: "+chatID)
		if err != nil {
			return zalo.Turn{}, err
		}
		turn.SessionID = created.ID
	} else if _, err := svc.api.GetSession(ctx, turn.WorkspaceID, turn.SessionID); err != nil {
		return zalo.Turn{}, err
	}
	return turn, nil
}

func (a *App) runZaloTurn(ctx context.Context, turn zalo.Turn, text string) error {
	svc, err := a.services()
	if err != nil {
		return err
	}
	return svc.api.SendPromptWithPurpose(ctx, turn.WorkspaceID, turn.SessionID, text, turn.ID, "zalo", nil)
}

func (a *App) stopZaloTurn(ctx context.Context, turn zalo.Turn) error {
	connection := a.getConn()
	if connection == nil || connection.api == nil {
		return errors.New("engine connection is unavailable")
	}
	return connection.api.CancelPrompt(ctx, turn.WorkspaceID, turn.SessionID)
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
