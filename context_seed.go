package main

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/Dyu-36/gotack/internal/contextseed"
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
	previousPaths, transactionLease, previousErr := a.captureContextRegistration(ctx, svc.api, workspaceID)
	if previousErr != nil {
		if a.log != nil {
			a.log.Warn("context: failed to capture current registration; refusing mutation", "err", previousErr)
		}
		return
	}
	dir, lease, snapshotErr := a.contextSeeder.BuildPromptSnapshotLease()
	if snapshotErr != nil {
		if transactionLease != nil {
			a.restoreAcknowledgedContextLease(workspaceID, transactionLease)
		}
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
		_ = lease.Release()
		if transactionLease != nil {
			a.restoreAcknowledgedContextLease(workspaceID, transactionLease)
		}
		if a.log != nil {
			a.log.Warn("context path registration failed", "err", err)
		}
		return
	}

	if err := svc.api.RefreshPromptContext(ctx, workspaceID); err != nil {
		rollbackErr := a.restoreContextRegistration(ctx, svc.api, workspaceID, previousPaths)
		if rollbackErr != nil {
			a.holdPendingContextLease(workspaceID, lease)
			if transactionLease != nil {
				a.restoreAcknowledgedContextLease(workspaceID, transactionLease)
			}
		} else {
			_ = lease.Release()
			if transactionLease != nil {
				a.replaceContextLease(workspaceID, transactionLease)
			}
		}
		if a.log != nil {
			a.log.Warn("context prompt refresh failed; restored prior registration", "err", err, "rollback_err", rollbackErr)
		}
		return
	}
	if transactionLease != nil {
		_ = transactionLease.Release()
	}
	a.replaceContextLease(workspaceID, lease)
	if err := a.contextSeeder.PrunePromptSnapshotsChecked(dir); err != nil && a.log != nil {
		a.log.Warn("context snapshot prune failed", "err", err)
	}
}

func (a *App) clearContextPath(ctx context.Context, api *crushapi.Client, workspaceID string) {
	previousPaths, transactionLease, previousErr := a.captureContextRegistration(ctx, api, workspaceID)
	if previousErr != nil {
		if a.log != nil {
			a.log.Warn("context: failed to capture current registration before removal", "err", previousErr)
		}
		return
	}
	if err := api.RemoveConfigField(ctx, workspaceID, crushapi.ConfigScopeWorkspace, "options.global_context_paths"); err != nil {
		if transactionLease != nil {
			a.restoreAcknowledgedContextLease(workspaceID, transactionLease)
		}
		if a.log != nil {
			a.log.Warn("context path removal failed", "err", err)
		}
		return
	}
	if err := api.RefreshPromptContext(ctx, workspaceID); err != nil {
		rollbackErr := a.restoreContextRegistration(ctx, api, workspaceID, previousPaths)
		if transactionLease != nil {
			if rollbackErr == nil {
				a.replaceContextLease(workspaceID, transactionLease)
			} else {
				a.restoreAcknowledgedContextLease(workspaceID, transactionLease)
			}
		}
		if a.log != nil {
			a.log.Warn("context prompt refresh after removal failed; restored prior registration", "err", err, "rollback_err", rollbackErr)
		}
		return
	}
	if transactionLease != nil {
		_ = transactionLease.Release()
	}
	a.replaceContextLease(workspaceID, nil)
}

func (a *App) captureContextRegistration(ctx context.Context, api *crushapi.Client, workspaceID string) ([]string, *contextseed.SnapshotLease, error) {
	cfg, err := api.GetWorkspaceConfig(ctx, workspaceID)
	if err != nil {
		return nil, nil, err
	}
	var paths []string
	if cfg.Options != nil {
		paths = append(paths, cfg.Options.GlobalContextPaths...)
	}
	if len(paths) != 1 || a.contextSeeder == nil {
		return paths, nil, nil
	}
	path := filepath.Clean(paths[0])
	if current := a.contextLeaseGeneration(workspaceID); current != "" && filepath.Clean(current) == path {
		return paths, nil, nil
	}
	root, err := filepath.Abs(a.contextSeeder.PromptContextRoot())
	if err != nil {
		return nil, nil, err
	}
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return nil, nil, err
	}
	relative, err := filepath.Rel(root, absolutePath)
	if err != nil || filepath.Dir(relative) != "." {
		return paths, nil, nil
	}
	lease, err := a.contextSeeder.AcquireSnapshotLease(path)
	if err != nil {
		return nil, nil, err
	}
	return paths, lease, nil
}

func (a *App) restoreContextRegistration(ctx context.Context, api *crushapi.Client, workspaceID string, paths []string) error {
	if len(paths) == 0 {
		return api.RemoveConfigField(ctx, workspaceID, crushapi.ConfigScopeWorkspace, "options.global_context_paths")
	}
	return api.SetConfigField(ctx, workspaceID, crushapi.ConfigScopeWorkspace, "options.global_context_paths", append([]string(nil), paths...))
}

func (a *App) contextLeaseGeneration(workspaceID string) string {
	a.contextLeaseMu.Lock()
	defer a.contextLeaseMu.Unlock()
	if lease := a.contextLeases[workspaceID]; lease != nil {
		return lease.Generation()
	}
	return ""
}

func (a *App) holdPendingContextLease(workspaceID string, lease *contextseed.SnapshotLease) {
	if lease == nil {
		return
	}
	a.contextLeaseMu.Lock()
	if a.contextPendingLeases == nil {
		a.contextPendingLeases = make(map[string][]*contextseed.SnapshotLease)
	}
	a.contextPendingLeases[workspaceID] = append(a.contextPendingLeases[workspaceID], lease)
	a.contextLeaseMu.Unlock()
}

func (a *App) restoreAcknowledgedContextLease(workspaceID string, lease *contextseed.SnapshotLease) {
	if lease == nil {
		return
	}
	a.contextLeaseMu.Lock()
	if a.contextLeases == nil {
		a.contextLeases = make(map[string]*contextseed.SnapshotLease)
	}
	old := a.contextLeases[workspaceID]
	a.contextLeases[workspaceID] = lease
	a.contextLeaseMu.Unlock()
	if old != nil && old != lease {
		_ = old.Release()
	}
}

func (a *App) replaceContextLease(workspaceID string, lease *contextseed.SnapshotLease) {
	a.contextLeaseMu.Lock()
	if a.contextLeases == nil {
		a.contextLeases = make(map[string]*contextseed.SnapshotLease)
	}
	old := a.contextLeases[workspaceID]
	pending := a.contextPendingLeases[workspaceID]
	delete(a.contextPendingLeases, workspaceID)
	if lease == nil {
		delete(a.contextLeases, workspaceID)
	} else {
		a.contextLeases[workspaceID] = lease
	}
	a.contextLeaseMu.Unlock()
	if old != nil && old != lease {
		_ = old.Release()
	}
	for _, pendingLease := range pending {
		if pendingLease != nil && pendingLease != lease && pendingLease != old {
			_ = pendingLease.Release()
		}
	}
}

func (a *App) releaseAllContextLeases() {
	a.contextLeaseMu.Lock()
	leases := a.contextLeases
	pending := a.contextPendingLeases
	a.contextLeases = nil
	a.contextPendingLeases = nil
	a.contextLeaseMu.Unlock()
	for _, lease := range leases {
		_ = lease.Release()
	}
	for _, group := range pending {
		for _, lease := range group {
			if lease != nil {
				_ = lease.Release()
			}
		}
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
