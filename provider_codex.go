package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/crushapi"
	"github.com/Dyu-36/gotack/internal/openaioauth"
)

const (
	openAIProviderID  = "openai"
	codexProviderID   = "codex"
	codexProviderName = "ChatGPT (Codex)"

	codexBackendURL = "https://chatgpt.com/backend-api/codex"

	codexProviderType = "openai"
)

func codexProviderSpec() localProviderSpec {
	return localProviderSpec{
		Provider: crushapi.Provider{
			ID:          codexProviderID,
			Name:        codexProviderName,
			Type:        codexProviderType,
			APIEndpoint: codexBackendURL,
		},
		OAuthOnly: true,
	}
}

func oauthCredentialPresent(pc crushapi.ProviderConfig) bool {
	raw := strings.TrimSpace(string(pc.OAuth))
	return raw != "" && raw != "null" && raw != "{}"
}

func seedCodexProvider(ctx context.Context, api *crushapi.Client, wsID string, scope int) error {
	spec := codexProviderSpec()
	base := "providers." + codexProviderID
	fields := localProviderConfigFields(spec)
	fields[base+".disable"] = false
	if err := api.SetConfigFields(ctx, wsID, scope, fields); err != nil {
		return fmt.Errorf("seed Codex provider: %w", err)
	}
	return nil
}

func migrateChatGPTOAuthToCodex(ctx context.Context, api *crushapi.Client, wsID string) (bool, error) {
	scope := crushapi.ConfigScopeGlobal
	cfg, err := api.GetWorkspaceConfig(ctx, wsID)
	if err != nil {
		return false, fmt.Errorf("read provider config before Codex migration: %w", err)
	}
	legacy, exists := cfg.Providers[openAIProviderID]
	if !exists || !oauthCredentialPresent(legacy) {
		return false, nil
	}
	if codex, ok := cfg.Providers[codexProviderID]; ok && oauthCredentialPresent(codex) {

		return true, clearLegacyChatGPTCredential(ctx, api, wsID, scope, legacy)
	}
	var token openaioauth.Token
	if err := json.Unmarshal(legacy.OAuth, &token); err != nil || token.AccessToken == "" {

		return false, nil
	}
	if err := seedCodexProvider(ctx, api, wsID, scope); err != nil {
		return false, err
	}
	if err := api.SetProviderOAuthToken(ctx, wsID, scope, codexProviderID, &token); err != nil {
		return false, fmt.Errorf("move the ChatGPT credential to Codex: %w", err)
	}
	return true, clearLegacyChatGPTCredential(ctx, api, wsID, scope, legacy)
}

func clearLegacyChatGPTCredential(ctx context.Context, api *crushapi.Client, wsID string, scope int, legacy crushapi.ProviderConfig) error {
	base := "providers." + openAIProviderID
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
	if strings.TrimSpace(legacy.BaseURL) == codexBackendURL {
		removals = append(removals, base+".base_url")
	}
	for _, key := range removals {
		if err := api.RemoveConfigField(ctx, wsID, scope, key); err != nil {
			return fmt.Errorf("clear the legacy ChatGPT credential: %w", err)
		}
	}
	return nil
}

func providerUsesOAuth(ctx context.Context, api *crushapi.Client, wsID, providerID string) (bool, error) {
	if providerID == codexProviderID {
		return true, nil
	}
	cfg, err := api.GetWorkspaceConfig(ctx, wsID)
	if err != nil {
		return false, fmt.Errorf("read the provider credential kind: %w", err)
	}
	pc, ok := cfg.Providers[providerID]
	return ok && oauthCredentialPresent(pc), nil
}

func (a *App) migrateChatGPTProviderCredential(svc *bridgeServices) {
	workspaceID, err := a.configWorkspaceID(a.ctx, svc)
	if err != nil {
		a.log.Warn("could not resolve a workspace for the Codex credential migration", "err", err)
		return
	}
	moved, err := migrateChatGPTOAuthToCodex(a.ctx, svc.api, workspaceID)
	if err != nil {
		a.log.Warn("could not move the ChatGPT credential to the Codex provider", "err", err)
		return
	}
	if !moved {
		a.repairStrandedChatGPTSelection(svc, workspaceID)
		return
	}
	if a.log != nil {
		a.log.Info("moved the ChatGPT credential to the Codex provider")
	}
	a.repointSavedModelAtCodex(svc, workspaceID)
}

func (a *App) repairStrandedChatGPTSelection(svc *bridgeServices, workspaceID string) {
	if a.cfg == nil {
		return
	}
	cfg, err := svc.api.GetWorkspaceConfig(a.ctx, workspaceID)
	if err != nil {
		a.warnCodexMigration("could not check the saved provider selection", "err", err)
		return
	}
	if !selectionStrandedOnLegacyOpenAI(cfg, a.cfg.Provider) {
		return
	}
	if a.log != nil {
		a.log.Info("repointing the saved model at the Codex provider that owns the credential")
	}
	a.repointSavedModelAtCodex(svc, workspaceID)
}

