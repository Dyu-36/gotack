package main

// context_seed.go -- role: seed the bundled context files (the Tack persona)
// into the per-user data directory at startup and register that directory in
// the active workspace's options.global_context_paths, the same way
// office_seed.go wires the Office runtime.

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

// registerContextPaths writes the seeded context directory into the
// workspace's options.global_context_paths. The global key is used rather
// than options.context_paths so the engine's own discovery of per-project
// AGENTS.md / CRUSH.md stays intact. When the seeded directory is absent the
// key is removed instead, so the host never leaves a dangling path behind.
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

	dir := a.contextSeeder.ContextDir()
	if info, statErr := os.Stat(dir); statErr != nil || !info.IsDir() {
		_ = svc.api.RemoveConfigField(ctx, workspaceID, crushapi.ConfigScopeWorkspace, "options.global_context_paths")
		return
	}
	if err := svc.api.SetConfigField(ctx, workspaceID, crushapi.ConfigScopeWorkspace, "options.global_context_paths", []string{dir}); err != nil && a.log != nil {
		a.log.Warn("context path registration failed", "err", err)
	}
}
