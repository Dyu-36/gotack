package main

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/Dyu-36/gotack/internal/crushapi"
)

var safeProviderID = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func (a *App) applyEffectiveCrushSettings(s SettingsInfo, apiKey string) (SettingsInfo, error) {

	if svc, err := a.services(); err == nil {
		if desc, ok := svc.ws.Current(); ok && desc.WorkspaceID != "" {
			redirected, err := a.redirectStrandedChatGPTSelection(svc, desc.WorkspaceID, s, apiKey)
			if err != nil {
				return s, err
			}
			s = redirected
		}
	}
	return s, a.applyCrushSettings(s, apiKey)
}

func (a *App) applyCrushSettings(s SettingsInfo, apiKey string) error {
	svc, err := a.services()
	if err != nil {
		return needWorkspace(apiKey, "Crush is not running")
	}
	desc, ok := svc.ws.Current()
	if !ok || desc.WorkspaceID == "" {
		return needWorkspace(apiKey, "no workspace is open")
	}

	provider := strings.TrimSpace(s.Provider)
	credentialProvider := strings.TrimSpace(s.CredentialProvider)
	if credentialProvider == "" {
		credentialProvider = provider
	}
	modelID := strings.TrimSpace(s.Model)
	endpoint := strings.TrimSpace(s.CustomURL)
	if apiKey != "" && credentialProvider == "" {
		return errors.New("provider is required before storing an API key")
	}
	if apiKey != "" && credentialProvider == codexProviderID {
		return errors.New("codex signs in with ChatGPT, not an API key; use the openai provider for an API key")
	}
	if (s.ProviderOnly || endpoint != "") && !safeProviderID.MatchString(credentialProvider) {
		return fmt.Errorf("provider id %q cannot be used in a Crush config path", credentialProvider)
	}

	ws, scope := desc.WorkspaceID, crushapi.ConfigScopeGlobal
	if endpoint != "" {
		oauthBacked, oauthErr := providerUsesOAuth(a.ctx, svc.api, ws, credentialProvider)
		if oauthErr != nil {
			return oauthErr
		}
		if oauthBacked {
			return fmt.Errorf("provider %q signs in with OAuth and does not accept a custom endpoint", credentialProvider)
		}
	}

	managedLocalProvider := false
	if credentialProvider != "" {
		managedLocalProvider, err = prepareLocalProviderConfig(a.ctx, svc.api, ws, scope, credentialProvider)
		if err != nil {
			return err
		}
	}
	if credentialProvider != "" && s.ProviderOnly {
		if err := svc.api.SetConfigField(a.ctx, ws, scope, "providers."+credentialProvider+".disable", false); err != nil {
			return fmt.Errorf("enable provider: %w", err)
		}
	}

	if apiKey != "" {
		if err := svc.api.SetProviderAPIKey(a.ctx, ws, scope, credentialProvider, apiKey); err != nil {
			return fmt.Errorf("apply Crush provider credential: %w", err)
		}
	}

	if endpoint != "" {
		key := "providers." + credentialProvider + ".base_url"
		if err := svc.api.SetConfigField(a.ctx, ws, scope, key, endpoint); err != nil {
			return fmt.Errorf("apply Crush provider endpoint: %w", err)
		}
	}
	if managedLocalProvider {
		if err := finalizeLocalProviderConfig(a.ctx, svc.api, ws, scope, credentialProvider); err != nil {
			return err
		}
	}

	if !s.ProviderOnly && provider != "" && modelID != "" {
		effort, think := crushReasoning(s.Thinking)
		selected := crushapi.SelectedModel{Provider: provider, Model: modelID, ReasoningEffort: effort, Think: think}
		if err := svc.api.SetPreferredModelPair(a.ctx, ws, scope, selected); err != nil {
			return fmt.Errorf("apply Crush model selection: %w", err)
		}
	}

	if provider != "" && modelID != "" {
		if err := svc.api.EnsureAgent(a.ctx, ws, true); err != nil {
			return fmt.Errorf("initialize Crush agent: %w", err)
		}
	}
	return nil
}

func needWorkspace(apiKey, reason string) error {
	if apiKey == "" {
		return nil
	}
	return fmt.Errorf("cannot store API key: %s", reason)
}

func crushReasoning(value string) (effort string, think bool) {
	switch v := strings.ToLower(strings.TrimSpace(value)); v {
	case "minimal", "low", "medium", "high", "xhigh", "max":
		return v, true
	default:
		return "", false
	}
}
