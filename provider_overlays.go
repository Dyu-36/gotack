package main

import (
	"context"
	"fmt"

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
	// OAuthOnly marks a provider whose only credential is an OAuth login.
	// Seeding must not invent an API key template for it, and model discovery
	// stays off because its catalog is account-scoped and written by the engine
	// when the token is stored.
	OAuthOnly bool
}

// providerConfigWrite is one `providers.<id>.<field>` upsert against Crush's
// config API. Seeding is expressed as an ordered list so the write order stays
// reviewable: identity first, credential last.
type providerConfigWrite struct {
	key   string
	value any
}

func mistralProviderSpec() localProviderSpec {
	// Pin the current GA model IDs so curated capability metadata cannot drift
	// when a `-latest` alias moves. Crush model discovery appends any additional
	// account-available models after the provider is configured.
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

// mergeLocalProviderOverlays appends providers Gotack supports locally only
// when Crush/Catwalk does not already advertise the same provider ID. The
// returned set identifies which entries came from the local overlay so callers
// may safely enrich only those entries from effective custom-provider config.
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

// prepareLocalProviderConfig seeds a custom provider only while Catwalk does
// not know that provider ID. Once Catwalk publishes Mistral, the upstream entry
// wins and this function becomes a no-op without requiring a migration.
//
// It returns true only when a fresh local provider was seeded. The caller then
// stores the credential/custom endpoint and calls finalizeLocalProviderConfig,
// which enables discovery after credentials are available.
func prepareLocalProviderConfig(ctx context.Context, api *crushapi.Client, wsID string, scope int, providerID string) (bool, error) {
	spec, supported := localProviderSpecFor(providerID)
	if !supported {
		return false, nil
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

	cfg, err := api.GetWorkspaceConfig(ctx, wsID)
	if err != nil {
		return false, fmt.Errorf("read provider config before local bootstrap: %w", err)
	}
	if _, exists := cfg.Providers[providerID]; exists {
		// Preserve an existing hand-written custom provider. The Gotack overlay
		// only supplies UI metadata for it; it does not take ownership of config.
		return false, nil
	}

	base := "providers." + providerID
	writes := []providerConfigWrite{
		{base + ".discover_models", false},
		{base + ".name", spec.Provider.Name},
		{base + ".type", spec.Provider.Type},
	}
	// An OAuth-only provider carries neither a curated catalog nor a key
	// template. Writing an empty models list would erase the account-scoped
	// catalog the engine stores with the token, and an empty api_key would read
	// back as a configured credential in Settings.
	if len(spec.Provider.Models) > 0 {
		writes = append(writes, providerConfigWrite{base + ".models", engineModelsFor(spec)})
	}
	writes = append(writes, providerConfigWrite{base + ".base_url", spec.Provider.APIEndpoint})
	if spec.APIKeyTemplate != "" {
		writes = append(writes, providerConfigWrite{base + ".api_key", spec.APIKeyTemplate})
	}
	for _, write := range writes {
		if err := api.SetConfigField(ctx, wsID, scope, write.key, write.value); err != nil {
			return false, fmt.Errorf("seed local provider %s: %w", providerID, err)
		}
	}
	return true, nil
}

func finalizeLocalProviderConfig(ctx context.Context, api *crushapi.Client, wsID string, scope int, providerID string) error {
	spec, supported := localProviderSpecFor(providerID)
	if !supported {
		return nil
	}
	if spec.OAuthOnly {
		// The account-scoped catalog arrives with the OAuth token. Turning
		// discovery on would ask the Codex backend for a model list it does not
		// serve, and a failed discovery must not replace that catalog.
		return nil
	}
	if err := api.SetConfigField(ctx, wsID, scope, "providers."+providerID+".discover_models", true); err != nil {
		return fmt.Errorf("enable model discovery for local provider %s: %w", providerID, err)
	}
	return nil
}
