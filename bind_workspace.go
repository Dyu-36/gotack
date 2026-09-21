package main

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/engine"
	"github.com/Dyu-36/gotack/internal/projecttrust"
	"github.com/Dyu-36/gotack/internal/workspace"
)

type WorkspaceInfo struct {
	Path               string   `json:"path"`
	WorkspaceID        string   `json:"workspace_id"`
	IsDefault          bool     `json:"is_default"`
	Trusted            bool     `json:"trusted"`
	TrustRequired      bool     `json:"trust_required"`
	TrustInheritedFrom string   `json:"trust_inherited_from,omitempty"`
	ProtectedResources []string `json:"protected_resources,omitempty"`
}

func (a *App) inspectWorkspaceTrust(path string) projecttrust.Status {
	if a.projectTrust == nil {
		return projecttrust.Status{Path: path, Trusted: true}
	}
	status, err := a.projectTrust.Inspect(path)
	if err != nil {
		if a.log != nil {
			a.log.Warn("project trust inspection failed; project resources remain disabled", "path", path, "err", err)
		}
		return projecttrust.Status{Path: path, Trusted: false, Required: true}
	}
	return status
}

func (a *App) workspaceInfo(desc workspace.Descriptor) WorkspaceInfo {
	trust := a.inspectWorkspaceTrust(desc.Path)
	return WorkspaceInfo{
		Path:               desc.Path,
		WorkspaceID:        desc.WorkspaceID,
		IsDefault:          isDefaultWorkspace(desc.Path),
		Trusted:            trust.Trusted,
		TrustRequired:      trust.Required,
		TrustInheritedFrom: trust.InheritedFrom,
		ProtectedResources: append([]string(nil), trust.Resources...),
	}
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

func (a *App) ListRecentWorkspaces() []string {
	if a.cfg == nil {
		return nil
	}
	out := make([]string, len(a.cfg.RecentWorkspaces))
	copy(out, a.cfg.RecentWorkspaces)
	return out
}

func (a *App) rebindWorkspaceRuntime(workspaceID string) {
	if a.link == nil || a.link.Status() != engine.StatusRunning {
		return
	}
	var scope context.Context
	if a.getConn() != nil {
		scope = a.link.ReplaceStreamScope(a.ctx)
	}
	if scope != nil {
		a.startStream(scope, workspaceID)
	}
	a.resetZaloSessions()

	svc, err := a.services()
	if err != nil {
		return
	}
	desc, ok := svc.ws.Current()
	if !ok || desc.WorkspaceID != workspaceID {
		return
	}
	trust := a.inspectWorkspaceTrust(desc.Path)
	if err := a.workspaceRuntimeManager().Apply(a.ctx, svc.api, desc, trust.Trusted); err != nil && a.log != nil {
		a.log.Warn("workspace runtime apply failed", "workspace", desc.Path, "err", err)
	}
}

func (a *App) activateCurrent(svc *bridgeServices, desc workspace.Descriptor, remember bool) (WorkspaceInfo, error) {
	if remember && a.cfg != nil {
		appconfig.AddRecentWorkspace(a.cfg, desc.Path)
	}
	a.rebindWorkspaceRuntime(desc.WorkspaceID)

	return a.workspaceInfo(desc), nil
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

	effective, err := a.applyEffectiveProviderSettings(saved, "")
	if err != nil {
		if a.log != nil {
			a.log.Warn("could not reapply saved provider settings", "err", err)
		}
		return
	}
	a.persistCorrectedSelection(effective)
}

func (a *App) persistCorrectedSelection(settings SettingsInfo) {
	if a.cfg == nil {
		return
	}
	providerID := strings.TrimSpace(settings.Provider)
	modelID := strings.TrimSpace(settings.Model)
	endpoint := strings.TrimSpace(settings.CustomURL)
	if providerID == a.cfg.Provider && modelID == a.cfg.Model && endpoint == a.cfg.CustomURL {
		return
	}
	a.cfg.Provider = providerID
	a.cfg.Model = modelID
	a.cfg.CustomURL = endpoint
	cfgCopy := *a.cfg
	if err := appconfig.Save(&cfgCopy); err != nil && a.log != nil {
		a.log.Warn("could not save the corrected provider selection", "err", err)
	}
}

func (a *App) activateAssistantWorkspace(svc *bridgeServices) (WorkspaceInfo, error) {
	if desc, ok := svc.ws.Current(); ok && isDefaultWorkspace(desc.Path) {
		a.rebindWorkspaceRuntime(desc.WorkspaceID)
		return a.workspaceInfo(desc), nil
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
	info := a.workspaceInfo(desc)
	return &info
}

// SetWorkspaceTrust stores an explicit trust decision for the active project.
// It controls only project-provided dynamic resources and never changes tool
// permissions or OS privileges.
func (a *App) SetWorkspaceTrust(trusted bool) (WorkspaceInfo, error) {
	svc, err := a.services()
	if err != nil {
		return WorkspaceInfo{}, err
	}
	desc, ok := svc.ws.Current()
	if !ok {
		return WorkspaceInfo{}, fmt.Errorf("no workspace is open")
	}
	if a.projectTrust == nil {
		a.projectTrust = projecttrust.New(filepath.Join(appconfig.Dir(), "project-trust.json"))
	}
	if err := a.projectTrust.Set(desc.Path, trusted); err != nil {
		return WorkspaceInfo{}, err
	}
	if err := a.workspaceRuntimeManager().Apply(a.ctx, svc.api, desc, trusted); err != nil {
		return WorkspaceInfo{}, err
	}
	if err := svc.api.RefreshPromptContext(a.ctx, desc.WorkspaceID); err != nil && a.log != nil {
		a.log.Debug("prompt refresh deferred until agent initialization", "err", err)
	}
	return a.workspaceInfo(desc), nil
}

func (a *App) ResetWorkspaceTrust() (WorkspaceInfo, error) {
	svc, err := a.services()
	if err != nil {
		return WorkspaceInfo{}, err
	}
	desc, ok := svc.ws.Current()
	if !ok {
		return WorkspaceInfo{}, fmt.Errorf("no workspace is open")
	}
	if a.projectTrust != nil {
		if err := a.projectTrust.Clear(desc.Path); err != nil {
			return WorkspaceInfo{}, err
		}
	}
	status := a.inspectWorkspaceTrust(desc.Path)
	if err := a.workspaceRuntimeManager().Apply(a.ctx, svc.api, desc, status.Trusted); err != nil {
		return WorkspaceInfo{}, err
	}
	if err := svc.api.RefreshPromptContext(a.ctx, desc.WorkspaceID); err != nil && a.log != nil {
		a.log.Debug("prompt refresh deferred until agent initialization", "err", err)
	}
	return a.workspaceInfo(desc), nil
}
