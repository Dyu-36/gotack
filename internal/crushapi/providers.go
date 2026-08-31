package crushapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
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

// InferModelVision returns true if the model is recognized as vision/multimodal capable.
func InferModelVision(providerID, modelID string) bool {
	m := strings.ToLower(strings.TrimSpace(modelID))
	p := strings.ToLower(strings.TrimSpace(providerID))

	// Generic suffixes / prefixes
	if strings.Contains(m, "vision") || strings.Contains(m, "-vl") || strings.Contains(m, "vl-") || strings.HasSuffix(m, "-vl") {
		return true
	}
	// OpenAI models with vision
	if strings.HasPrefix(m, "gpt-4o") || strings.HasPrefix(m, "gpt-4-turbo") || strings.HasPrefix(m, "gpt-4.5") || strings.Contains(m, "gpt-5") || strings.HasPrefix(m, "o1") || strings.HasPrefix(m, "o3") {
		return true
	}
	// Anthropic Claude 3 / 3.5 / 3.7 / 4
	if strings.HasPrefix(m, "claude-3") || strings.HasPrefix(m, "claude-4") {
		return true
	}
	// Google Gemini
	if strings.Contains(m, "gemini") {
		return true
	}
	// MiniMax multimodal (MiniMax-M3, MiniMax-VL-01, abab6.5g, abab7)
	if strings.Contains(m, "minimax-m3") || strings.Contains(m, "minimax-vl") || strings.Contains(m, "abab6.5g") || strings.Contains(m, "abab7") || (p == "minimax" && strings.Contains(m, "m3")) {
		return true
	}
	// Qwen VL
	if strings.Contains(m, "qwen") && strings.Contains(m, "vl") {
		return true
	}
	// Mistral / Pixtral
	if strings.Contains(m, "pixtral") {
		return true
	}
	// Zhipu / GLM-4V
	if strings.Contains(m, "glm-4v") {
		return true
	}
	// Llama 3.2 Vision
	if strings.Contains(m, "llama-3.2") && (strings.Contains(m, "11b") || strings.Contains(m, "90b") || strings.Contains(m, "vision")) {
		return true
	}
	return false
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
	for i := range providers {
		for j := range providers[i].Models {
			m := &providers[i].Models[j]
			m.SupportsVision = m.SupportsVision || InferModelVision(providers[i].ID, m.ID)
		}
	}
	return providers, nil
}
