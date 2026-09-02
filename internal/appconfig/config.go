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

type Config struct {
	Theme            string   `json:"theme"`
	EngineBinary     string   `json:"engine_binary"`
	RecentWorkspaces []string `json:"recent_workspaces"`
	Debug            bool     `json:"debug"`

	AutoApprove       bool                               `json:"auto_approve,omitempty"`
	Provider          string                             `json:"provider,omitempty"`
	Model             string                             `json:"model,omitempty"`
	Thinking          string                             `json:"thinking,omitempty"`
	APIKey            string                             `json:"api_key,omitempty"`
	CustomURL         string                             `json:"custom_url,omitempty"`
	ModelCapabilities map[string]ModelCapabilityOverride `json:"model_capabilities,omitempty"`
	Zalo              ZaloSettings                       `json:"zalo,omitempty"`
}

type ModelCapabilityOverride struct {
	SupportsVision *bool `json:"supports_vision,omitempty"`
	CanReason      *bool `json:"can_reason,omitempty"`
}

type ZaloSettings struct {
	Enabled bool `json:"enabled,omitempty"`
	// Token carried the bot token before the channel state file existed.
	//
	// Deprecated: the channel state file (<configDir>/zalo.json) owns the
	// token now; this field is consumed only once at startup by
	// zalo.Manager.ImportLegacy. Removal target: Gotack v1.0 — drop the
	// field, the ImportLegacy call site, and ImportLegacy together.
	Token string `json:"token,omitempty"`
	// AllowedChats carried the pre-pairing allow-list.
	//
	// Deprecated: paired chats live in the channel state file now; this
	// field is consumed only once at startup by zalo.Manager.ImportLegacy.
	// Removal target: Gotack v1.0, together with Token.
	AllowedChats []string `json:"allowed_chats,omitempty"`
}

func Defaults() *Config {
	return &Config{
		Theme: "system",
	}
}

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

func AddRecentWorkspace(cfg *Config, path string) {
	if cfg == nil || path == "" {
		return
	}

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

func LogDir() string {
	base, err := os.UserCacheDir()
	if err != nil || base == "" {
		base = os.TempDir()
	}
	dir := filepath.Join(base, "gotack")

	_ = os.MkdirAll(dir, 0o755)
	return dir
}
