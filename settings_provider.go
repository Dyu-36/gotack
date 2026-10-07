package main

import (
	"fmt"
	"strings"

	providerdomain "github.com/Dyu-36/gotack/internal/provider"
)

func providerSettingsFromInfo(settings SettingsInfo) providerdomain.Settings {
	return providerdomain.Settings{
		Theme:              settings.Theme,
		Provider:           settings.Provider,
		CredentialProvider: settings.CredentialProvider,
		ProviderOnly:       settings.ProviderOnly,
		Model:              settings.Model,
		Thinking:           settings.Thinking,
		CustomURL:          settings.CustomURL,
	}
}

func settingsInfoFromProvider(settings providerdomain.Settings) SettingsInfo {
	return SettingsInfo{
		Theme:              settings.Theme,
		Provider:           settings.Provider,
		CredentialProvider: settings.CredentialProvider,
		ProviderOnly:       settings.ProviderOnly,
		Model:              settings.Model,
		Thinking:           settings.Thinking,
		CustomURL:          settings.CustomURL,
	}
}

func (a *App) applyEffectiveProviderSettings(settings SettingsInfo, apiKey string) (SettingsInfo, error) {
	if svc, err := a.services(); err == nil {
		if desc, ok := svc.Workspace.Current(); ok && desc.WorkspaceID != "" {
			redirected, err := a.redirectStrandedChatGPTSelection(svc, desc.WorkspaceID, settings, apiKey)
			if err != nil {
				return settings, err
			}
			settings = redirected
		}
	}
	return settings, a.applyProviderSettings(settings, apiKey)
}

func (a *App) applyProviderSettings(settings SettingsInfo, apiKey string) error {
	if !settings.ProviderOnly {
		if err := providerdomain.ValidateChatModel(settings.Provider, settings.Model); err != nil {
			return err
		}
	}
	svc, err := a.services()
	if err != nil {
		return needWorkspace(apiKey, "Tack engine is not running")
	}
	desc, ok := svc.Workspace.Current()
	if !ok || desc.WorkspaceID == "" {
		return needWorkspace(apiKey, "no workspace is open")
	}
	if a.providerCatalog != nil {
		providerID := strings.TrimSpace(settings.CredentialProvider)
		if providerID == "" {
			providerID = strings.TrimSpace(settings.Provider)
		}
		if providerID != "" {
			if err := a.providerCatalog.Prepare(a.ctx, svc.API, desc.WorkspaceID, providerID); err != nil {
				return fmt.Errorf("prepare provider models: %w", err)
			}
		}
	}
	providerSettings := providerSettingsFromInfo(settings)
	providerSettings.CatalogManaged = a.providerCatalog != nil
	return providerdomain.Apply(a.ctx, svc.API, desc.WorkspaceID, providerSettings, apiKey)
}

func needWorkspace(apiKey, reason string) error {
	if apiKey == "" {
		return nil
	}
	return fmt.Errorf("cannot store API key: %s", reason)
}
