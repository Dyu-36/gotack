package main

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/crushapi"
)

// bind_config.go -- role: Wails-bound API for user settings, theme, provider
// and model configuration.

type SettingsInfo struct {
	Theme           string `json:"theme"`
	AutostartEngine bool   `json:"autostart_engine"`
	Provider        string `json:"provider"`
	Model           string `json:"model"`
	SmallModel      string `json:"small_model"`
	Thinking        string `json:"thinking"`
	// APIKey is write-only from the UI. GetSettings always returns an empty
	// value so a credential is never round-tripped through Wails state.
	APIKey    string `json:"api_key"`
	CustomURL string `json:"custom_url"`
}

// GetSettings returns non-secret Gotack preferences. Provider credentials are
// owned by Crush and deliberately never returned to the webview.
func (a *App) GetSettings() SettingsInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.cfg == nil {
		return SettingsInfo{Theme: "system", AutostartEngine: true}
	}
	return SettingsInfo{
		Theme:           a.cfg.Theme,
		AutostartEngine: a.cfg.AutostartEngine,
		Provider:        a.cfg.Provider,
		Model:           a.cfg.Model,
		SmallModel:      a.cfg.SmallModel,
		Thinking:        a.cfg.Thinking,
		APIKey:          "",
		CustomURL:       a.cfg.CustomURL,
	}
}

// ListProviders returns the live provider and model catalog from Crush for
// the open workspace. The deadline is generous because the engine may refresh
// its catalog from the network on a cold cache.
func (a *App) ListProviders() ([]crushapi.Provider, error) {
	svc, err := a.services()
	if err != nil {
		return nil, err
	}
	desc, ok := svc.ws.Current()
	if !ok {
		return nil, errors.New("no workspace is open")
	}
	ctx, cancel := context.WithTimeout(a.ctx, 90*time.Second)
	defer cancel()
	return svc.api.ListProviders(ctx, desc.WorkspaceID)
}

// SaveSettings persists non-secret UI preferences and applies agent-affecting
// settings through Crush's REST API. API keys are sent directly to Crush's
// provider-key endpoint and are never written to Gotack's config.json.
func (a *App) SaveSettings(s SettingsInfo) error {
	apiKey := strings.TrimSpace(s.APIKey)
	if err := a.applyCrushSettings(s, apiKey); err != nil {
		return err
	}

	a.mu.Lock()
	if a.cfg == nil {
		a.cfg = appconfig.Defaults()
	}
	if s.Theme != "" {
		a.cfg.Theme = s.Theme
	}
	a.cfg.AutostartEngine = s.AutostartEngine
	a.cfg.Provider = strings.TrimSpace(s.Provider)
	a.cfg.Model = strings.TrimSpace(s.Model)
	a.cfg.SmallModel = strings.TrimSpace(s.SmallModel)
	a.cfg.Thinking = strings.TrimSpace(s.Thinking)
	a.cfg.APIKey = "" // scrub any credential persisted by older builds
	a.cfg.CustomURL = strings.TrimSpace(s.CustomURL)
	cfgCopy := *a.cfg
	a.mu.Unlock()

	return appconfig.Save(&cfgCopy)
}
