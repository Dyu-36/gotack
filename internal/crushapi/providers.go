package crushapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// Provider and Model mirror the UI-relevant subset of the catwalk catalog
// served by GET /v1/workspaces/{id}/providers. The engine resolves the full
// catalog; the desktop layer only needs identity, naming and capability
// fields to render provider and model pickers.
type Provider struct {
	ID                  string  `json:"id"`
	Name                string  `json:"name"`
	Type                string  `json:"type,omitempty"`
	APIEndpoint         string  `json:"api_endpoint,omitempty"`
	DefaultLargeModelID string  `json:"default_large_model_id,omitempty"`
	DefaultSmallModelID string  `json:"default_small_model_id,omitempty"`
	Models              []Model `json:"models,omitempty"`
	// Configured is enriched by Gotack from Crush's resolved workspace config.
	// A provider is true only when Crush actually loaded it into the effective
	// config (credentials/endpoint requirements satisfied and not disabled).
	Configured     bool   `json:"configured"`
	CredentialKind string `json:"credential_kind,omitempty"`
}

// Model is one selectable model inside a provider.
type Model struct {
	ID                     string   `json:"id"`
	Name                   string   `json:"name"`
	ContextWindow          int64    `json:"context_window,omitempty"`
	DefaultMaxTokens       int64    `json:"default_max_tokens,omitempty"`
	CanReason              bool     `json:"can_reason"`
	SupportsVision         bool     `json:"supports_vision"`
	Modalities             []string `json:"modalities,omitempty"`
	ReasoningLevels        []string `json:"reasoning_levels,omitempty"`
	DefaultReasoningEffort string   `json:"default_reasoning_effort,omitempty"`
	CostPer1MIn            float64  `json:"cost_per_1m_in,omitempty"`
	CostPer1MOut           float64  `json:"cost_per_1m_out,omitempty"`
}

// UnmarshalJSON accepts both Gotack's UI-facing supports_vision field and
// Catwalk/Crush's native supports_attachments field. Gotack keeps serializing
// supports_vision across Wails, but capability decisions must preserve the
// authoritative value returned by Crush instead of guessing from model names.
func (m *Model) UnmarshalJSON(data []byte) error {
	type modelWire Model
	var decoded modelWire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var capabilities struct {
		SupportsVision      *bool `json:"supports_vision"`
		SupportsAttachments *bool `json:"supports_attachments"`
	}
	if err := json.Unmarshal(data, &capabilities); err != nil {
		return err
	}
	*m = Model(decoded)
	if capabilities.SupportsVision != nil {
		m.SupportsVision = *capabilities.SupportsVision
	} else if capabilities.SupportsAttachments != nil {
		m.SupportsVision = *capabilities.SupportsAttachments
	}
	return nil
}

// ListProviders returns the provider catalog for a workspace. The engine may
// take tens of seconds on a cold cache while it refreshes from the network,
// so callers should pass a context with a generous deadline.
func (c *Client) ListProviders(ctx context.Context, wsID string) ([]Provider, error) {
	if wsID == "" {
		return nil, errors.New("crushapi: workspace id is required")
	}
	resp, err := c.do(ctx, http.MethodGet, "/v1/workspaces/"+wsID+"/providers", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("crushapi: list providers: %s", resp.Status)
	}
	var providers []Provider
	if err := json.NewDecoder(resp.Body).Decode(&providers); err != nil {
		return nil, fmt.Errorf("crushapi: decode providers: %w", err)
	}

	return providers, nil
}
