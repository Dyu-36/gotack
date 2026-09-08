package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Dyu-36/gotack/internal/engineapi"
	"github.com/Dyu-36/gotack/internal/openaioauth"
)

func SeedCodex(ctx context.Context, api *engineapi.Client, workspaceID string, scope int) error {
	fields := ConfigFields(CodexSpec())
	fields["providers."+CodexID+".disable"] = false
	if err := api.SetConfigFields(ctx, workspaceID, scope, fields); err != nil {
		return fmt.Errorf("seed Codex provider: %w", err)
	}
	return nil
}

func MigrateChatGPTOAuthToCodex(ctx context.Context, api *engineapi.Client, workspaceID string) (bool, error) {
	scope := engineapi.ConfigScopeGlobal
	cfg, err := api.GetWorkspaceConfig(ctx, workspaceID)
	if err != nil {
		return false, fmt.Errorf("read provider config before Codex migration: %w", err)
	}
	legacy, exists := cfg.Providers[OpenAIID]
	if !exists || !OAuthCredentialPresent(legacy) {
		return false, nil
	}
	if codex, ok := cfg.Providers[CodexID]; ok && OAuthCredentialPresent(codex) {
		return true, clearLegacyChatGPTCredential(ctx, api, workspaceID, scope, legacy)
	}

	var token openaioauth.Token
	if err := json.Unmarshal(legacy.OAuth, &token); err != nil || token.AccessToken == "" {
		return false, nil
	}
	if err := SeedCodex(ctx, api, workspaceID, scope); err != nil {
		return false, err
	}
	if err := api.SetProviderOAuthToken(ctx, workspaceID, scope, CodexID, &token); err != nil {
		return false, fmt.Errorf("move the ChatGPT credential to Codex: %w", err)
	}
	return true, clearLegacyChatGPTCredential(ctx, api, workspaceID, scope, legacy)
}

func clearLegacyChatGPTCredential(ctx context.Context, api *engineapi.Client, workspaceID string, scope int, legacy engineapi.ProviderConfig) error {
	base := "providers." + OpenAIID
	removals := []string{
		base + ".oauth",
		base + ".models",
		base + ".discover_models",
		base + ".flat_rate",
	}
	var token openaioauth.Token
	_ = json.Unmarshal(legacy.OAuth, &token)
	if token.AccessToken != "" && strings.TrimSpace(legacy.APIKey) == token.AccessToken {
		removals = append(removals, base+".api_key")
	}
	if strings.TrimSpace(legacy.BaseURL) == CodexBackendURL {
		removals = append(removals, base+".base_url")
	}
	for _, key := range removals {
		if err := api.RemoveConfigField(ctx, workspaceID, scope, key); err != nil {
			return fmt.Errorf("clear the legacy ChatGPT credential: %w", err)
		}
	}
	return nil
}

func SelectionStrandedOnLegacyOpenAI(cfg engineapi.WorkspaceConfig, savedProvider string) bool {
	switch strings.TrimSpace(savedProvider) {
	case "", OpenAIID:
	default:
		return false
	}
	if !OAuthCredentialPresent(cfg.Providers[CodexID]) {
		return false
	}
	legacy := cfg.Providers[OpenAIID]
	return strings.TrimSpace(legacy.APIKey) == "" && !OAuthCredentialPresent(legacy)
}

func codexCatalogEntry(ctx context.Context, api *engineapi.Client, workspaceID string) (engineapi.Provider, error) {
	providers, err := api.ListProviders(ctx, workspaceID)
	if err != nil {
		return engineapi.Provider{}, fmt.Errorf("read the provider catalog: %w", err)
	}
	entry := CodexSpec().Provider
	for _, candidate := range providers {
		if candidate.ID == CodexID {
			entry = candidate
			break
		}
	}
	cfg, err := api.GetWorkspaceConfig(ctx, workspaceID)
	if err != nil {
		return engineapi.Provider{}, fmt.Errorf("read the stored Codex catalog: %w", err)
	}
	stored := cfg.Providers[CodexID]
	if stored.BaseURL != "" {
		entry.APIEndpoint = stored.BaseURL
	}
	entry.Models = MergeModels(stored.Models, entry.Models)
	return entry, nil
}

