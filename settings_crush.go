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
// Crush workspace. Crush hot-reloads all three endpoints used here, so these
// changes do not require an engine restart on the pinned upstream contract.
// apiKey is intentionally an argument rather than persisted Gotack state.
func (a *App) applyCrushSettings(s SettingsInfo, apiKey string) error {
	svc, err := a.services()
	if err != nil {
		if apiKey != "" {
			return errors.New("cannot store API key until Crush is running and a workspace is open")
		}
		return nil
	}
	desc, ok := svc.ws.Current()
	if !ok || desc.WorkspaceID == "" {
		if apiKey != "" {
			return errors.New("cannot store API key until a workspace is open")
		}
		return nil
	}

	provider := strings.TrimSpace(s.Provider)
	modelID := strings.TrimSpace(s.Model)
	if provider != "" && modelID != "" {
		reasoning, think := crushReasoning(s.Thinking)
		model := crushapi.SelectedModel{
			Provider:        provider,
			Model:           modelID,
			ReasoningEffort: reasoning,
			Think:           think,
		}
		if err := svc.api.SetPreferredModel(a.ctx, desc.WorkspaceID, crushapi.ConfigScopeGlobal, "large", model); err != nil {
			return fmt.Errorf("apply Crush model: %w", err)
		}
	}

	if apiKey != "" {
		if provider == "" {
			return errors.New("provider is required before storing an API key")
		}
		if err := svc.api.SetProviderAPIKey(a.ctx, desc.WorkspaceID, crushapi.ConfigScopeGlobal, provider, apiKey); err != nil {
			return fmt.Errorf("apply Crush provider credential: %w", err)
		}
	}

	if endpoint := strings.TrimSpace(s.CustomURL); endpoint != "" {
		if !safeProviderID.MatchString(provider) {
			return fmt.Errorf("provider id %q cannot be used in a Crush config path", provider)
		}
		key := "providers." + provider + ".base_url"
		if err := svc.api.SetConfigField(a.ctx, desc.WorkspaceID, crushapi.ConfigScopeGlobal, key, endpoint); err != nil {
			return fmt.Errorf("apply Crush provider endpoint: %w", err)
		}
	}
	return nil
}

func crushReasoning(value string) (effort string, think bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "low", "medium", "high":
		return strings.ToLower(strings.TrimSpace(value)), true
	case "max":
		// Crush's SelectedModel currently accepts low/medium/high for the
		// reasoning_effort field. Max therefore maps to the strongest level.
		return "high", true
	case "none", "off", "disabled":
		return "", false
	default:
		// "auto" leaves reasoning_effort unset and lets the provider/model
		// default decide. Think stays false for Anthropic-style providers.
		return "", false
	}
}
