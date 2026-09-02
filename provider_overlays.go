package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/Dyu-36/gotack/internal/crushapi"
)

const (
	mistralProviderID      = "mistral"
	mistralDefaultEndpoint = "https://api.mistral.ai/v1"
	openAICompatType       = "openai-compat"
)

type localEngineModel struct {
	ID                     string   `json:"id"`
	Name                   string   `json:"name"`
	ContextWindow          int64    `json:"context_window,omitempty"`
	DefaultMaxTokens       int64    `json:"default_max_tokens,omitempty"`
	CanReason              bool     `json:"can_reason"`
	ReasoningLevels        []string `json:"reasoning_levels,omitempty"`
	DefaultReasoningEffort string   `json:"default_reasoning_effort,omitempty"`
	SupportsAttachments    bool     `json:"supports_attachments"`
}

type localProviderSpec struct {
	Provider       crushapi.Provider
	APIKeyTemplate string

	OAuthOnly bool
}

func mistralProviderSpec() localProviderSpec {

	models := []crushapi.Model{
		{
			ID:             "mistral-medium-3-5",
			Name:           "Mistral Medium 3.5",
			ContextWindow:  262144,
			SupportsVision: true,
			Modalities:     []string{"text", "image"},
		},
		{
			ID:             "mistral-small-2603",
			Name:           "Mistral Small 4",
			ContextWindow:  262144,
			SupportsVision: true,
			Modalities:     []string{"text", "image"},
		},
		{
			ID:             "mistral-large-2512",
			Name:           "Mistral Large 3",
			ContextWindow:  262144,
			SupportsVision: true,
			Modalities:     []string{"text", "image"},
		},
	}
	return localProviderSpec{
		Provider: crushapi.Provider{
			ID:                  mistralProviderID,
			Name:                "Mistral AI",
			Type:                openAICompatType,
			APIEndpoint:         mistralDefaultEndpoint,
			DefaultLargeModelID: "mistral-medium-3-5",
			DefaultSmallModelID: "mistral-small-2603",
			Models:              models,
		},
		APIKeyTemplate: "$MISTRAL_API_KEY",
	}
}

func localProviderSpecFor(providerID string) (localProviderSpec, bool) {
	switch providerID {
	case mistralProviderID:
		return mistralProviderSpec(), true
	case codexProviderID:
		return codexProviderSpec(), true
	default:
		return localProviderSpec{}, false
	}
}

func mergeLocalProviderOverlays(providers []crushapi.Provider) ([]crushapi.Provider, map[string]bool) {
	seen := make(map[string]bool, len(providers))
	for _, provider := range providers {
		seen[provider.ID] = true
	}

	overlaid := make(map[string]bool)
	for _, providerID := range []string{mistralProviderID, codexProviderID} {
		if seen[providerID] {
			continue
		}
		spec, _ := localProviderSpecFor(providerID)
		providers = append(providers, spec.Provider)
		overlaid[providerID] = true
	}
	return providers, overlaid
}

func mergeProviderModels(primary, fallback []crushapi.Model) []crushapi.Model {
	result := make([]crushapi.Model, 0, len(primary)+len(fallback))
	seen := make(map[string]bool, len(primary)+len(fallback))
	for _, models := range [][]crushapi.Model{primary, fallback} {
		for _, model := range models {
			if model.ID == "" || seen[model.ID] {
				continue
			}
			seen[model.ID] = true
			result = append(result, model)
		}
	}
	return result
}

func engineModelsFor(spec localProviderSpec) []localEngineModel {
	models := make([]localEngineModel, 0, len(spec.Provider.Models))
	for _, model := range spec.Provider.Models {
		models = append(models, localEngineModel{
			ID:                     model.ID,
			Name:                   model.Name,
			ContextWindow:          model.ContextWindow,
			DefaultMaxTokens:       model.DefaultMaxTokens,
			CanReason:              model.CanReason,
			ReasoningLevels:        model.ReasoningLevels,
			DefaultReasoningEffort: model.DefaultReasoningEffort,
			SupportsAttachments:    model.SupportsVision,
		})
	}
	return models
}

func localProviderConfigFields(spec localProviderSpec) map[string]any {
	base := "providers." + spec.Provider.ID
	fields := map[string]any{
		base + ".discover_models": false,
		base + ".name":            spec.Provider.Name,
		base + ".type":            spec.Provider.Type,
		base + ".base_url":        spec.Provider.APIEndpoint,
	}
	if len(spec.Provider.Models) > 0 {
		fields[base+".models"] = engineModelsFor(spec)
	}
	if spec.APIKeyTemplate != "" {
		fields[base+".api_key"] = spec.APIKeyTemplate
	}
	return fields
}

func prepareLocalProviderConfig(ctx context.Context, api *crushapi.Client, wsID string, scope int, providerID string) (bool, error) {
	spec, supported := localProviderSpecFor(providerID)
	if !supported {
		return false, nil
	}

	cfg, err := api.GetWorkspaceConfig(ctx, wsID)
	if err != nil {
		return false, fmt.Errorf("read provider config before local bootstrap: %w", err)
	}
	if configured, exists := cfg.Providers[providerID]; exists {

		return localProviderIdentityMatches(configured, spec), nil
	}

	upstream, err := api.ListProviders(ctx, wsID)
	if err != nil {
		return false, fmt.Errorf("check upstream provider catalog: %w", err)
	}
	for _, provider := range upstream {
		if provider.ID == providerID {
			return false, nil
		}
	}

	if err := api.SetConfigFields(ctx, wsID, scope, localProviderConfigFields(spec)); err != nil {
		return false, fmt.Errorf("seed local provider %s: %w", providerID, err)
	}
	return true, nil
}

func localProviderIdentityMatches(configured crushapi.ProviderConfig, spec localProviderSpec) bool {
	return strings.TrimSpace(configured.Name) == spec.Provider.Name &&
		strings.TrimSpace(configured.Type) == spec.Provider.Type
}

func finalizeLocalProviderConfig(ctx context.Context, api *crushapi.Client, wsID string, scope int, providerID string) error {
	spec, supported := localProviderSpecFor(providerID)
	if !supported {
		return nil
	}
	if spec.OAuthOnly {

		return nil
	}
	if err := api.SetConfigField(ctx, wsID, scope, "providers."+providerID+".discover_models", true); err != nil {
		return fmt.Errorf("enable model discovery for local provider %s: %w", providerID, err)
	}
	return nil
}