func SelectChatGPTModel(providers []engineapi.Provider, current string) (string, error) {
	for _, candidate := range providers {
		if candidate.ID != CodexID {
			continue
		}
		for _, model := range candidate.Models {
			if current != "" && model.ID == current {
				return current, nil
			}
		}
		if candidate.DefaultLargeModelID != "" {
			return candidate.DefaultLargeModelID, nil
		}
		if len(candidate.Models) > 0 {
			return candidate.Models[0].ID, nil
		}
	}
	return "", fmt.Errorf("ChatGPT subscription returned no selectable models")
}

func SelectCodexModel(ctx context.Context, api *engineapi.Client, workspaceID, currentModel, thinking string) (string, error) {
	entry, err := codexCatalogEntry(ctx, api, workspaceID)
	if err != nil {
		return "", err
	}
	modelID, err := SelectChatGPTModel([]engineapi.Provider{entry}, strings.TrimSpace(currentModel))
	if err != nil {
		return "", err
	}
	effort, think := Reasoning(thinking)
	selected := engineapi.SelectedModel{Provider: CodexID, Model: modelID, ReasoningEffort: effort, Think: think}
	if err := api.SetPreferredModelPair(ctx, workspaceID, engineapi.ConfigScopeGlobal, selected); err != nil {
		return "", fmt.Errorf("repoint the saved model at the Codex provider: %w", err)
	}
	return modelID, nil
}

func ChatGPTRedirectCandidate(settings Settings, apiKey string) bool {
	if strings.TrimSpace(apiKey) != "" || settings.ProviderOnly {
		return false
	}
	if strings.TrimSpace(settings.Provider) != OpenAIID {
		return false
	}
	switch strings.TrimSpace(settings.CredentialProvider) {
	case "", OpenAIID:
		return true
	default:
		return false
	}
}

func RedirectStrandedChatGPTSelection(ctx context.Context, api *engineapi.Client, workspaceID string, settings Settings, apiKey string) (Settings, error) {
	if !ChatGPTRedirectCandidate(settings, apiKey) {
		return settings, nil
	}
	cfg, err := api.GetWorkspaceConfig(ctx, workspaceID)
	if err != nil {
		return settings, fmt.Errorf("read the provider credentials: %w", err)
	}
	if !SelectionStrandedOnLegacyOpenAI(cfg, settings.Provider) {
		return settings, nil
	}
	entry, err := codexCatalogEntry(ctx, api, workspaceID)
	if err != nil {
		return settings, err
	}
	modelID, err := SelectChatGPTModel([]engineapi.Provider{entry}, strings.TrimSpace(settings.Model))
	if err != nil {
		return settings, err
	}
	settings.Provider = CodexID
	settings.Model = modelID
	if strings.TrimSpace(settings.CredentialProvider) != "" {
		settings.CredentialProvider = CodexID
	}
	settings.CustomURL = ""
	return settings, nil
}

func ApplyChatGPTLoginSelection(ctx context.Context, api *engineapi.Client, workspaceID, currentModel, thinking string) (string, error) {
	providers, err := api.ListProviders(ctx, workspaceID)
	if err != nil {
		return "", fmt.Errorf("load ChatGPT subscription models: %w", err)
	}
	modelID, err := SelectChatGPTModel(providers, currentModel)
	if err != nil {
		return "", err
	}
	effort, think := Reasoning(thinking)
	selected := engineapi.SelectedModel{Provider: CodexID, Model: modelID, ReasoningEffort: effort, Think: think}
	if err := api.SetPreferredModelPair(ctx, workspaceID, engineapi.ConfigScopeGlobal, selected); err != nil {
		return "", fmt.Errorf("select ChatGPT model: %w", err)
	}
	return modelID, nil
}
