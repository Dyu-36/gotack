package main

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/Dyu-36/gotack/internal/crushapi"
)

var safeProviderID = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// applyEffectiveCrushSettings applies a settings payload after repointing a
// selection that the provider split stranded, and returns what was actually
// written. Every caller that persists the selection must use this instead of
// applyCrushSettings: the UI replays the selection it read at boot, so a
// payload naming the credential-less "openai" provider would otherwise undo the
// Codex migration on the very next save.
func (a *App) applyEffectiveCrushSettings(s SettingsInfo, apiKey string) (SettingsInfo, error) {
	// A missing engine or workspace is not an error here; applyCrushSettings
	// already decides whether that is a no-op or a refused credential write.
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

// applyCrushSettings pushes agent-affecting settings into the currently open
// Crush workspace. Crush hot-reloads these endpoints, so no engine restart is
// needed. apiKey is an argument on purpose: it is never persisted in Gotack.
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
		credentialProvider = provider // backwards compatibility with older UI payloads
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

	// Everything above is read-only. From this point onward the REST API does
	// not offer a transaction, so keep mutations in their dependency order.
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

	// Select the provider only after its credential, endpoint and discovery
	// state are ready. A failed setup must leave the previous model usable.
	if !s.ProviderOnly && provider != "" && modelID != "" {
		effort, think := crushReasoning(s.Thinking)
		selected := crushapi.SelectedModel{Provider: provider, Model: modelID, ReasoningEffort: effort, Think: think}
		if err := svc.api.SetPreferredModelPair(a.ctx, ws, scope, selected); err != nil {
			return fmt.Errorf("apply Crush model selection: %w", err)
		}
	}

	// Crush keeps a workspace agent uninitialized until /agent/init is called.
	// Ensure it exists after the effective provider/model configuration is ready;
	// already-ready coordinators are preserved and refresh models per run.
	if provider != "" && modelID != "" {
		if err := svc.api.EnsureAgent(a.ctx, ws, true); err != nil {
			return fmt.Errorf("initialize Crush agent: %w", err)
		}
	}
	return nil
}

// needWorkspace turns a missing workspace into a no-op unless the caller was
// trying to store a credential, which must not be silently dropped.
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
