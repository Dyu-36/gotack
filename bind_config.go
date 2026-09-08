package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/engineapi"
	providerdomain "github.com/Dyu-36/gotack/internal/provider"
)

type SettingsInfo struct {
	Theme              string `json:"theme"`
	Provider           string `json:"provider"`
	CredentialProvider string `json:"credential_provider,omitempty"`
	ProviderOnly       bool   `json:"provider_only,omitempty"`
	Model              string `json:"model"`
	Thinking           string `json:"thinking"`
	APIKey             string `json:"api_key"`
	CustomURL          string `json:"custom_url"`
}

func (a *App) GetSettings() SettingsInfo {
	if a.cfg == nil {
		return SettingsInfo{Theme: "system"}
	}
	return SettingsInfo{
		Theme: a.cfg.Theme, Provider: a.cfg.Provider, Model: a.cfg.Model,
		Thinking: a.cfg.Thinking, CustomURL: a.cfg.CustomURL,
	}
}

func (a *App) configWorkspaceID(ctx context.Context, svc *bridgeServices) (string, error) {
	if desc, ok := svc.ws.Current(); ok && desc.WorkspaceID != "" {
		return desc.WorkspaceID, nil
	}
	catalogPath := filepath.Join(appconfig.Dir(), "catalog-workspace")
	if err := os.MkdirAll(catalogPath, 0o755); err != nil {
		return "", fmt.Errorf("create catalog workspace directory: %w", err)
	}
	ws, err := svc.api.CreateWorkspace(ctx, catalogPath, false)
	if err != nil {
		return "", fmt.Errorf("create catalog workspace: %w", err)
	}
	return ws.ID, nil
}

func (a *App) ListProviders() ([]engineapi.Provider, error) {
	svc, err := a.services()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(a.ctx, 90*time.Second)
	defer cancel()

	workspaceID, err := a.configWorkspaceID(ctx, svc)
	if err != nil {
		return nil, err
	}
	providers, err := providerdomain.ListCatalog(ctx, svc.api, workspaceID)
	if err != nil {
		return nil, err
	}
	for i := range providers {
		for j := range providers[i].Models {
			model := &providers[i].Models[j]
			if a.cfg != nil && a.cfg.ModelCapabilities != nil {
				if override, ok := a.cfg.ModelCapabilities[model.ID]; ok {
					if override.SupportsVision != nil && !*override.SupportsVision {
						model.SupportsVision = false
					}
					if override.CanReason != nil {
						model.CanReason = *override.CanReason
					}
				}
			}
		}
	}
	a.vision.Clear()
	for _, provider := range providers {
		for _, model := range provider.Models {
			a.vision.Store(visionCacheKey{workspaceID: workspaceID, providerID: provider.ID, modelID: model.ID}, model.SupportsVision)
		}
	}
	return providers, nil
}

func (a *App) RevealProviderAPIKey(providerID string) (string, error) {
	svc, err := a.services()
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Second)
	defer cancel()

	workspaceID, err := a.configWorkspaceID(ctx, svc)
	if err != nil {
		return "", err
	}
	cfg, err := svc.api.GetWorkspaceConfig(ctx, workspaceID)
	if err != nil {
		return "", err
	}
	configured, exists := cfg.Providers[strings.TrimSpace(providerID)]
	if !exists || configured.Disable {
		return "", fmt.Errorf("provider %q is not configured", providerID)
	}
	kind, key, usable := providerdomain.ResolvedCredential(configured)
	if !usable || kind != "api_key" {
		return "", fmt.Errorf("provider %q does not have a revealable API key", providerID)
	}
	return key, nil
}

func (a *App) DeleteProvider(providerID string) error {
	providerID = strings.TrimSpace(providerID)
	if !providerdomain.ValidID(providerID) {
		return fmt.Errorf("invalid provider id %q", providerID)
	}
	svc, err := a.services()
	if err != nil {
		return err
	}
	desc, ok := svc.ws.Current()
	if !ok || desc.WorkspaceID == "" {
		return fmt.Errorf("no workspace is open")
	}

	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Second)
	defer cancel()
	engineConfig, err := svc.api.GetWorkspaceConfig(ctx, desc.WorkspaceID)
	if err != nil {
		return fmt.Errorf("read provider state before deletion: %w", err)
	}
	clearSelection := a.cfg != nil && strings.TrimSpace(a.cfg.Provider) == providerID
	clearModels := clearSelection || providerdomain.PreferredModelsUseProvider(engineConfig.Models, providerID)
	if clearSelection {
		next := *a.cfg
		next.Provider, next.Model, next.CustomURL = "", "", ""
		if err := appconfig.Save(&next); err != nil {
			return fmt.Errorf("save cleared provider selection: %w", err)
		}
		a.cfg = &next
	}
	if err := providerdomain.DeleteEngineConfig(ctx, svc.api, desc.WorkspaceID, providerID, clearModels); err != nil {
		return err
	}
	a.vision.Clear()
	return nil
}

func (a *App) SaveSettings(settings SettingsInfo) error {
	apiKey := strings.TrimSpace(settings.APIKey)
	effective, err := a.applyEffectiveProviderSettings(settings, apiKey)
	if err != nil {
		return err
	}

	current := a.cfg
	if current == nil {
		current = appconfig.Defaults()
	}
	next := *current
	if effective.Theme != "" {
		next.Theme = effective.Theme
	}
	next.Provider = strings.TrimSpace(effective.Provider)
	next.Model = strings.TrimSpace(effective.Model)
	next.Thinking = strings.TrimSpace(effective.Thinking)
	next.APIKey = ""
	credentialProvider := strings.TrimSpace(effective.CredentialProvider)
	if credentialProvider == "" || credentialProvider == next.Provider {
		next.CustomURL = strings.TrimSpace(effective.CustomURL)
	}
	if err := appconfig.Save(&next); err != nil {
		return err
	}
	a.cfg = &next
	a.vision.Clear()
	return nil
}
