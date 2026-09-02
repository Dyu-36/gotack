package crushapi

import (
	"context"
	"encoding/json"
	"errors"
)

type WorkspaceConfig struct {
	Providers map[string]ProviderConfig `json:"providers,omitempty"`
	Models    map[string]SelectedModel  `json:"models,omitempty"`
	Options   *WorkspaceOptions         `json:"options,omitempty"`
	Env       map[string]string         `json:"env,omitempty"`

	Hooks map[string][]HookEntry `json:"hooks,omitempty"`
}

type WorkspaceOptions struct {
	SkillsPaths []string `json:"skills_paths,omitempty"`
}

func (c WorkspaceConfig) SkillsPaths() []string {
	if c.Options == nil {
		return nil
	}
	return c.Options.SkillsPaths
}

type HookEntry struct {
	Name    string `json:"name,omitempty"`
	Matcher string `json:"matcher,omitempty"`
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"`
}

type ProviderConfig struct {
	ID      string          `json:"id,omitempty"`
	Name    string          `json:"name,omitempty"`
	BaseURL string          `json:"base_url,omitempty"`
	Type    string          `json:"type,omitempty"`
	APIKey  string          `json:"api_key,omitempty"`
	OAuth   json.RawMessage `json:"oauth,omitempty"`
	Disable bool            `json:"disable,omitempty"`
	Models  []Model         `json:"models,omitempty"`
}

func (c *Client) GetWorkspaceConfig(ctx context.Context, wsID string) (WorkspaceConfig, error) {
	if wsID == "" {
		return WorkspaceConfig{}, errors.New("crushapi: workspace id is required")
	}
	var cfg WorkspaceConfig
	if err := c.doJSON(ctx, "GET", expandPath("/v1/workspaces/{id}/config", "id", wsID), nil, &cfg); err != nil {
		return WorkspaceConfig{}, err
	}
	return cfg, nil
}
