package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

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

// Service orchestrates session lifecycle: list/create/switch and the two
// per-prompt flows (send, cancel). It does not own session state; the engine
// does. It only caches nothing and forwards every call.
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

// currentWorkspaceID returns the id of the workspace the user has opened. An
// error here is reported to the caller; there is no implicit fallback because
// every session call needs a workspace scope.
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

// Create opens a new session under the current workspace. Presence
// (current-session) tracking happens at the bind layer once the workspace
// stream is attached; the engine rejects current-session updates otherwise.
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

// Send submits a prompt to the engine. The returned run id is a fresh UUID
// per call; the caller can correlate it with the eventual run_complete
// stream event for that prompt.
func (s *Service) Send(ctx context.Context, id, text string) (string, error) {
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
	if strings.TrimSpace(text) == "" {
		return "", errors.New("prompt text is required")
	}
	runID := uuid.NewString()
	if err := s.api.SendPrompt(ctx, wsID, id, text, runID); err != nil {
		return "", fmt.Errorf("send prompt: %w", err)
	}
	return runID, nil
}

// Cancel asks the engine to abort the in-flight prompt for the session. The
// engine replies 202; a missing or already-finished run is not an error from
// the host's perspective.
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
