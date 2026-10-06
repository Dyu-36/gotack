package main

import (
	"context"
	"fmt"
)

type bridgeServices = conn

func (a *App) services() (*bridgeServices, error) { return a.host.Services() }
func (a *App) tryConnect() bool                   { return a.host.TryConnect() }
func (a *App) startStream(scope context.Context, workspaceID string) {
	a.host.StartStream(scope, workspaceID)
}
func (a *App) replaceWorkspaceStream(workspaceID string) error {
	return a.host.ReplaceWorkspaceStream(a.ctx, workspaceID)
}
func (a *App) stopTransport() { a.host.Disconnect() }

func (a *App) initializeEngineWorkspace(svc *bridgeServices) string {
	if info, err := a.activateAssistantWorkspace(svc); err != nil {
		a.log.Warn("could not attach the default workspace", "err", err)
		return fmt.Sprintf("initialize assistant workspace: %v", err)
	} else if err := a.rebindWorkspaceRuntime(info.WorkspaceID); err != nil {
		a.log.Warn("could not apply assistant workspace runtime", "err", err)
		return fmt.Sprintf("apply assistant workspace runtime: %v", err)
	}
	return ""
}
