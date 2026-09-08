package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/Dyu-36/gotack/internal/engineapi"
)

const (
	OpenAIID        = "openai"
	CodexID         = "codex"
	CodexName       = "ChatGPT (Codex)"
	CodexBackendURL = "https://chatgpt.com/backend-api/codex"
	CodexType       = "openai"

	MistralID              = "mistral"
	MistralDefaultEndpoint = "https://api.mistral.ai/v1"
	OpenAICompatType       = "openai-compat"
)

type LocalEngineModel struct {
	ID                     string   `json:"id"`
	Name                   string   `json:"name"`
	ContextWindow          int64    `json:"context_window,omitempty"`
	DefaultMaxTokens       int64    `json:"default_max_tokens,omitempty"`
	CanReason              bool     `json:"can_reason"`
	ReasoningLevels        []string `json:"reasoning_levels,omitempty"`
	DefaultReasoningEffort string   `json:"default_reasoning_effort,omitempty"`
	SupportsAttachments    bool     `json:"supports_attachments"`
}

type LocalSpec struct {
	Provider       engineapi.Provider
	APIKeyTemplate string
	OAuthOnly      bool
}

func CodexSpec() LocalSpec {
	return LocalSpec{
		Provider: engineapi.Provider{
			ID:          CodexID,
			Name:        CodexName,
			Type:        CodexType,
			APIEndpoint: CodexBackendURL,
		},
		OAuthOnly: true,
	}
}

func MistralSpec() LocalSpec {
	models := []engineapi.Model{
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
	return LocalSpec{
		Provider: engineapi.Provider{
			ID:                  MistralID,
			Name:                "Mistral AI",
			Type:                OpenAICompatType,
			APIEndpoint:         MistralDefaultEndpoint,
			DefaultLargeModelID: "mistral-medium-3-5",
			DefaultSmallModelID: "mistral-small-2603",
			Models:              models,
		},
		APIKeyTemplate: "$MISTRAL_API_KEY",
	}
}

func localSpecFor(providerID string) (LocalSpec, bool) {
	switch providerID {
	case MistralID:
		return MistralSpec(), true
	case CodexID:
		return CodexSpec(), true
	default:
		return LocalSpec{}, false
	}
}

func MergeLocalOverlays(providers []engineapi.Provider) ([]engineapi.Provider, map[string]bool) {
	seen := make(map[string]bool, len(providers))
	for _, candidate := range providers {
		seen[candidate.ID] = true
	}

	overlaid := make(map[string]bool)
	for _, providerID := range []string{MistralID, CodexID} {
		if seen[providerID] {
			continue
		}
		spec, _ := localSpecFor(providerID)
		providers = append(providers, spec.Provider)
		overlaid[providerID] = true
	}
	return providers, overlaid
}

func MergeModels(primary, fallback []engineapi.Model) []engineapi.Model {
	result := make([]engineapi.Model, 0, len(primary)+len(fallback))
	seen := make(map[string]bool, len(primary)+len(fallback))
	for _, models := range [][]engineapi.Model{primary, fallback} {
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

func engineModelsFor(spec LocalSpec) []LocalEngineModel {
	models := make([]LocalEngineModel, 0, len(spec.Provider.Models))
	for _, model := range spec.Provider.Models {
		models = append(models, LocalEngineModel{
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

func ConfigFields(spec LocalSpec) map[string]any {
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

func PrepareLocal(ctx context.Context, api *engineapi.Client, workspaceID string, scope int, providerID string) (bool, error) {
	spec, supported := localSpecFor(providerID)
	if !supported {
		return false, nil
	}

	cfg, err := api.GetWorkspaceConfig(ctx, workspaceID)
	if err != nil {
		return false, fmt.Errorf("read provider config before local bootstrap: %w", err)
	}
	if configured, exists := cfg.Providers[providerID]; exists {
		return IdentityMatches(configured, spec), nil
	}

	upstream, err := api.ListProviders(ctx, workspaceID)
	if err != nil {
		return false, fmt.Errorf("check upstream provider catalog: %w", err)
	}
	for _, candidate := range upstream {
		if candidate.ID == providerID {
			return false, nil
		}
	}

	if err := api.SetConfigFields(ctx, workspaceID, scope, ConfigFields(spec)); err != nil {
		return false, fmt.Errorf("seed local provider %s: %w", providerID, err)
	}
	return true, nil
}

func IdentityMatches(configured engineapi.ProviderConfig, spec LocalSpec) bool {
	return strings.TrimSpace(configured.Name) == spec.Provider.Name &&
		strings.TrimSpace(configured.Type) == spec.Provider.Type
}

func FinalizeLocal(ctx context.Context, api *engineapi.Client, workspaceID string, scope int, providerID string) error {
	spec, supported := localSpecFor(providerID)
	if !supported || spec.OAuthOnly {
		return nil
	}
	if err := api.SetConfigField(ctx, workspaceID, scope, "providers."+providerID+".discover_models", true); err != nil {
		return fmt.Errorf("enable model discovery for local provider %s: %w", providerID, err)
	}
	return nil
}

func ListCatalog(ctx context.Context, api *engineapi.Client, workspaceID string) ([]engineapi.Provider, error) {
	providers, err := api.ListProviders(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	providers, localOverlays := MergeLocalOverlays(providers)
	cfg, err := api.GetWorkspaceConfig(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("get resolved engine config: %w", err)
	}
	for i := range providers {
		configured, exists := cfg.Providers[providers[i].ID]
		if !exists || configured.Disable {
			continue
		}
		if localOverlays[providers[i].ID] {
			if configured.Name != "" {
				providers[i].Name = configured.Name
			}
			if configured.Type != "" {
				providers[i].Type = configured.Type
			}
			if configured.BaseURL != "" {
				providers[i].APIEndpoint = configured.BaseURL
			}
			if len(configured.Models) > 0 {
				providers[i].Models = MergeModels(configured.Models, providers[i].Models)
			}
		}
		kind, _, usable := ResolvedCredential(configured)
		if !usable {
			continue
		}
		providers[i].Configured = true
		providers[i].CredentialKind = kind
	}
	return providers, nil
}
