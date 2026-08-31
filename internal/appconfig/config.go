package appconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	configFileName = "config.json"
	maxRecent      = 10
)

// config.go -- role: typed settings model plus load and save.
//
// UI preferences (theme), engine preferences (autostart, endpoint override) and
// recent workspaces. Persisted as JSON inside the user config directory.

// Config is the on-disk user settings model. Empty fields fall back to
// Defaults() at load time; EngineBinary empty means PATH lookup.
type Config struct {
	Theme             string                             `json:"theme"`
	EngineBinary      string                             `json:"engine_binary"` // path to crush executable, empty = PATH lookup
	AutostartEngine   bool                               `json:"autostart_engine"`
	RecentWorkspaces  []string                           `json:"recent_workspaces"`
	Debug             bool                               `json:"debug"`
	Provider          string                             `json:"provider,omitempty"`
	Model             string                             `json:"model,omitempty"`
	SmallModel        string                             `json:"small_model,omitempty"`
	Thinking          string                             `json:"thinking,omitempty"`
	APIKey            string                             `json:"api_key,omitempty"`
	CustomURL         string                             `json:"custom_url,omitempty"`
	ModelCapabilities map[string]ModelCapabilityOverride `json:"model_capabilities,omitempty"`
	Zalo              ZaloSettings                       `json:"zalo,omitempty"`
}

// ModelCapabilityOverride allows user-defined or runtime overrides for a model's capabilities.
type ModelCapabilityOverride struct {
	SupportsVision *bool `json:"supports_vision,omitempty"`
	CanReason      *bool `json:"can_reason,omitempty"`
}

// ZaloSettings holds the Zalo Bot API connection. The bot token is a secret
// stored locally on the user's machine; it is never returned to the webview.
type ZaloSettings struct {
	Enabled      bool     `json:"enabled,omitempty"`
	Token        string   `json:"token,omitempty"`
	AllowedChats []string `json:"allowed_chats,omitempty"`
}

// Defaults returns the factory config: system theme, autostart on, and empty
// agent settings so Crush's own catalog defaults apply until the user picks a
// provider and model.
func Defaults() *Config {
	return &Config{
		Theme:           "system",
		AutostartEngine: true,
	}
}

// Load reads the JSON config from Dir()/config.json. A missing file returns
// Defaults with no error; an unreadable or malformed file returns an error.
func Load() (*Config, error) {
	path := filepath.Join(Dir(), configFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Defaults(), nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}
	cfg := Defaults()
	if len(data) == 0 {
		return cfg, nil
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}

// Save writes cfg as pretty JSON to Dir()/config.json, creating the directory
// if it does not exist.
func Save(cfg *Config) error {
	if cfg == nil {
		return errors.New("nil config")
	}
	dir := Dir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir config dir: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	path := filepath.Join(dir, configFileName)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// AddRecentWorkspace moves path to the front of cfg.RecentWorkspaces, removes
// any prior equal entry, and caps the list at maxRecent entries. Comparison is
// on filepath.Clean(path) so trailing separators and redundant slashes do not
// produce duplicates. Empty path is ignored.
func AddRecentWorkspace(cfg *Config, path string) {
	if cfg == nil || path == "" {
		return
	}
	// ToSlash keeps the stored form stable across platforms so a config
	// synced or compared between OSes matches; Windows accepts both forms.
	cleaned := filepath.ToSlash(filepath.Clean(path))
	out := make([]string, 0, maxRecent)
	out = append(out, cleaned)
	for _, p := range cfg.RecentWorkspaces {
		if p == cleaned || filepath.ToSlash(filepath.Clean(p)) == cleaned {
			continue
		}
		out = append(out, p)
		if len(out) >= maxRecent {
			break
		}
	}
	cfg.RecentWorkspaces = out
}

// LogDir returns the per-user log directory under os.UserCacheDir() and
// ensures it exists. Shared with the logging package.
func LogDir() string {
	base, err := os.UserCacheDir()
	if err != nil || base == "" {
		base = os.TempDir()
	}
	dir := filepath.Join(base, "gotack")
	// best-effort: ignore mkdir failure and let the caller log the issue.
	_ = os.MkdirAll(dir, 0o755)
	return dir
}
