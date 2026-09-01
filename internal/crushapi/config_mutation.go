package crushapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// config_mutation.go mirrors the supported Crush v1 configuration mutation
// endpoints without importing upstream internal packages. The wire shapes are
// pinned against the upstream commit recorded in third_party/README.md.

const (
	configSetPath         = "/v1/workspaces/{id}/config/set"
	configRemovePath      = "/v1/workspaces/{id}/config/remove"
	configModelPath       = "/v1/workspaces/{id}/config/model"
	configProviderKeyPath = "/v1/workspaces/{id}/config/provider-key"
)

// Config scope values are defined by Crush as global=0 and workspace=1.
const (
	ConfigScopeGlobal    = 0
	ConfigScopeWorkspace = 1
)

// SelectedModel is Crush config.SelectedModel's UI-relevant wire shape.
type SelectedModel struct {
	Model           string `json:"model"`
	Provider        string `json:"provider"`
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
	Think           bool   `json:"think,omitempty"`
}

// SetPreferredModel updates either the large or small preferred model. Changes
// are hot-reloaded by Crush and published as config_changed to subscribers.
func (c *Client) SetPreferredModel(ctx context.Context, wsID string, scope int, modelType string, model SelectedModel) error {
	if wsID == "" {
		return errors.New("crushapi: workspace id is required")
	}
	if modelType != "large" && modelType != "small" {
		return fmt.Errorf("crushapi: invalid model type %q", modelType)
	}
	if strings.TrimSpace(model.Provider) == "" || strings.TrimSpace(model.Model) == "" {
		return errors.New("crushapi: provider and model are required")
	}
	body, err := json.Marshal(struct {
		Scope     int           `json:"scope"`
		ModelType string        `json:"model_type"`
		Model     SelectedModel `json:"model"`
	}{Scope: scope, ModelType: modelType, Model: model})
	if err != nil {
		return fmt.Errorf("crushapi: encode preferred model: %w", err)
	}
	return c.doJSON(ctx, "POST", expandPath(configModelPath, "id", wsID), bytes.NewReader(body), nil)
}

// SetProviderAPIKey stores a plain string provider credential through Crush's
// typed credential endpoint. The value is never logged by this package.
func (c *Client) SetProviderAPIKey(ctx context.Context, wsID string, scope int, providerID, apiKey string) error {
	if wsID == "" || strings.TrimSpace(providerID) == "" {
		return errors.New("crushapi: workspace id and provider id are required")
	}
	raw, err := json.Marshal(apiKey)
	if err != nil {
		return fmt.Errorf("crushapi: encode provider key: %w", err)
	}
	body, err := json.Marshal(struct {
		Scope      int             `json:"scope"`
		ProviderID string          `json:"provider_id"`
		Kind       string          `json:"kind"`
		APIKey     json.RawMessage `json:"api_key"`
	}{Scope: scope, ProviderID: providerID, Kind: "string", APIKey: raw})
	if err != nil {
		return fmt.Errorf("crushapi: encode provider key request: %w", err)
	}
	return c.doJSON(ctx, "POST", expandPath(configProviderKeyPath, "id", wsID), bytes.NewReader(body), nil)
}

// SetProviderOAuthToken stores an OAuth token credential through Crush's
// typed credential endpoint.
func (c *Client) SetProviderOAuthToken(ctx context.Context, wsID string, scope int, providerID string, token any) error {
	if wsID == "" || strings.TrimSpace(providerID) == "" {
		return errors.New("crushapi: workspace id and provider id are required")
	}
	raw, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("crushapi: encode provider oauth token: %w", err)
	}
	body, err := json.Marshal(struct {
		Scope      int             `json:"scope"`
		ProviderID string          `json:"provider_id"`
		Kind       string          `json:"kind"`
		APIKey     json.RawMessage `json:"api_key"`
	}{Scope: scope, ProviderID: providerID, Kind: "oauth", APIKey: raw})
	if err != nil {
		return fmt.Errorf("crushapi: encode provider oauth key request: %w", err)
	}
	return c.doJSON(ctx, "POST", expandPath(configProviderKeyPath, "id", wsID), bytes.NewReader(body), nil)
}


// SetConfigField writes one Crush config field using the server's sjson path
// semantics. It is used for provider base_url, which Crush hot-reloads.
func (c *Client) SetConfigField(ctx context.Context, wsID string, scope int, key string, value any) error {
	if wsID == "" || strings.TrimSpace(key) == "" {
		return errors.New("crushapi: workspace id and config key are required")
	}
	body, err := json.Marshal(struct {
		Scope int    `json:"scope"`
		Key   string `json:"key"`
		Value any    `json:"value"`
	}{Scope: scope, Key: key, Value: value})
	if err != nil {
		return fmt.Errorf("crushapi: encode config field: %w", err)
	}
	return c.doJSON(ctx, "POST", expandPath(configSetPath, "id", wsID), bytes.NewReader(body), nil)
}

// RemoveConfigField deletes one Crush config field, for example a registered
// MCP server entry. Deleting an absent key is not an error server-side.
func (c *Client) RemoveConfigField(ctx context.Context, wsID string, scope int, key string) error {
	if wsID == "" || strings.TrimSpace(key) == "" {
		return errors.New("crushapi: workspace id and config key are required")
	}
	body, err := json.Marshal(struct {
		Scope int    `json:"scope"`
		Key   string `json:"key"`
	}{Scope: scope, Key: key})
	if err != nil {
		return fmt.Errorf("crushapi: encode config removal: %w", err)
	}
	return c.doJSON(ctx, "POST", expandPath(configRemovePath, "id", wsID), bytes.NewReader(body), nil)
}
