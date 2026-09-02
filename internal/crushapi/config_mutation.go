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
	configSetPath          = "/v1/workspaces/{id}/config/set"
	configSetBatchPath     = "/v1/workspaces/{id}/config/set-batch"
	configRemovePath       = "/v1/workspaces/{id}/config/remove"
	configModelsPath       = "/v1/workspaces/{id}/config/models"
	configProviderKeyPath  = "/v1/workspaces/{id}/config/provider-key"
	configRefreshOAuthPath = "/v1/workspaces/{id}/config/refresh-oauth"
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

// SetPreferredModelPair atomically pins Crush's large and small slots to the
// one model Gotack exposes in Settings.
func (c *Client) SetPreferredModelPair(ctx context.Context, wsID string, scope int, model SelectedModel) error {
	if wsID == "" {
		return errors.New("crushapi: workspace id is required")
	}
	if strings.TrimSpace(model.Provider) == "" || strings.TrimSpace(model.Model) == "" {
		return errors.New("crushapi: provider and model are required")
	}
	return c.mutatePreferredModelPair(ctx, wsID, scope, &model)
}

// RemovePreferredModelPair atomically removes both preferred slots. Recent
// model history remains owned by Crush and is deliberately preserved.
func (c *Client) RemovePreferredModelPair(ctx context.Context, wsID string, scope int) error {
	if wsID == "" {
		return errors.New("crushapi: workspace id is required")
	}
	return c.mutatePreferredModelPair(ctx, wsID, scope, nil)
}

func (c *Client) mutatePreferredModelPair(ctx context.Context, wsID string, scope int, model *SelectedModel) error {
	body, err := json.Marshal(struct {
		Scope  int                       `json:"scope"`
		Models map[string]*SelectedModel `json:"models"`
	}{
		Scope: scope,
		Models: map[string]*SelectedModel{
			"large": model,
			"small": model,
		},
	})
	if err != nil {
		return fmt.Errorf("crushapi: encode preferred model pair: %w", err)
	}
	return c.doJSON(ctx, "POST", expandPath(configModelsPath, "id", wsID), bytes.NewReader(body), nil)
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

// RefreshProviderOAuthToken asks Crush to refresh and persist a provider's
// OAuth credential using its provider-specific token exchange.
func (c *Client) RefreshProviderOAuthToken(ctx context.Context, wsID string, scope int, providerID string) error {
	if wsID == "" || strings.TrimSpace(providerID) == "" {
		return errors.New("crushapi: workspace id and provider id are required")
	}
	body, err := json.Marshal(struct {
		Scope      int    `json:"scope"`
		ProviderID string `json:"provider_id"`
	}{Scope: scope, ProviderID: providerID})
	if err != nil {
		return fmt.Errorf("crushapi: encode provider oauth refresh: %w", err)
	}
	return c.doJSON(ctx, "POST", expandPath(configRefreshOAuthPath, "id", wsID), bytes.NewReader(body), nil)
}

// SetConfigField writes one Crush config field using the server's sjson path
// semantics.
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

// SetConfigFields writes related Crush config paths in one server-side
// mutation. Callers use it when a partial provider definition would be invalid.
func (c *Client) SetConfigFields(ctx context.Context, wsID string, scope int, fields map[string]any) error {
	if wsID == "" {
		return errors.New("crushapi: workspace id is required")
	}
	if len(fields) == 0 {
		return errors.New("crushapi: config fields are required")
	}
	for key := range fields {
		if strings.TrimSpace(key) == "" {
			return errors.New("crushapi: config field key is required")
		}
	}
	body, err := json.Marshal(struct {
		Scope  int            `json:"scope"`
		Fields map[string]any `json:"fields"`
	}{Scope: scope, Fields: fields})
	if err != nil {
		return fmt.Errorf("crushapi: encode config fields: %w", err)
	}
	return c.doJSON(ctx, "POST", expandPath(configSetBatchPath, "id", wsID), bytes.NewReader(body), nil)
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
