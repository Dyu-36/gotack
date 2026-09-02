package crushapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

type Provider struct {
	ID                  string  `json:"id"`
	Name                string  `json:"name"`
	Type                string  `json:"type,omitempty"`
	APIEndpoint         string  `json:"api_endpoint,omitempty"`
	DefaultLargeModelID string  `json:"default_large_model_id,omitempty"`
	DefaultSmallModelID string  `json:"default_small_model_id,omitempty"`
	Models              []Model `json:"models,omitempty"`

	Configured     bool   `json:"configured"`
	CredentialKind string `json:"credential_kind,omitempty"`
}

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
