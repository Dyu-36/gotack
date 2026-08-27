package changes

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/Dyu-36/gotack/internal/crushapi"
	"github.com/Dyu-36/gotack/internal/workspace"
)

// service.go -- role: list changed files for a session with status and size.

// FileStatus is the row the UI shows in the "files changed" pane. Path is
// canonical (clean), Size is the last known byte count, and UpdatedAt is the
// engine's millisecond timestamp.
type FileStatus struct {
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	UpdatedAt int64  `json:"updated_at"`
}

// Service aggregates file-level changes for a session. It owns no state; the
// engine is the source of truth via the history endpoint.
type Service struct {
	api *crushapi.Client
	ws  *workspace.Service
}

// NewService wires a Service. The workspace service must already have a
// current workspace because history calls are workspace-scoped.
func NewService(api *crushapi.Client, ws *workspace.Service) *Service {
	return &Service{api: api, ws: ws}
}

// ChangedFiles returns the latest version of every distinct path the agent
// touched in this session. The engine's history endpoint emits one row per
// version, so we collapse to the last (by Version, then UpdatedAt) per path.
// The result is sorted by path for stable UI rendering.
func (s *Service) ChangedFiles(ctx context.Context, sessionID string) ([]FileStatus, error) {
	if sessionID == "" {
		return nil, errors.New("session id is required")
	}
	wsID, err := s.currentWorkspaceID()
	if err != nil {
		return nil, err
	}
	if s.api == nil {
		return nil, errors.New("engine client not configured")
	}
	history, err := s.api.History(ctx, wsID, sessionID)
	if err != nil {
		return nil, fmt.Errorf("fetch history: %w", err)
	}
	// crushapi.File already carries Version, UpdatedAt and Content, so the
	// winning row is kept as-is and projected to FileStatus once at the end.
	// A parallel tracker struct would only restate fields the row already has,
	// and keeping the row costs nothing: its string header shares history's
	// backing bytes.
	latest := make(map[string]crushapi.File, len(history))
	for _, f := range history {
		path := strings.TrimSpace(f.Path)
		if path == "" {
			continue
		}
		f.Path = path
		cur, ok := latest[path]
		// First row wins by definition; later rows replace when they outrank
		// on Version, then on UpdatedAt.
		if !ok || f.Version > cur.Version || (f.Version == cur.Version && f.UpdatedAt > cur.UpdatedAt) {
			latest[path] = f
		}
	}
	out := make([]FileStatus, 0, len(latest))
	for _, f := range latest {
		out = append(out, FileStatus{
			Path:      f.Path,
			Size:      int64(len(f.Content)),
			UpdatedAt: f.UpdatedAt,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// Diff returns a unified diff between the previous and last version of path
// in the session. If only one version exists, the diff is from the empty
// file. maxBytes caps the result; output past the cap is replaced with a
// truncation marker so the UI knows the diff was cut.
func (s *Service) Diff(ctx context.Context, sessionID, path string, maxBytes int64) (string, error) {
	if sessionID == "" {
		return "", errors.New("session id is required")
	}
	if strings.TrimSpace(path) == "" {
		return "", errors.New("file path is required")
	}
	wsID, err := s.currentWorkspaceID()
	if err != nil {
		return "", err
	}
	if s.api == nil {
		return "", errors.New("engine client not configured")
	}
	history, err := s.api.History(ctx, wsID, sessionID)
	if err != nil {
		return "", fmt.Errorf("fetch history: %w", err)
	}
	versions := versionsForPath(history, path)
	if len(versions) == 0 {
		return "", fmt.Errorf("no history for path: %s", path)
	}
	prev := ""
	if len(versions) > 1 {
		prev = versions[len(versions)-2].Content
	}
	last := versions[len(versions)-1].Content
	return RenderDiff(prev, last, path, maxBytes)
}

// currentWorkspaceID resolves the active workspace. Pulled out so both methods
// fail consistently when no workspace is open.
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

// versionsForPath returns the rows for path, oldest first. We sort by Version
// defensively in case the engine's ordering shifts in the future.
func versionsForPath(history []crushapi.File, path string) []crushapi.File {
	target := strings.TrimSpace(path)
	var out []crushapi.File
	for _, f := range history {
		if strings.TrimSpace(f.Path) == target {
			out = append(out, f)
		}
	}
	if len(out) == 0 {
		return nil
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out
}
