package main

import (
	"github.com/Dyu-36/gotack/internal/crushapi"
)

// bind_session.go -- role: Wails-bound API for sessions and prompts.
//

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

// ListSessions lists sessions of the active workspace.

// setCurrentSessionBestEffort marks sessionID as this client's active
// session in the engine. It needs an attached workspace stream; failures are
// logged and ignored because presence is advisory, never load-bearing.
func (a *App) setCurrentSessionBestEffort(sessionID string) {
	a.mu.RLock()
	api := a.api
	ws := a.ws
	a.mu.RUnlock()
	if api == nil || ws == nil {
		return
	}
	desc, ok := ws.Current()
	if !ok {
		return
	}
	if err := api.SetCurrentSession(a.ctx, desc.WorkspaceID, sessionID); err != nil && a.log != nil {
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
	out := make([]SessionInfo, len(sessions))
	for i, s := range sessions {
		out[i] = SessionInfo{
			ID:           s.ID,
			Title:        s.Title,
			MessageCount: s.MessageCount,
			Cost:         s.Cost,
			UpdatedAt:    s.UpdatedAt,
			IsBusy:       s.IsBusy,
		}
	}
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
	return SessionInfo{
		ID:        s.ID,
		Title:     s.Title,
		UpdatedAt: s.UpdatedAt,
	}, nil
}

// SwitchSession sets the active session in the engine.
func (a *App) SwitchSession(id string) error {
	a.setCurrentSessionBestEffort(id)
	return nil
}

// SessionMessages replays history for the first render. Text parts are
// concatenated; tool activity is delivered live via tool:activity instead.
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

// SendPrompt starts an agent turn and returns the run ID used to correlate
// the terminal session:done event.
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
