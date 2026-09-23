package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/Dyu-36/gotack/internal/attachments"
	"github.com/Dyu-36/gotack/internal/engineapi"
)

type SessionInfo struct {
	ID              string  `json:"id"`
	ParentSessionID string  `json:"parent_session_id,omitempty"`
	Title           string  `json:"title"`
	MessageCount    int64   `json:"message_count"`
	Cost            float64 `json:"cost"`
	UpdatedAt       int64   `json:"updated_at"`
	IsBusy          bool    `json:"is_busy"`
}

type MessageInfo struct {
	ID          string           `json:"id"`
	Role        string           `json:"role"`
	Text        string           `json:"text"`
	Model       string           `json:"model"`
	Provider    string           `json:"provider"`
	CreatedAt   int64            `json:"created_at"`
	CompletedAt int64            `json:"completed_at,omitempty"`
	Attachments []AttachmentInfo `json:"attachments,omitempty"`
	ToolCalls   []ToolCallInfo   `json:"tool_calls,omitempty"`
}

type ToolCallInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Input    string `json:"input,omitempty"`
	Finished bool   `json:"finished"`
}

type PromptAttachment struct {
	FileName string `json:"file_name"`
	MimeType string `json:"mime_type,omitempty"`
	Content  string `json:"content,omitempty"`
	Path     string `json:"path,omitempty"`
}

type AttachmentInfo struct {
	FileName string `json:"file_name"`
	MimeType string `json:"mime_type"`
	Size     int    `json:"size"`
	Content  string `json:"content,omitempty"`
	Path     string `json:"path,omitempty"`
}


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
	} else if !engineapi.IsClientNotAttached(err) {
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

func (a *App) ListSessions() ([]SessionInfo, error) {
	svc, err := a.services()
	if err != nil {
		return nil, err
	}
	sessions, err := svc.sess.List(a.ctx)
	if err != nil {
		return nil, err
	}
	return toSessionInfos(sessions), nil
}

func (a *App) CreateSession(title string) (SessionInfo, error) {
	svc, err := a.services()
	if err != nil {
		return SessionInfo{}, err
	}
	session, err := svc.sess.Create(a.ctx, title)
	if err != nil {
		return SessionInfo{}, err
	}
	a.setCurrentSessionBestEffort(session.ID)
	return toSessionInfo(session), nil
}

func (a *App) CloneSession(id string) (SessionInfo, error) {
	svc, err := a.services()
	if err != nil {
		return SessionInfo{}, err
	}
	session, err := svc.sess.Clone(a.ctx, id)
	if err != nil {
		return SessionInfo{}, err
	}
	a.setCurrentSessionBestEffort(session.ID)
	return toSessionInfo(session), nil
}

func (a *App) ForkSession(id, messageID string) (SessionInfo, error) {
	svc, err := a.services()
	if err != nil {
		return SessionInfo{}, err
	}
	session, err := svc.sess.Fork(a.ctx, id, messageID)
	if err != nil {
		return SessionInfo{}, err
	}
	a.setCurrentSessionBestEffort(session.ID)
	return toSessionInfo(session), nil
}

func (a *App) CompactSession(id string) error {
	svc, err := a.services()
	if err != nil {
		return err
	}
	return svc.sess.Compact(a.ctx, id)
}

func (a *App) RenameSession(id, title string) (SessionInfo, error) {
	svc, err := a.services()
	if err != nil {
		return SessionInfo{}, err
	}
	session, err := svc.sess.Rename(a.ctx, id, title)
	if err != nil {
		return SessionInfo{}, err
	}
	return toSessionInfo(session), nil
}

func (a *App) DeleteSession(id string) error {
	svc, err := a.services()
	if err != nil {
		return err
	}
	if err := svc.sess.Delete(a.ctx, id); err != nil {
		return err
	}
	return nil
}

func (a *App) SwitchSession(id string) error { return a.setCurrentSession(id) }

func (a *App) SessionMessages(id string) ([]MessageInfo, error) {
	svc, err := a.services()
	if err != nil {
		return nil, err
	}
	messages, err := svc.sess.Messages(a.ctx, id)
	if err != nil {
		return nil, err
	}
	out := make([]MessageInfo, len(messages))
	for i, message := range messages {
		out[i] = toMessageInfo(message)
	}
	a.setCurrentSessionBestEffort(id)
	return out, nil
}


func (a *App) SendPrompt(id, text string, input []PromptAttachment) (string, error) {
	svc, err := a.services()
	if err != nil {
		return "", err
	}
	if err := a.setCurrentSession(id); err != nil {
		return "", fmt.Errorf("prepare prompt event stream: %w", err)
	}
	supportsVision := false
	prompt, tagged := attachments.FileTags(text)
	if len(input) > 0 || len(tagged) > 0 {
		supportsVision = a.isCurrentModelVision(svc)
	}

	prepared := decodePromptAttachments(input, supportsVision)
	for _, path := range tagged {
		item, prepErr := attachments.PrepareFile(path, supportsVision)
		if prepErr != nil {
			prepared = append(prepared, attachments.Failed(filepath.Base(path), prepErr.Error()))
			continue
		}
		prepared = append(prepared, item)
	}
	runID, err := svc.sess.SendWithAttachments(a.ctx, id, prompt, prepared)
	if err != nil {
		return "", err
	}
	return runID, nil
}

func (a *App) CancelPrompt(id string) error {
	svc, err := a.services()
	if err != nil {
		return err
	}
	return svc.sess.Cancel(a.ctx, id)
}



func toSessionInfo(session engineapi.Session) SessionInfo {
	return SessionInfo{
		ID:              session.ID,
		ParentSessionID: session.ParentSessionID,
		Title:           session.Title,
		MessageCount:    session.MessageCount,
		Cost:            session.Cost,
		UpdatedAt:       engineapi.TimestampMillis(session.UpdatedAt),
		IsBusy:          session.IsBusy,
	}
}

func toSessionInfos(input []engineapi.Session) []SessionInfo {
	out := make([]SessionInfo, len(input))
	for i, session := range input {
		out[i] = toSessionInfo(session)
	}
	return out
}
