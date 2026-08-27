package main

import (
	"context"

	"github.com/Dyu-36/gotack/internal/appconfig"
)
// bind_workspace.go -- role: Wails-bound API for workspace selection.

// WorkspaceInfo describes the active engine workspace.
type WorkspaceInfo struct {
	Path        string `json:"path"`
	WorkspaceID string `json:"workspace_id"`
}

// ListRecentWorkspaces returns remembered project roots, most recent first.
func (a *App) ListRecentWorkspaces() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.cfg == nil {
		return nil
	}
	out := make([]string, len(a.cfg.RecentWorkspaces))
	copy(out, a.cfg.RecentWorkspaces)
	return out
}

// OpenWorkspace validates the path, attaches it in the engine, remembers it
// in the config, and switches the event stream to the new workspace.
func (a *App) OpenWorkspace(path string) (WorkspaceInfo, error) {
	svc, err := a.services()
	if err != nil {
		return WorkspaceInfo{}, err
	}
	desc, err := svc.ws.Open(a.ctx, path)
	if err != nil {
		return WorkspaceInfo{}, err
	}

	// App owns the recent-workspaces list. Mutating it here, under the
	// same mutex that ListRecentWorkspaces reads under, removes the
	// second-mutex race that arose when the workspace Service also held
	// a pointer to cfg.
	a.mu.Lock()
	appconfig.AddRecentWorkspace(a.cfg, desc.Path)
	cancel := a.cancelStream
	streamCtx, cancelNew := context.WithCancel(a.ctx)
	a.cancelStream = cancelNew
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	a.startStream(streamCtx, desc.WorkspaceID)

	return WorkspaceInfo{Path: desc.Path, WorkspaceID: desc.WorkspaceID}, nil
}

// CurrentWorkspace returns the active workspace or null when none is attached.
func (a *App) CurrentWorkspace() *WorkspaceInfo {
	a.mu.RLock()
	ws := a.ws
	a.mu.RUnlock()
	if ws == nil {
		return nil
	}
	desc, ok := ws.Current()
	if !ok {
		return nil
	}
	return &WorkspaceInfo{Path: desc.Path, WorkspaceID: desc.WorkspaceID}
}
