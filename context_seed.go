package main

import (
	"context"
	"os"
	"path/filepath"

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

func (a *App) ensureContextRegistrar() *contextseed.Registrar {
	if a.contextSeeder == nil {
		if a.contextRegistrar != nil {
			a.contextRegistrar.ReleaseAll()
			a.contextRegistrar = nil
		}
		return nil
	}
	if a.contextRegistrar == nil || a.contextRegistrar.Seeder() != a.contextSeeder {
		if a.contextRegistrar != nil {
			a.contextRegistrar.ReleaseAll()
		}
		a.contextRegistrar = contextseed.NewRegistrar(a.contextSeeder, a.log)
	}
	return a.contextRegistrar
}

func (a *App) registerContextPaths(workspaceID string) {
	registrar := a.ensureContextRegistrar()
	if registrar == nil {
		return
	}
	svc, err := a.services()
	if err != nil {
		return
	}
	registrar.Register(a.ctx, svc.api, workspaceID)
}

func (a *App) clearContextPath(ctx context.Context, api *crushapi.Client, workspaceID string) {
	if registrar := a.ensureContextRegistrar(); registrar != nil {
		registrar.Clear(ctx, api, workspaceID)
	}
}

func (a *App) contextLeaseGeneration(workspaceID string) string {
	if registrar := a.ensureContextRegistrar(); registrar != nil {
		return registrar.LeaseGeneration(workspaceID)
	}
	return ""
}

func (a *App) releaseAllContextLeases() {
	if a.contextRegistrar != nil {
		a.contextRegistrar.ReleaseAll()
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
