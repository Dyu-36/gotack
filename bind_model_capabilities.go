package main

import (
	"context"
	"strings"
	"time"

	"github.com/Dyu-36/gotack/internal/provider"
)

type visionCacheKey struct {
	workspaceID string
	providerID  string
	modelID     string
}

func (a *App) isCurrentModelVision(svc *bridgeServices) bool {
	if a.cfg == nil || svc == nil || svc.api == nil || svc.ws == nil {
		return false
	}
	providerID := strings.TrimSpace(a.cfg.Provider)
	modelID := strings.TrimSpace(a.cfg.Model)
	if providerID == "" || modelID == "" {
		return false
	}
	if override, ok := a.cfg.ModelCapabilities[modelID]; ok && override.SupportsVision != nil && !*override.SupportsVision {
		return false
	}
	desc, ok := svc.ws.Current()
	if !ok || desc.WorkspaceID == "" {
		return false
	}
	key := visionCacheKey{workspaceID: desc.WorkspaceID, providerID: providerID, modelID: modelID}
	if cached, ok := a.vision.Load(key); ok {
		return cached.(bool)
	}
	base := a.ctx
	if base == nil {
		base = context.Background()
	}
	ctx, cancel := context.WithTimeout(base, 10*time.Second)
	defer cancel()
	supportsVision, err := provider.SupportsVision(ctx, svc.api, desc.WorkspaceID, providerID, modelID)
	if err != nil {
		if a.log != nil {
			a.log.Warn("could not resolve model attachment capability; using text fallback", "provider", providerID, "model", modelID, "err", err)
		}
		return false
	}
	a.vision.Store(key, supportsVision)
	return supportsVision
}
