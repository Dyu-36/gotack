package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/Dyu-36/gotack/internal/engineapi"
)

type Descriptor struct {
	Path        string `json:"path"`
	WorkspaceID string `json:"workspace_id"`
	DataDir     string `json:"-"`
}

type Service struct {
	api *engineapi.Client

	openMu  sync.Mutex
	mu      sync.RWMutex
	current Descriptor
}

func NewService(api *engineapi.Client) *Service {
	return &Service{api: api}
}

func (s *Service) Current() (Descriptor, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.current, s.current.WorkspaceID != ""
}

func (s *Service) Open(ctx context.Context, path string) (Descriptor, error) {
	return s.open(ctx, path, "")
}

func (s *Service) OpenWithDataDir(ctx context.Context, path, dataDir string) (Descriptor, error) {
	return s.open(ctx, path, dataDir)
}

func (s *Service) open(ctx context.Context, path, dataDir string) (Descriptor, error) {
	s.openMu.Lock()
	defer s.openMu.Unlock()

	clean, err := s.preparePath(path)
	if err != nil {
		return Descriptor{}, err
	}

	ws, err := s.claimWorkspace(ctx, clean, dataDir)
	if err != nil {
		return Descriptor{}, err
	}

	desc := Descriptor{Path: clean, WorkspaceID: ws.ID, DataDir: ws.DataDir}
	s.mu.Lock()
	s.current = desc
	s.mu.Unlock()
	return desc, nil
}

func (s *Service) preparePath(path string) (string, error) {
	if path == "" {
		return "", errors.New("workspace path is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	clean := filepath.Clean(abs)
	info, err := os.Stat(clean)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("directory does not exist: %s", clean)
		}
		return "", fmt.Errorf("stat path: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("not a directory: %s", clean)
	}
	return clean, nil
}

func (s *Service) claimWorkspace(ctx context.Context, clean, dataDir string) (engineapi.Workspace, error) {
	if s.api == nil {
		return engineapi.Workspace{}, errors.New("engine client not configured")
	}
	// The engine's create endpoint is intentionally idempotent by workspace
	// path. Calling it for every open both reuses an existing workspace and
	// registers this client ID, which is required by current-session and
	// presence semantics after reconnects.
	ws, err := s.api.CreateWorkspaceWithDataDir(ctx, clean, dataDir, true)
	if err != nil {
		return engineapi.Workspace{}, fmt.Errorf("claim workspace: %w", err)
	}
	return ws, nil
}
