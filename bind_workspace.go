package main

import (
	"context"

	"github.com/Dyu-36/gotack/internal/appconfig"
)

// bind_workspace.go -- role: Wails-bound API for workspace selection.

type WorkspaceInfo struct {
	Path        string `json:"path"`
	WorkspaceID string `json:"workspace_id"`
}

// ListRecentWorkspaces returns remembered project roots, most recent first.
func (a *App) ListRecentWorkspaces() []string {
	if a.cfg == nil {
		return nil
	}
	out := make([]string, len(a.cfg.RecentWorkspaces))
	copy(out, a.cfg.RecentWorkspaces)
	return out
}

// OpenWorkspace validates the path, attaches it in the engine, remembers it
// in the config, switches the event stream and reapplies saved non-secret
// model/provider settings to the live Crush workspace.
func (a *App) OpenWorkspace(path string) (WorkspaceInfo, error) {
	svc, err := a.services()
	if err != nil {
		return WorkspaceInfo{}, err
	}
	desc, err := svc.ws.Open(a.ctx, path)
	if err != nil {
		return WorkspaceInfo{}, err
	}

	if a.cfg != nil {
		appconfig.AddRecentWorkspace(a.cfg, desc.Path)
	}
	var cancel context.CancelFunc
	var streamCtx context.Context
	var saved SettingsInfo
	if a.cfg != nil {
		saved = SettingsInfo{
			Theme:           a.cfg.Theme,
			AutostartEngine: a.cfg.AutostartEngine,
			Provider:        a.cfg.Provider,
			Model:           a.cfg.Model,
			SmallModel:      a.cfg.SmallModel,
			Thinking:        a.cfg.Thinking,
			CustomURL:       a.cfg.CustomURL,
		}
	}
	if a.getConn() != nil {
		a.swapConn(func(c *conn) *conn {
			cancel = c.cancelStream
			streamCtx, c.cancelStream = context.WithCancel(a.ctx)
			return c
		})
	}
	if cancel != nil {
		cancel()
	}
	if streamCtx != nil {
		a.startStream(streamCtx, desc.WorkspaceID)
	}
	a.resetZaloSessions()
	a.registerOfficeTools(desc.WorkspaceID)

	// A saved API key is intentionally not available here: credentials live in
	// Crush, not Gotack. Model and endpoint settings are safe to replay.
	if err := a.applyCrushSettings(saved, ""); err != nil && a.log != nil {
		a.log.Warn("could not reapply saved Crush settings", "err", err)
	}

	return WorkspaceInfo{Path: desc.Path, WorkspaceID: desc.WorkspaceID}, nil
}

// CurrentWorkspace returns the active workspace or null when none is attached.
func (a *App) CurrentWorkspace() *WorkspaceInfo {
	c := a.getConn()
	if c == nil || c.ws == nil {
		return nil
	}
	desc, ok := c.ws.Current()
	if !ok {
		return nil
	}
	return &WorkspaceInfo{Path: desc.Path, WorkspaceID: desc.WorkspaceID}
}
