package crushapi

import (
	"context"
	"encoding/json"
	"errors"
)

// WorkspaceConfig is the UI-relevant subset of Crush's resolved workspace
// configuration. The server returns the effective config, so provider entries
// include credentials resolved from global/workspace config and environment.
type WorkspaceConfig struct {
	Providers map[string]ProviderConfig `json:"providers,omitempty"`
}

// ProviderConfig mirrors the provider fields needed by Gotack to decide which
// providers are actually configured and to reveal a credential on explicit
// user request. OAuth is kept raw because Gotack only needs presence here.
type ProviderConfig struct {
	ID      string          `json:"id,omitempty"`
	Name    string          `json:"name,omitempty"`
	BaseURL string          `json:"base_url,omitempty"`
	Type    string          `json:"type,omitempty"`
	APIKey  string          `json:"api_key,omitempty"`
	OAuth   json.RawMessage `json:"oauth,omitempty"`
	Disable bool            `json:"disable,omitempty"`
}

// GetWorkspaceConfig calls GET /v1/workspaces/{id}/config.
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
