package main

import (
	"errors"
	"fmt"

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
	Model     string `json:"model"`
	Provider  string `json:"provider"`
	CreatedAt int64  `json:"created_at"`
}

// setCurrentSession marks sessionID as this client's active session. Crush
// requires a live workspace SSE subscription for presence; if that stream was
// lost, reattach it synchronously and retry once so the next prompt still has
// a live event path back to the UI.
func (a *App) setCurrentSession(sessionID string) error {
	if sessionID == "" {
		return errors.New("session id is required")
	}
	c := a.getConn()
	if c == nil || c.api == nil || c.ws == nil {
		return errors.New("engine services unavailable")
	}
	desc, ok := c.ws.Current()
	if !ok {
		return errors.New("workspace not selected")
	}
	if err := c.api.SetCurrentSession(a.ctx, desc.WorkspaceID, sessionID); err == nil {
		return nil
	} else if !crushapi.IsClientNotAttached(err) {
		return err
	}

	if err := a.replaceWorkspaceStream(desc.WorkspaceID); err != nil {
		return fmt.Errorf("reattach workspace event stream: %w", err)
	}
	c = a.getConn()
	if c == nil || c.api == nil {
		return errors.New("engine services unavailable after event stream reattach")
	}
	if err := c.api.SetCurrentSession(a.ctx, desc.WorkspaceID, sessionID); err != nil {
		return fmt.Errorf("set current session after event stream reattach: %w", err)
	}
	return nil
}

func (a *App) setCurrentSessionBestEffort(sessionID string) {
	if err := a.setCurrentSession(sessionID); err != nil && a.log != nil {
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

// SwitchSession sets the active session in the engine and guarantees the
// workspace event stream is attached before returning.
func (a *App) SwitchSession(id string) error {
	return a.setCurrentSession(id)
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
		out[i] = toMessageInfo(m)
	}
	a.setCurrentSessionBestEffort(id)
	return out, nil
}

// SendPrompt starts an agent turn and returns the run ID. Reassert session
// presence first so a stale/lost SSE attachment cannot accept the POST while
// leaving the UI with no message or run-complete events.
func (a *App) SendPrompt(id, text string) (string, error) {
	svc, err := a.services()
	if err != nil {
		return "", err
	}
	if err := a.setCurrentSession(id); err != nil {
		return "", fmt.Errorf("prepare prompt event stream: %w", err)
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

func toMessageInfo(m crushapi.Message) MessageInfo {
	return MessageInfo{
		ID:        m.ID,
		Role:      string(m.Role),
		Text:      crushapi.ExtractText(m.Parts),
		Model:     m.Model,
		Provider:  m.Provider,
		CreatedAt: m.CreatedAt,
	}
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