func selectionStrandedOnLegacyOpenAI(cfg crushapi.WorkspaceConfig, savedProvider string) bool {
	switch strings.TrimSpace(savedProvider) {
	case "", openAIProviderID:
	default:
		return false
	}
	if !oauthCredentialPresent(cfg.Providers[codexProviderID]) {
		return false
	}
	legacy := cfg.Providers[openAIProviderID]
	return strings.TrimSpace(legacy.APIKey) == "" && !oauthCredentialPresent(legacy)
}

func codexCatalogEntry(ctx context.Context, api *crushapi.Client, wsID string) (crushapi.Provider, error) {
	providers, err := api.ListProviders(ctx, wsID)
	if err != nil {
		return crushapi.Provider{}, fmt.Errorf("read the provider catalog: %w", err)
	}
	entry := codexProviderSpec().Provider
	for _, provider := range providers {
		if provider.ID == codexProviderID {
			entry = provider
			break
		}
	}
	cfg, err := api.GetWorkspaceConfig(ctx, wsID)
	if err != nil {
		return crushapi.Provider{}, fmt.Errorf("read the stored Codex catalog: %w", err)
	}
	stored := cfg.Providers[codexProviderID]
	if stored.BaseURL != "" {
		entry.APIEndpoint = stored.BaseURL
	}
	entry.Models = mergeProviderModels(stored.Models, entry.Models)
	return entry, nil
}

func (a *App) repointSavedModelAtCodex(svc *bridgeServices, workspaceID string) {
	if a.cfg == nil {
		return
	}
	switch strings.TrimSpace(a.cfg.Provider) {
	case "", openAIProviderID, codexProviderID:
	default:
		return
	}
	entry, err := codexCatalogEntry(a.ctx, svc.api, workspaceID)
	if err != nil {
		a.warnCodexMigration("could not load the Codex model catalog", "err", err)
		return
	}

	modelID, err := selectChatGPTModel([]crushapi.Provider{entry}, strings.TrimSpace(a.cfg.Model))
	if err != nil {
		a.warnCodexMigration("could not pick a Codex model", "err", err)
		return
	}
	effort, think := crushReasoning(a.cfg.Thinking)
	selected := crushapi.SelectedModel{Provider: codexProviderID, Model: modelID, ReasoningEffort: effort, Think: think}
	if err := svc.api.SetPreferredModelPair(a.ctx, workspaceID, crushapi.ConfigScopeGlobal, selected); err != nil {
		a.warnCodexMigration("could not repoint the saved model at the Codex provider", "err", err)
		return
	}
	next := *a.cfg
	next.Provider = codexProviderID
	next.Model = modelID
	next.CustomURL = ""
	if err := appconfig.Save(&next); err != nil {
		a.warnCodexMigration("could not save the Codex provider selection", "err", err)
		return
	}
	a.cfg = &next
}

func (a *App) warnCodexMigration(msg string, args ...any) {
	if a.log == nil {
		return
	}
	a.log.Warn(msg, args...)
}

func (a *App) redirectStrandedChatGPTSelection(svc *bridgeServices, workspaceID string, s SettingsInfo, apiKey string) (SettingsInfo, error) {
	if !chatGPTRedirectCandidate(s, apiKey) {
		return s, nil
	}
	cfg, err := svc.api.GetWorkspaceConfig(a.ctx, workspaceID)
	if err != nil {
		return s, fmt.Errorf("read the provider credentials: %w", err)
	}
	if !selectionStrandedOnLegacyOpenAI(cfg, s.Provider) {
		return s, nil
	}
	entry, err := codexCatalogEntry(a.ctx, svc.api, workspaceID)
	if err != nil {
		return s, err
	}

	modelID, err := selectChatGPTModel([]crushapi.Provider{entry}, strings.TrimSpace(s.Model))
	if err != nil {
		return s, err
	}
	if a.log != nil {
		a.log.Info("redirected a stale ChatGPT selection at the Codex provider", "model", modelID)
	}
	s.Provider = codexProviderID
	s.Model = modelID
	if strings.TrimSpace(s.CredentialProvider) != "" {
		s.CredentialProvider = codexProviderID
	}

	s.CustomURL = ""
	return s, nil
}

func chatGPTRedirectCandidate(s SettingsInfo, apiKey string) bool {
	if strings.TrimSpace(apiKey) != "" || s.ProviderOnly {
		return false
	}
	if strings.TrimSpace(s.Provider) != openAIProviderID {
		return false
	}
	switch strings.TrimSpace(s.CredentialProvider) {
	case "", openAIProviderID:
		return true
	default:
		return false
	}
}
