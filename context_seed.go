package main

import (
	"context"

	"github.com/Dyu-36/gotack/internal/contextseed"
	"github.com/Dyu-36/gotack/internal/engineapi"
)

func (a *App) ensureContextSeed() {
	if a.contextSeeder == nil {
		return
	}
	if err := a.contextSeeder.Seed(""); err != nil && a.log != nil {
		a.log.Warn("assistant: personal context initialization failed; originals preserved", "err", err)
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
	if err == nil {
		registrar.Register(a.ctx, svc.api, workspaceID)
	}
}

func (a *App) clearContextPath(ctx context.Context, api *engineapi.Client, workspaceID string) {
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
	if c == nil || c.ws == nil || a.contextSeeder == nil {
		return
	}
	desc, ok := c.ws.Current()
	if !ok {
		return
	}
	// Pin while comparing and registering; prune must not remove a generation
	// between publication and the registrar's acknowledgement.
	generation, lease, err := a.contextSeeder.BuildPromptSnapshotLease()
	if err != nil {
		if a.log != nil {
			a.log.Warn("assistant context refresh deferred", "err", err)
		}
		return
	}
	defer lease.Release()
	if generation != a.contextLeaseGeneration(desc.WorkspaceID) {
		a.registerContextPaths(desc.WorkspaceID)
	}
}
