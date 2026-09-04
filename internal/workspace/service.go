package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/Dyu-36/gotack/internal/crushapi"
)

type Descriptor struct {
	Path        string `json:"path"`
	WorkspaceID string `json:"workspace_id"`
	DataDir     string `json:"-"`
}

type Service struct {
	api *crushapi.Client

	mu      sync.RWMutex
	current Descriptor
}

func NewService(api *crushapi.Client) *Service {
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
	clean, err := s.preparePath(path)
	if err != nil {
		return Descriptor{}, err
	}

	_ = MigrateLegacyDataDir(clean)

	ws, err := s.findOrCreate(ctx, clean, dataDir)
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

func (s *Service) findOrCreate(ctx context.Context, clean, dataDir string) (crushapi.Workspace, error) {
	if s.api == nil {
		return crushapi.Workspace{}, errors.New("engine client not configured")
	}

	existing, err := s.api.ListWorkspaces(ctx)
	if err == nil {
		for _, w := range existing {
			if samePath(w.Path, clean) {
				return w, nil
			}
		}
	}

	ws, err := s.api.CreateWorkspaceWithDataDir(ctx, clean, dataDir, true)
	if err != nil {
		return crushapi.Workspace{}, fmt.Errorf("create workspace: %w", err)
	}
	return ws, nil
}

func samePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	ca, err := filepath.Abs(a)
	if err != nil {
		ca = a
	}
	cb, err := filepath.Abs(b)
	if err != nil {
		cb = b
	}
	ca = filepath.Clean(ca)
	cb = filepath.Clean(cb)
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return strings.EqualFold(ca, cb)
	}
	return ca == cb
}

// MigrateLegacyDataDir checks if a legacy .crush directory exists in the workspace
// and renames it to .tack if .tack does not already exist.
func MigrateLegacyDataDir(workspacePath string) error {
	if workspacePath == "" {
		return nil
	}
	legacyDir := filepath.Join(workspacePath, ".crush")
	targetDir := filepath.Join(workspacePath, ".tack")

	info, err := os.Stat(legacyDir)
	if err != nil || !info.IsDir() {
		return nil
	}
	if _, err := os.Stat(targetDir); err == nil {
		// Target .tack directory already exists; do not overwrite.
		return nil
	}
	return os.Rename(legacyDir, targetDir)
}
