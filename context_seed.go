package main

// context_seed.go -- role: seed the bundled context files (the Tack persona)
// into the per-user data directory at startup and register an immutable prompt
// projection in the active workspace's options.global_context_paths.

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/Dyu-36/gotack/internal/crushapi"
)

// resolveContextSourceDir locates the bundled context directory next to the
// running executable. The tracked persona file is the marker instead of a
// hardcoded executable name: naming the executable here would repeat risk
// R10, which made office seeding unreachable off Windows.
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
		if info, err := os.Stat(filepath.Join(candidate, "TACK.md")); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

// ensureContextSeed is called once during startup, next to ensureOfficeSeed.
// A missing bundled directory degrades to "no persona injection" rather than
// a startup failure, matching the office seeding posture.
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

// registerContextPaths writes one immutable prompt snapshot into the
// workspace's options.global_context_paths. The writable source directory is
// never registered: that keeps memory lock/temp files out of the prompt and
// lets load-time sanitization preserve poisoned raw entries for removal. The
// global key is used rather than options.context_paths so the engine's own
// discovery of per-project AGENTS.md / CRUSH.md stays intact.
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
		if a.log != nil {
			a.log.Warn("context prompt snapshot failed", "err", snapshotErr)
		}
		a.clearContextPath(ctx, svc.api, workspaceID)
		return
	}
	if err := svc.api.SetConfigField(ctx, workspaceID, crushapi.ConfigScopeWorkspace, "options.global_context_paths", []string{dir}); err != nil {
		if a.log != nil {
			a.log.Warn("context path registration failed", "err", err)
		}
		return
	}
	// Crush builds and retains the coder system prompt during agent
	// initialization. Context registration happens after workspace creation,
	// so a config reload alone would leave the first prompt on the previous
	// (possibly writable/raw) path. Refresh only that prompt from the committed
	// immutable projection; replacing the coordinator would discard its active
	// run, queue, and per-session state.
	if err := svc.api.RefreshPromptContext(ctx, workspaceID); err != nil {
		if a.log != nil {
			a.log.Warn("context prompt refresh failed", "err", err)
		}
		return
	}
	a.contextSeeder.PrunePromptSnapshots(dir)
}

// clearContextPath removes a stale/raw registration and refreshes the agent
// when possible. The refresh matters during upgrades: Crush may already have
// built a prompt from the previous path before this host-side cleanup ran.
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
