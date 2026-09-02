package main

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/workspace"
)

type WorkspaceInfo struct {
	Path        string `json:"path"`
	WorkspaceID string `json:"workspace_id"`
	IsDefault   bool   `json:"is_default"`
}

func defaultWorkspacePath() string {
	if runtime.GOOS == "windows" {
		return `C:\`
	}
	return string(filepath.Separator)
}

func defaultWorkspaceDataDir() string {
	return filepath.Join(appconfig.Dir(), "default-workspace-data")
}

func isDefaultWorkspace(path string) bool {
	return filepath.Clean(path) == filepath.Clean(defaultWorkspacePath())
}

func (a *App) permissionsSkip() bool {
	return a.cfg != nil && a.cfg.AutoApprove
}

func (a *App) ListRecentWorkspaces() []string {
	if a.cfg == nil {
		return nil
	}
	out := make([]string, len(a.cfg.RecentWorkspaces))
	copy(out, a.cfg.RecentWorkspaces)
	return out
}

func (a *App) rebindWorkspaceRuntime(workspaceID string) {
	var scope context.Context
	if a.getConn() != nil {
		scope = a.link.ReplaceStreamScope(a.ctx)
	}
	if scope != nil {
		a.startStream(scope, workspaceID)
	}
	a.resetZaloSessions()
	a.registerOfficeTools(workspaceID)
	a.registerMemoryTools(workspaceID)
	a.registerSkillsTools(workspaceID)
	a.registerRecallTools(workspaceID)
	a.registerContextPaths(workspaceID)
	a.registerGuardHook(workspaceID)
}

func (a *App) activateCurrent(svc *bridgeServices, desc workspace.Descriptor, remember bool) (WorkspaceInfo, error) {

	if err := svc.api.SetPermissionsSkip(a.ctx, desc.WorkspaceID, a.permissionsSkip()); err != nil {
		return WorkspaceInfo{}, err
	}
	if remember && a.cfg != nil {
		appconfig.AddRecentWorkspace(a.cfg, desc.Path)
	}
	a.rebindWorkspaceRuntime(desc.WorkspaceID)

	return WorkspaceInfo{
		Path:        desc.Path,
		WorkspaceID: desc.WorkspaceID,
		IsDefault:   isDefaultWorkspace(desc.Path),
	}, nil
}

func (a *App) activateWorkspace(svc *bridgeServices, path string, remember bool) (WorkspaceInfo, error) {
	desc, err := svc.ws.Open(a.ctx, path)
	if err != nil {
		return WorkspaceInfo{}, err
	}
	return a.activateCurrent(svc, desc, remember)
}

func (a *App) reapplySavedWorkspaceSettings() {
	if a.cfg == nil {
		return
	}
	saved := SettingsInfo{
		Theme:     a.cfg.Theme,
		Provider:  a.cfg.Provider,
		Model:     a.cfg.Model,
		Thinking:  a.cfg.Thinking,
		CustomURL: a.cfg.CustomURL,
	}

	effective, err := a.applyEffectiveCrushSettings(saved, "")
	if err != nil {
		if a.log != nil {
			a.log.Warn("could not reapply saved Crush settings", "err", err)
		}
		return
	}
	a.persistCorrectedSelection(effective)
}

func (a *App) persistCorrectedSelection(s SettingsInfo) {
	if a.cfg == nil {
		return
	}
	provider := strings.TrimSpace(s.Provider)
	model := strings.TrimSpace(s.Model)
	endpoint := strings.TrimSpace(s.CustomURL)
	if provider == a.cfg.Provider && model == a.cfg.Model && endpoint == a.cfg.CustomURL {
		return
	}
	a.cfg.Provider = provider
	a.cfg.Model = model
	a.cfg.CustomURL = endpoint
	cfgCopy := *a.cfg
	if err := appconfig.Save(&cfgCopy); err != nil && a.log != nil {
		a.log.Warn("could not save the corrected provider selection", "err", err)
	}
}

func (a *App) activateAssistantWorkspace(svc *bridgeServices) (WorkspaceInfo, error) {
	if desc, ok := svc.ws.Current(); ok && isDefaultWorkspace(desc.Path) {
		if err := svc.api.SetPermissionsSkip(a.ctx, desc.WorkspaceID, a.permissionsSkip()); err != nil {
			return WorkspaceInfo{}, err
		}
		return WorkspaceInfo{Path: desc.Path, WorkspaceID: desc.WorkspaceID, IsDefault: true}, nil
	}
	desc, err := svc.ws.OpenWithDataDir(a.ctx, defaultWorkspacePath(), defaultWorkspaceDataDir())
	if err != nil {
		return WorkspaceInfo{}, err
	}
	return a.activateCurrent(svc, desc, false)
}

func (a *App) EnsureAssistantWorkspace() (WorkspaceInfo, error) {
	svc, err := a.services()
	if err != nil {
		return WorkspaceInfo{}, err
	}
	info, err := a.activateAssistantWorkspace(svc)
	if err != nil {
		return WorkspaceInfo{}, err
	}
	a.reapplySavedWorkspaceSettings()
	return info, nil
}

func (a *App) OpenWorkspace(path string) (WorkspaceInfo, error) {
	svc, err := a.services()
	if err != nil {
		return WorkspaceInfo{}, err
	}
	info, err := a.activateWorkspace(svc, path, true)
	if err != nil {
		return WorkspaceInfo{}, err
	}
	a.reapplySavedWorkspaceSettings()
	return info, nil
}

func (a *App) CurrentWorkspace() *WorkspaceInfo {
	c := a.getConn()
	if c == nil || c.ws == nil {
		return nil
	}
	desc, ok := c.ws.Current()
	if !ok {
		return nil
	}
	return &WorkspaceInfo{Path: desc.Path, WorkspaceID: desc.WorkspaceID, IsDefault: isDefaultWorkspace(desc.Path)}
}
