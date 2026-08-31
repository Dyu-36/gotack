package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Dyu-36/gotack/internal/attachments"
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
	ID          string           `json:"id"`
	Role        string           `json:"role"`
	Text        string           `json:"text"`
	Model       string           `json:"model"`
	Provider    string           `json:"provider"`
	CreatedAt   int64            `json:"created_at"`
	Attachments []AttachmentInfo `json:"attachments,omitempty"`
}

// PromptAttachment is the JSON shape accepted from the composer. Content is
// base64 so Wails does not need to coerce browser ArrayBuffers into Go bytes.
type PromptAttachment struct {
	FileName string `json:"file_name"`
	MimeType string `json:"mime_type,omitempty"`
	Content  string `json:"content"`
}

// AttachmentInfo is attachment metadata returned with replayed messages.
// Image content is returned as base64 so the UI can restore its preview.
type AttachmentInfo struct {
	FileName string `json:"file_name"`
	MimeType string `json:"mime_type"`
	Size     int    `json:"size"`
	Content  string `json:"content,omitempty"`
}

const maxPromptAttachmentSize = 5 * 1024 * 1024

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
// concatenated, binary attachments are included, and live tool activity
// arrives via tool:activity.
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

func (a *App) isCurrentModelVision() bool {
	if a.cfg == nil {
		return true
	}
	modelID := strings.TrimSpace(a.cfg.Model)
	if modelID == "" {
		return true
	}
	if a.cfg.ModelCapabilities != nil {
		if override, ok := a.cfg.ModelCapabilities[modelID]; ok && override.SupportsVision != nil {
			return *override.SupportsVision
		}
	}
	return crushapi.InferModelVision(a.cfg.Provider, modelID)
}

// SendPrompt starts an agent turn and returns the run ID. Reassert session
// presence first so a stale/lost SSE attachment cannot accept the POST while
// leaving the UI with no message or run-complete events.
func (a *App) SendPrompt(id, text string, input []PromptAttachment) (string, error) {
	svc, err := a.services()
	if err != nil {
		return "", err
	}
	if err := a.setCurrentSession(id); err != nil {
		return "", fmt.Errorf("prepare prompt event stream: %w", err)
	}
	supportsVision := a.isCurrentModelVision()
	attachments, err := decodePromptAttachments(input, supportsVision)
	if err != nil {
		return "", err
	}
	return svc.sess.SendWithAttachments(a.ctx, id, text, attachments)
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
	info := MessageInfo{
		ID:        m.ID,
		Role:      string(m.Role),
		Text:      crushapi.ExtractText(m.Parts),
		Model:     m.Model,
		Provider:  m.Provider,
		CreatedAt: m.CreatedAt,
	}
	for _, attachment := range crushapi.ExtractAttachments(m.Parts) {
		content := ""
		if strings.HasPrefix(attachment.MimeType, "image/") {
			content = base64.StdEncoding.EncodeToString(attachment.Content)
		}
		info.Attachments = append(info.Attachments, AttachmentInfo{
			FileName: filepath.Base(attachment.FileName),
			MimeType: attachment.MimeType,
			Size:     len(attachment.Content),
			Content:  content,
		})
	}
	return info
}

func decodePromptAttachments(input []PromptAttachment, supportsVision bool) ([]crushapi.Attachment, error) {
	out := make([]crushapi.Attachment, 0, len(input))
	for i, item := range input {
		name := filepath.Base(strings.TrimSpace(item.FileName))
		if name == "" || name == "." {
			return nil, fmt.Errorf("attachment %d: file name is required", i+1)
		}
		if len(item.Content) > base64.StdEncoding.EncodedLen(maxPromptAttachmentSize) {
			return nil, fmt.Errorf("attachment %q exceeds the 5 MB limit", name)
		}
		content, err := base64.StdEncoding.DecodeString(item.Content)
		if err != nil {
			return nil, fmt.Errorf("attachment %q has invalid content: %w", name, err)
		}
		if len(content) > maxPromptAttachmentSize {
			return nil, fmt.Errorf("attachment %q exceeds the 5 MB limit", name)
		}
		att, err := attachments.ProcessWithModel(name, item.MimeType, content, supportsVision)
		if err != nil {
			return nil, fmt.Errorf("attachment %q: %w", name, err)
		}
		out = append(out, att)
	}
	return out, nil
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
