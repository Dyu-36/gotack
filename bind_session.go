package main

import (
	"github.com/Dyu-36/gotack/internal/crushapi"
)

// bind_session.go -- role: Wails-bound API for sessions and prompts.

// SessionInfo is the JSON shape of a session row for the sidebar.
type SessionInfo struct {
	ID           string  `json:"id"`
	Title        string  `json:"title"`
	MessageCount int64   `json:"message_count"`
	Cost         float64 `json:"cost"`
	UpdatedAt    int64   `json:"updated_at"`
	IsBusy       bool    `json:"is_busy"`
}

// MessageInfo is the JSON shape of a replayed history message.
type MessageInfo struct {
	ID        string `json:"id"`
	Role      string `json:"role"`
	Text      string `json:"text"`
	CreatedAt int64  `json:"created_at"`
}

// setCurrentSessionBestEffort marks sessionID as this client's active session
// in the engine. Presence is advisory, never load-bearing.
func (a *App) setCurrentSessionBestEffort(sessionID string) {
	c := a.getConn()
	if c == nil || c.api == nil || c.ws == nil {
		return
	}
	desc, ok := c.ws.Current()
	if !ok {
		return
	}
	if err := c.api.SetCurrentSession(a.ctx, desc.WorkspaceID, sessionID); err != nil && a.log != nil {
		a.log.Debug("current-session update skipped", "session", sessionID, "err", err)
	}
}

// ListSessions lists sessions of the active workspace.
func (a *App) ListSessions() ([]SessionInfo, error) {
	svc, err := a.services()
	if err != nil {
		return nil, err
	}
	sessions, err := svc.sess.List(a.ctx)
	if err != nil {
		return nil, err
	}
	out := toSessionInfos(sessions)
	return out, nil
}

// CreateSession starts a new session and makes it the current one.
func (a *App) CreateSession(title string) (SessionInfo, error) {
	svc, err := a.services()
	if err != nil {
		return SessionInfo{}, err
	}
	s, err := svc.sess.Create(a.ctx, title)
	if err != nil {
		return SessionInfo{}, err
	}
	a.setCurrentSessionBestEffort(s.ID)
	return toSessionInfo(s), nil
}

// RenameSession persists a title change in Crush and returns the refreshed row.
func (a *App) RenameSession(id, title string) (SessionInfo, error) {
	svc, err := a.services()
	if err != nil {
		return SessionInfo{}, err
	}
	s, err := svc.sess.Rename(a.ctx, id, title)
	if err != nil {
		return SessionInfo{}, err
	}
	return toSessionInfo(s), nil
}

// DeleteSession removes the session from Crush. If it was current, the UI
// selects another session and calls SwitchSession afterwards.
func (a *App) DeleteSession(id string) error {
	svc, err := a.services()
	if err != nil {
		return err
	}
	return svc.sess.Delete(a.ctx, id)
}

// SwitchSession sets the active session in the engine.
func (a *App) SwitchSession(id string) error {
	a.setCurrentSessionBestEffort(id)
	return nil
}

// SessionMessages replays history for the first render. Text parts are
// concatenated; live tool activity arrives via tool:activity.
func (a *App) SessionMessages(id string) ([]MessageInfo, error) {
	svc, err := a.services()
	if err != nil {
		return nil, err
	}
	msgs, err := svc.sess.Messages(a.ctx, id)
	if err != nil {
		return nil, err
	}
	out := make([]MessageInfo, len(msgs))
	for i, m := range msgs {
		out[i] = MessageInfo{
			ID:        m.ID,
			Role:      string(m.Role),
			Text:      crushapi.ExtractText(m.Parts),
			CreatedAt: m.CreatedAt,
		}
	}
	a.setCurrentSessionBestEffort(id)
	return out, nil
}

// SendPrompt starts an agent turn and returns the run ID.
func (a *App) SendPrompt(id, text string) (string, error) {
	svc, err := a.services()
	if err != nil {
		return "", err
	}
	return svc.sess.Send(a.ctx, id, text)
}

// CancelPrompt interrupts the running turn.
func (a *App) CancelPrompt(id string) error {
	svc, err := a.services()
	if err != nil {
		return err
	}
	return svc.sess.Cancel(a.ctx, id)
}

// toSessionInfo maps a Crush session row to the Wails-bound shape.
func toSessionInfo(s crushapi.Session) SessionInfo {
	return SessionInfo{
		ID:           s.ID,
		Title:        s.Title,
		MessageCount: s.MessageCount,
		Cost:         s.Cost,
		UpdatedAt:    s.UpdatedAt,
		IsBusy:       s.IsBusy,
	}
}

// toSessionInfos maps a slice of Crush session rows to the Wails-bound shape.
func toSessionInfos(in []crushapi.Session) []SessionInfo {
	out := make([]SessionInfo, len(in))
	for i, s := range in {
		out[i] = toSessionInfo(s)
	}
	return out
}
