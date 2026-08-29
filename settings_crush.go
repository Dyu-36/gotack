package main

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/Dyu-36/gotack/internal/crushapi"
)

var safeProviderID = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

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
	ws, scope := desc.WorkspaceID, crushapi.ConfigScopeGlobal
	if apiKey != "" && provider == "" {
		return errors.New("provider is required before storing an API key")
	}

	setModel := func(modelType, modelID string, effort string, think bool) error {
		model := crushapi.SelectedModel{Provider: provider, Model: modelID, ReasoningEffort: effort, Think: think}
		if err := svc.api.SetPreferredModel(a.ctx, ws, scope, modelType, model); err != nil {
			return fmt.Errorf("apply Crush %s model: %w", modelType, err)
		}
		return nil
	}
	if modelID := strings.TrimSpace(s.Model); provider != "" && modelID != "" {
		effort, think := crushReasoning(s.Thinking)
		if err := setModel("large", modelID, effort, think); err != nil {
			return err
		}
	}
	if modelID := strings.TrimSpace(s.SmallModel); provider != "" && modelID != "" {
		if err := setModel("small", modelID, "", false); err != nil {
			return err
		}
	}

	if apiKey != "" {
		if err := svc.api.SetProviderAPIKey(a.ctx, ws, scope, provider, apiKey); err != nil {
			return fmt.Errorf("apply Crush provider credential: %w", err)
		}
	}

	if endpoint := strings.TrimSpace(s.CustomURL); endpoint != "" {
		if !safeProviderID.MatchString(provider) {
			return fmt.Errorf("provider id %q cannot be used in a Crush config path", provider)
		}
		key := "providers." + provider + ".base_url"
		if err := svc.api.SetConfigField(a.ctx, ws, scope, key, endpoint); err != nil {
			return fmt.Errorf("apply Crush provider endpoint: %w", err)
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
	case "low", "medium", "high":
		return v, true
	case "max":
		// Crush's reasoning_effort accepts low/medium/high, so max maps to high.
		return "high", true
	default:
		// none/off/auto-style values leave the provider default in charge.
		return "", false
	}
}
