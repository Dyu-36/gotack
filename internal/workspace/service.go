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

// service.go -- role: validate a path, attach it in the engine, surface the
// current pick. Persistence of the recent-workspace list belongs to App
// (bind_workspace.go), where the config is already guarded by a.mu. Keeping
// it out of this package removes the second-mutex race that arose when the
// workspace Service exposed Recent() alongside App.ListRecentWorkspaces.

// Descriptor is the pair the bind layer returns to the UI. The workspace id
// is what subsequent crushapi calls need; the path is what the user sees.
type Descriptor struct {
	Path        string `json:"path"`
	WorkspaceID string `json:"workspace_id"`
}

// Service validates a local path, attaches it to the engine, and remembers
// only the active pick. The recent-workspace list is owned by App, which
// persists it through appconfig.Save. It is safe for concurrent use.
type Service struct {
	api *crushapi.Client

	mu      sync.RWMutex
	current Descriptor
}

// NewService wires a Service over the engine client. The config is read on
// demand through Current; Open does not mutate the config in place because
// the bind layer (App) now owns the list and saves it.
func NewService(api *crushapi.Client) *Service {
	return &Service{api: api}
}

// Current returns the last successful Open result. The bool is true when at
// least one Open has been called and the workspace still resolves.
func (s *Service) Current() (Descriptor, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.current, s.current.WorkspaceID != ""
}

// Open validates path, finds or creates the matching engine workspace, and
// records it as the current pick. The path is cleaned and must be an
// existing directory. Recent-list bookkeeping is the caller's job; see
// bind_workspace.go.
func (s *Service) Open(ctx context.Context, path string) (Descriptor, error) {
	return s.open(ctx, path, "")
}

// OpenWithDataDir is Open with an explicit Crush data directory. The working
// directory and the persistence directory stay independent.
func (s *Service) OpenWithDataDir(ctx context.Context, path, dataDir string) (Descriptor, error) {
	return s.open(ctx, path, dataDir)
}

func (s *Service) open(ctx context.Context, path, dataDir string) (Descriptor, error) {
	clean, err := s.preparePath(path)
	if err != nil {
		return Descriptor{}, err
	}

	ws, err := s.findOrCreate(ctx, clean, dataDir)
	if err != nil {
		return Descriptor{}, err
	}

	desc := Descriptor{Path: clean, WorkspaceID: ws.ID}
	s.mu.Lock()
	s.current = desc
	s.mu.Unlock()
	return desc, nil
}

// preparePath turns a user-typed path into the canonical directory path that
// crushapi expects. Errors are user-visible; the bind layer surfaces them.
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

// findOrCreate first looks for an existing workspace with this path; if none
// exists, it creates a new one. The match is on the cleaned absolute path so
// trivial differences like trailing slashes do not produce duplicates.
func (s *Service) findOrCreate(ctx context.Context, clean, dataDir string) (crushapi.Workspace, error) {
	if s.api == nil {
		return crushapi.Workspace{}, errors.New("engine client not configured")
	}
	// A transient list error is treated as "no match" so the create path still
	// runs; the bind layer surfaces the failure if create also fails.
	existing, err := s.api.ListWorkspaces(ctx)
	if err == nil {
		for _, w := range existing {
			if samePath(w.Path, clean) {
				return w, nil
			}
		}
	}
	// Gotack is a local assistant, not a project sandbox. New workspaces are
	// created in YOLO mode as the permissive baseline; the activation path
	// then calls SetPermissionsSkip with the real approval posture (guard
	// tiers plus the interactive ask relay), so this create-time value never
	// decides whether prompts appear.
	ws, err := s.api.CreateWorkspaceWithDataDir(ctx, clean, dataDir, true)
	if err != nil {
		return crushapi.Workspace{}, fmt.Errorf("create workspace: %w", err)
	}
	return ws, nil
}

// samePath compares two paths for workspace identity. We compare cleaned
// forms only. Case sensitivity follows the host filesystem: on Windows and
// macOS the default volume is case-insensitive, so D:\Proj and d:\proj are
// the same workspace; on Linux they are not. Branching on runtime.GOOS
// keeps the cross-compile unit tests for both behaviours next to the code
// they exercise.
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
