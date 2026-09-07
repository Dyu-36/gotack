package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Dyu-36/gotack/internal/attachments"
	"github.com/Dyu-36/gotack/internal/crushapi"
	providerdomain "github.com/Dyu-36/gotack/internal/provider"
)

type SessionInfo struct {
	ID           string  `json:"id"`
	Title        string  `json:"title"`
	MessageCount int64   `json:"message_count"`
	Cost         float64 `json:"cost"`
	UpdatedAt    int64   `json:"updated_at"`
	IsBusy       bool    `json:"is_busy"`
}

type MessageInfo struct {
	ID          string           `json:"id"`
	Role        string           `json:"role"`
	Text        string           `json:"text"`
	Model       string           `json:"model"`
	Provider    string           `json:"provider"`
	CreatedAt   int64            `json:"created_at"`
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
	a.forgetReflection(id)
	return nil
}

func (a *App) SwitchSession(id string) error {
	return a.setCurrentSession(id)
}

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

func (a *App) isCurrentModelVision(svc *bridgeServices) bool {
	if a.cfg == nil || svc == nil || svc.api == nil || svc.ws == nil {
		return false
	}
	providerID := strings.TrimSpace(a.cfg.Provider)
	modelID := strings.TrimSpace(a.cfg.Model)
	if providerID == "" || modelID == "" {
		return false
	}
	if override, ok := a.cfg.ModelCapabilities[modelID]; ok && override.SupportsVision != nil && !*override.SupportsVision {
		return false
	}
	desc, ok := svc.ws.Current()
	if !ok || desc.WorkspaceID == "" {
		return false
	}
	base := a.ctx
	if base == nil {
		base = context.Background()
	}
	ctx, cancel := context.WithTimeout(base, 10*time.Second)
	defer cancel()
	supportsVision, err := providerdomain.SupportsVision(ctx, svc.api, desc.WorkspaceID, providerID, modelID)
	if err != nil {
		if a.log != nil {
			a.log.Warn("could not resolve model attachment capability; using text fallback", "provider", providerID, "model", modelID, "err", err)
		}
		return false
	}
	return supportsVision
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
	cadenceReady := a.prepareReflectionTurn(id)
	runID, err := svc.sess.SendWithAttachments(a.ctx, id, prompt, prepared)
	if err != nil {
		return "", err
	}
	a.reflectionTurnAccepted(id, cadenceReady)
	return runID, nil
}

func (a *App) CancelPrompt(id string) error {
	svc, err := a.services()
	if err != nil {
		return err
	}
	return svc.sess.Cancel(a.ctx, id)
}

const maxToolInputPreview = 4096

func toMessageInfo(message crushapi.Message) MessageInfo {
	text, refs := attachments.ParseAttachmentBlocks(crushapi.ExtractText(message.Parts))
	info := MessageInfo{
		ID:        message.ID,
		Role:      string(message.Role),
		Text:      text,
		Model:     message.Model,
		Provider:  message.Provider,
		CreatedAt: message.CreatedAt,
	}
	for _, ref := range refs {
		info.Attachments = append(info.Attachments, AttachmentInfo{
			FileName: ref.FileName,
			MimeType: ref.MimeType,
			Size:     ref.Size,
			Path:     ref.Path,
		})
	}
	for _, attachment := range crushapi.ExtractAttachments(message.Parts) {
		content := ""
		if strings.HasPrefix(attachment.MimeType, "image/") {
			content = base64.StdEncoding.EncodeToString(attachment.Content)
		}
		size := len(attachment.Content)
		if stat, err := os.Stat(attachment.FilePath); err == nil {
			size = int(stat.Size())
		}
		info.Attachments = append(info.Attachments, AttachmentInfo{
			FileName: attachments.BaseName(attachment.FileName),
			MimeType: attachment.MimeType,
			Size:     size,
			Content:  content,
			Path:     attachment.FilePath,
		})
	}
	for _, call := range crushapi.ExtractToolCalls(message.Parts) {
		input := string(call.Input)
		if runes := []rune(input); len(runes) > maxToolInputPreview {
			input = string(runes[:maxToolInputPreview]) + "…"
		}
		info.ToolCalls = append(info.ToolCalls, ToolCallInfo{
			ID:       call.ID,
			Name:     call.Name,
			Input:    input,
			Finished: call.Finished,
		})
	}
	return info
}

func decodePromptAttachments(input []PromptAttachment, supportsVision bool) []attachments.Prepared {
	items := make([]attachments.Input, len(input))
	for i, item := range input {
		items[i] = attachments.Input{
			FileName: item.FileName,
			MimeType: item.MimeType,
			Content:  item.Content,
			Path:     item.Path,
		}
	}
	return attachments.PrepareInputs(items, supportsVision)
}

func toSessionInfo(session crushapi.Session) SessionInfo {
	return SessionInfo{
		ID:           session.ID,
		Title:        session.Title,
		MessageCount: session.MessageCount,
		Cost:         session.Cost,
		UpdatedAt:    session.UpdatedAt,
		IsBusy:       session.IsBusy,
	}
}

func toSessionInfos(input []crushapi.Session) []SessionInfo {
	out := make([]SessionInfo, len(input))
	for i, session := range input {
		out[i] = toSessionInfo(session)
	}
	return out
}
