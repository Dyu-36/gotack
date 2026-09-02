package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Dyu-36/gotack/internal/attachments"
	"github.com/Dyu-36/gotack/internal/crushapi"
	"github.com/Dyu-36/gotack/internal/workspace"
	"github.com/google/uuid"
)

// service.go -- role: list, create and switch sessions, send and cancel prompts.
//
// Streaming updates are not returned here, they arrive via internal/uievents.

// defaultTitle is the seed title used when the caller passes an empty string.
// The UI shows this until the user renames the session.
const defaultTitle = "New session"

// Service orchestrates session lifecycle. It owns no durable session state;
// every mutation is forwarded to Crush, which remains the source of truth.
type Service struct {
	api *crushapi.Client
	ws  *workspace.Service
}

// NewService wires a Service. The workspace service is used to resolve the
// current workspace id, so Open must have been called before any session
// method.
func NewService(api *crushapi.Client, ws *workspace.Service) *Service {
	return &Service{api: api, ws: ws}
}

func (s *Service) currentWorkspaceID() (string, error) {
	if s.ws == nil {
		return "", errors.New("workspace service not configured")
	}
	desc, ok := s.ws.Current()
	if !ok || desc.WorkspaceID == "" {
		return "", errors.New("no workspace open")
	}
	return desc.WorkspaceID, nil
}

// List returns all sessions for the current workspace.
func (s *Service) List(ctx context.Context) ([]crushapi.Session, error) {
	wsID, err := s.currentWorkspaceID()
	if err != nil {
		return nil, err
	}
	if s.api == nil {
		return nil, errors.New("engine client not configured")
	}
	return s.api.ListSessions(ctx, wsID)
}

// Create opens a new session under the current workspace.
func (s *Service) Create(ctx context.Context, title string) (crushapi.Session, error) {
	wsID, err := s.currentWorkspaceID()
	if err != nil {
		return crushapi.Session{}, err
	}
	if s.api == nil {
		return crushapi.Session{}, errors.New("engine client not configured")
	}
	if strings.TrimSpace(title) == "" {
		title = defaultTitle
	}
	sess, err := s.api.CreateSession(ctx, wsID, title)
	if err != nil {
		return crushapi.Session{}, fmt.Errorf("create session: %w", err)
	}
	return sess, nil
}

// Rename fetches the current complete session snapshot and persists the title
// with Crush's PUT session endpoint so the change survives UI restarts.
func (s *Service) Rename(ctx context.Context, id, title string) (crushapi.Session, error) {
	wsID, err := s.currentWorkspaceID()
	if err != nil {
		return crushapi.Session{}, err
	}
	if s.api == nil {
		return crushapi.Session{}, errors.New("engine client not configured")
	}
	if id == "" {
		return crushapi.Session{}, errors.New("session id is required")
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return crushapi.Session{}, errors.New("session title is required")
	}
	sess, err := s.api.GetSession(ctx, wsID, id)
	if err != nil {
		return crushapi.Session{}, fmt.Errorf("get session before rename: %w", err)
	}
	sess.Title = title
	saved, err := s.api.SaveSession(ctx, wsID, sess)
	if err != nil {
		return crushapi.Session{}, fmt.Errorf("rename session: %w", err)
	}
	return saved, nil
}

// Delete removes a session from Crush. Messages and file history are deleted
// by the engine's session service in the same transaction.
func (s *Service) Delete(ctx context.Context, id string) error {
	wsID, err := s.currentWorkspaceID()
	if err != nil {
		return err
	}
	if s.api == nil {
		return errors.New("engine client not configured")
	}
	if id == "" {
		return errors.New("session id is required")
	}
	if err := s.api.DeleteSession(ctx, wsID, id); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// Messages returns the full message history for the given session.
func (s *Service) Messages(ctx context.Context, id string) ([]crushapi.Message, error) {
	wsID, err := s.currentWorkspaceID()
	if err != nil {
		return nil, err
	}
	if s.api == nil {
		return nil, errors.New("engine client not configured")
	}
	if id == "" {
		return nil, errors.New("session id is required")
	}
	return s.api.Messages(ctx, wsID, id)
}

// Send submits a prompt to the engine.
func (s *Service) Send(ctx context.Context, id, text string) (string, error) {
	return s.SendWithAttachments(ctx, id, text, nil)
}

// SendWithInputBudget submits a prompt with an aggregate input-token ceiling.
// It is reserved for the detached background reviewer; normal foreground
// sends continue through Send/SendWithAttachments without a budget.
func (s *Service) SendWithInputBudget(ctx context.Context, id, text string, maxInputTokens int64) (string, error) {
	return s.sendWithAttachmentsAndBudget(ctx, id, text, nil, maxInputTokens)
}

// SendWithAttachments submits a prompt and its inline files to the engine.
//
// Derived file text rides inside the prompt: Crush converts every attachment
// into a binary content part, so text placed there never reaches the model.
// Only items carrying bytes the model can consume become native attachments.
func (s *Service) SendWithAttachments(ctx context.Context, id, text string, items []attachments.Prepared) (string, error) {
	return s.sendWithAttachmentsAndBudget(ctx, id, text, items, 0)
}

func (s *Service) sendWithAttachmentsAndBudget(ctx context.Context, id, text string, items []attachments.Prepared, maxInputTokens int64) (string, error) {
	wsID, err := s.currentWorkspaceID()
	if err != nil {
		return "", err
	}
	if s.api == nil {
		return "", errors.New("engine client not configured")
	}
	if id == "" {
		return "", errors.New("session id is required")
	}
	promptText := attachments.ComposePrompt(text, items)
	if promptText == "" {
		return "", errors.New("prompt text is required")
	}
	atts := make([]crushapi.Attachment, 0, len(items))
	for _, item := range items {
		if item.Attachment != nil {
			atts = append(atts, *item.Attachment)
		}
	}
	runID := uuid.NewString()
	if err := s.api.SendPromptWithAttachmentsAndBudget(ctx, wsID, id, promptText, runID, atts, maxInputTokens); err != nil {
		return "", fmt.Errorf("send prompt: %w", err)
	}
	return runID, nil
}

// Cancel asks the engine to abort the in-flight prompt for the session.
func (s *Service) Cancel(ctx context.Context, id string) error {
	wsID, err := s.currentWorkspaceID()
	if err != nil {
		return err
	}
	if s.api == nil {
		return errors.New("engine client not configured")
	}
	if id == "" {
		return errors.New("session id is required")
	}
	if err := s.api.CancelPrompt(ctx, wsID, id); err != nil {
		return fmt.Errorf("cancel prompt: %w", err)
	}
	return nil
}
