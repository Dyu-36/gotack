package main

import (
	"context"
	"path/filepath"
	"runtime"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/workspace"
)

// bind_workspace.go -- role: Wails-bound API for workspace selection.

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

// permissionsSkip reports whether Crush should skip interactive permission
// prompts for attached workspaces. The default is false: everything the
// guard leaves undecided reaches the UI as a prompt through the permission
// relay (the ask tier). Setting auto_approve in config.json restores the
// legacy fully-automatic behaviour; the guard's deny rules still apply
// because the hook decides before Crush's permission system ever runs.
func (a *App) permissionsSkip() bool {
	return a.cfg != nil && a.cfg.AutoApprove
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

// rebindWorkspaceRuntime re-points every workspace-scoped runtime at
// workspaceID: it swaps the SSE attach scope (cancelling the previous one),
// drops the Zalo chat-to-session mappings that belonged to the old workspace,
// re-registers the bundled Office MCP server, and re-points the seeded
// global context directory.
//
// Every activation path runs exactly this sequence through activateCurrent,
// which is how it stays aligned with replaceWorkspaceStream in
// bind_engine.go. Note it is NOT the same as replaceWorkspaceStream: that
// helper returns an error and cancels on attach failure, while these paths
// route failure through transportLost. Collapsing them would change
// reconnect behaviour.
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
	a.registerContextPaths(workspaceID)
	a.registerGuardHook(workspaceID)
}

// activateCurrent is the single shared activation sequence: apply the approval
// posture, optionally record the path in the recent list, and re-point every
// workspace-scoped runtime at the new workspace. Both activation entry points
// funnel through it so the two cannot drift.
func (a *App) activateCurrent(svc *bridgeServices, desc workspace.Descriptor, remember bool) (WorkspaceInfo, error) {
	// CreateWorkspace uses YOLO=true for new workspaces. This explicit call also
	// upgrades a workspace that an older Gotack/Crush process created with YOLO
	// disabled, because Crush uses first-wins semantics for duplicate paths.
	// The value follows the approval posture: prompts stay on unless the user
	// opted into auto_approve (ADR 0002 escape hatch).
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

// activateWorkspace makes a Crush workspace current through the shared
// activation sequence.
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
	// Credentials are owned by Crush and are intentionally not replayed here.
	if err := a.applyCrushSettings(saved, ""); err != nil && a.log != nil {
		a.log.Warn("could not reapply saved Crush settings", "err", err)
	}
}

// activateAssistantWorkspace attaches the default workspace through the
// shared activation sequence. When the current workspace already is the
// default one, only the permission flag is re-asserted: rebinding would
// replace a healthy event stream with an identical one for no benefit.
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

// EnsureAssistantWorkspace attaches Gotack's always-available default
// workspace. On Windows this is C:\, so chat and integrations have a valid
// Crush workspace immediately after startup without requiring folder selection.
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

// OpenWorkspace changes the default working directory for the agent. It is an
// optional convenience only: the agent remains able to use absolute paths to
// files and folders anywhere the OS account can access.
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
	return &WorkspaceInfo{Path: desc.Path, WorkspaceID: desc.WorkspaceID, IsDefault: isDefaultWorkspace(desc.Path)}
}
