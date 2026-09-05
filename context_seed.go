package main

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/Dyu-36/gotack/internal/crushapi"
)

func resolveContextSourceDir() string {
	executable, err := os.Executable()
	if err != nil {
		return ""
	}
	root := filepath.Dir(executable)
	for _, candidate := range []string{
		filepath.Join(root, "resources", "context"),
		filepath.Join(root, "..", "resources", "context"),
	} {
		if info, err := os.Stat(filepath.Join(candidate, "TACK_CORE.md")); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

func (a *App) ensureContextSeed() {
	if a.contextSeeder == nil {
		return
	}
	source := resolveContextSourceDir()
	if source == "" {
		if a.log != nil {
			a.log.Debug("context: bundled context files not found, skipping seed")
		}
		return
	}
	if err := a.contextSeeder.Seed(source); err != nil && a.log != nil {
		a.log.Warn("context: failed to seed bundled context files", "err", err)
	}
}

func (a *App) registerContextPaths(workspaceID string) {
	if a.contextSeeder == nil {
		return
	}
	svc, err := a.services()
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Second)
	defer cancel()

	if info, statErr := os.Stat(a.contextSeeder.ContextDir()); statErr != nil || !info.IsDir() {
		a.clearContextPath(ctx, svc.api, workspaceID)
		return
	}
	dir, snapshotErr := a.contextSeeder.BuildPromptSnapshot()
	if snapshotErr != nil {
		// A failed refresh keeps the previously committed revision: the
		// engine keeps serving the last snapshot registered in its
		// config (content-addressed directories are immutable, so that
		// revision is still intact). Clearing the registration here
		// would silently drop the user's context on a transient error.
		if a.log != nil {
			a.log.Warn("context prompt snapshot failed; keeping committed revision", "err", snapshotErr)
		}
		return
	}
	if err := svc.api.SetConfigField(ctx, workspaceID, crushapi.ConfigScopeWorkspace, "options.global_context_paths", []string{dir}); err != nil {
		if a.log != nil {
			a.log.Warn("context path registration failed", "err", err)
		}
		return
	}

	if err := svc.api.RefreshPromptContext(ctx, workspaceID); err != nil {
		if a.log != nil {
			a.log.Warn("context prompt refresh failed", "err", err)
		}
		return
	}
	a.contextSeeder.PrunePromptSnapshots(dir)
}

func (a *App) clearContextPath(ctx context.Context, api *crushapi.Client, workspaceID string) {
	if err := api.RemoveConfigField(ctx, workspaceID, crushapi.ConfigScopeWorkspace, "options.global_context_paths"); err != nil {
		if a.log != nil {
			a.log.Warn("context path removal failed", "err", err)
		}
		return
	}
	if err := api.RefreshPromptContext(ctx, workspaceID); err != nil && a.log != nil {
		a.log.Warn("context prompt refresh after removal failed", "err", err)
	}
}

func (a *App) refreshCurrentContextSnapshot() {
	c := a.getConn()
	if c == nil || c.ws == nil {
		return
	}
	desc, ok := c.ws.Current()
	if !ok {
		return
	}
	a.registerContextPaths(desc.WorkspaceID)
}
