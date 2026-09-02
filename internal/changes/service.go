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

type FileStatus struct {
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	UpdatedAt int64  `json:"updated_at"`
}

type Service struct {
	api *crushapi.Client
	ws  *workspace.Service
}

func NewService(api *crushapi.Client, ws *workspace.Service) *Service {
	return &Service{api: api, ws: ws}
}

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

	latest := make(map[string]crushapi.File, len(history))
	for _, f := range history {
		path := strings.TrimSpace(f.Path)
		if path == "" {
			continue
		}
		f.Path = path
		cur, ok := latest[path]

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
