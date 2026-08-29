package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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

// ListProviders returns the live provider and model catalog from Crush. When
// no user workspace is open, it uses a private workspace under Gotack's config
// directory without changing the user's current workspace.
func (a *App) ListProviders() ([]crushapi.Provider, error) {
	svc, err := a.services()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(a.ctx, 90*time.Second)
	defer cancel()

	var workspaceID string
	if desc, ok := svc.ws.Current(); ok {
		workspaceID = desc.WorkspaceID
	}
	if workspaceID == "" {
		catalogPath := filepath.Join(appconfig.Dir(), "catalog-workspace")
		if err := os.MkdirAll(catalogPath, 0o755); err != nil {
			return nil, fmt.Errorf("create catalog workspace directory: %w", err)
		}
		workspace, err := svc.api.CreateWorkspace(ctx, catalogPath, false)
		if err != nil {
			return nil, fmt.Errorf("create catalog workspace: %w", err)
		}
		workspaceID = workspace.ID
	}
	return svc.api.ListProviders(ctx, workspaceID)
}

// SaveSettings persists non-secret UI preferences and applies agent-affecting
// settings through Crush's REST API. API keys are sent directly to Crush's
// provider-key endpoint and are never written to Gotack's config.json.
func (a *App) SaveSettings(s SettingsInfo) error {
	apiKey := strings.TrimSpace(s.APIKey)
	if err := a.applyCrushSettings(s, apiKey); err != nil {
		return err
	}

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

	return appconfig.Save(&cfgCopy)
}
