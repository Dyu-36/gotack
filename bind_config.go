package main

import (
	"github.com/Dyu-36/gotack/internal/appconfig"
)

// bind_config.go -- role: Wails-bound API for user settings, theme, provider and model configuration.

// SettingsInfo describes the application settings for UI consumption.
type SettingsInfo struct {
	Theme           string `json:"theme"`
	AutostartEngine bool   `json:"autostart_engine"`
	Provider        string `json:"provider"`
	Model           string `json:"model"`
	Thinking        string `json:"thinking"`
	APIKey          string `json:"api_key"`
	CustomURL       string `json:"custom_url"`
}

// GetSettings returns current application settings.
func (a *App) GetSettings() SettingsInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.cfg == nil {
		return SettingsInfo{
			Theme:           "system",
			AutostartEngine: true,
			Provider:        "crush",
			Model:           "crush-default",
			Thinking:        "auto",
		}
	}
	return SettingsInfo{
		Theme:           a.cfg.Theme,
		AutostartEngine: a.cfg.AutostartEngine,
		Provider:        a.cfg.Provider,
		Model:           a.cfg.Model,
		Thinking:        a.cfg.Thinking,
		APIKey:          a.cfg.APIKey,
		CustomURL:       a.cfg.CustomURL,
	}
}

// SaveSettings updates and persists application settings.
func (a *App) SaveSettings(s SettingsInfo) error {
	a.mu.Lock()
	if a.cfg == nil {
		a.cfg = appconfig.Defaults()
	}
	if s.Theme != "" {
		a.cfg.Theme = s.Theme
	}
	a.cfg.AutostartEngine = s.AutostartEngine
	a.cfg.Provider = s.Provider
	a.cfg.Model = s.Model
	a.cfg.Thinking = s.Thinking
	a.cfg.APIKey = s.APIKey
	a.cfg.CustomURL = s.CustomURL
	cfgCopy := *a.cfg
	a.mu.Unlock()

	return appconfig.Save(&cfgCopy)
}
