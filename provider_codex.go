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

// provider_codex.go -- role: keep ChatGPT subscription auth (provider "codex")
// separate from the public OpenAI API provider ("openai").
//
// A provider id holds exactly one credential in Crush, and storing a ChatGPT
// OAuth token rewrites that provider's endpoint to the Codex backend and
// replaces its catalog with subscription-only models. Sharing one id therefore
// made an API key and a ChatGPT login mutually destructive. Split apart,
// "openai" always means an API key against the public API and "codex" always
// means the browser login against the subscription backend.

const (
	openAIProviderID  = "openai"
	codexProviderID   = "codex"
	codexProviderName = "ChatGPT (Codex)"
	// codexBackendURL mirrors the engine's Codex endpoint. Gotack must not
	// import Crush internals (AGENTS.md rule 3), so the value is duplicated
	// here on purpose.
	codexBackendURL = "https://chatgpt.com/backend-api/codex"
	// codexProviderType selects Crush's OpenAI client, which speaks the
	// protocol the Codex backend expects.
	codexProviderType = "openai"
)

// codexProviderSpec describes Codex for the Settings catalog. It carries no
// API key template and no models on purpose: the subscription catalog is
// account-scoped and the engine writes it when the OAuth token is stored.
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

// oauthCredentialPresent reports whether a provider config carries a real
// OAuth credential rather than an empty placeholder. Crush serializes a
// cleared credential as null or {}, both of which mean "not signed in".
func oauthCredentialPresent(pc crushapi.ProviderConfig) bool {
	raw := strings.TrimSpace(string(pc.OAuth))
	return raw != "" && raw != "null" && raw != "{}"
}

// seedCodexProvider writes the identity fields Crush needs before a credential
// can be stored: the engine rejects a provider that is neither in the Catwalk
// catalog nor already present in config. Endpoint, model catalog and flat-rate
// flags are written by the engine together with the token, so this
// deliberately does not touch them.
func seedCodexProvider(ctx context.Context, api *crushapi.Client, wsID string, scope int) error {
	spec := codexProviderSpec()
	base := "providers." + codexProviderID
	writes := []providerConfigWrite{
		{base + ".name", spec.Provider.Name},
		{base + ".type", spec.Provider.Type},
		{base + ".base_url", spec.Provider.APIEndpoint},
		{base + ".discover_models", false},
		{base + ".disable", false},
	}
	for _, write := range writes {
		if err := api.SetConfigField(ctx, wsID, scope, write.key, write.value); err != nil {
			return fmt.Errorf("seed Codex provider: %w", err)
		}
	}
	return nil
}

// migrateChatGPTOAuthToCodex moves a ChatGPT credential written by pre-split
// builds from "openai" to "codex". Without it an upgrade would leave the login
// on a provider that now means "OpenAI API key", so subscription traffic would
// keep the public API entry rewritten and the user would be asked to sign in
// again for no reason.
//
// It reports true when a credential was moved or a stale copy was cleaned up.
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
		// Codex already holds a login, so the copy left on "openai" is stale and
		// only keeps the old routing alive.
		return true, clearLegacyChatGPTCredential(ctx, api, wsID, scope, legacy)
	}
	var token openaioauth.Token
	if err := json.Unmarshal(legacy.OAuth, &token); err != nil || token.AccessToken == "" {
		// Nothing usable to move. Leave it in place so the user can see and
		// repair the credential instead of losing it silently.
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

// clearLegacyChatGPTCredential removes everything the OAuth path wrote under
// "openai" so that provider means exactly one thing again: an API key against
// the public OpenAI API. An API key the user typed stays; the access-token copy
// the engine mirrored into api_key does not, because it only works against the
// Codex backend that "openai" no longer points at.
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

// providerUsesOAuth reports whether a provider is authenticated by a browser
// sign-in rather than a key the user typed. Codex always is; any other provider
// counts only while a credential is actually stored for it.
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

// migrateChatGPTProviderCredential moves a pre-split ChatGPT login onto Codex
// at startup so an upgrade never asks the user to sign in again, and repoints
// the saved model selection at the provider that now owns it.
//
// Everything here is best effort: a failure must not stop the app from
// starting, and the next status check retries.
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

// repairStrandedChatGPTSelection converges an install whose credential already
// reached Codex while the saved selection stayed on "openai" -- the state a
// move without a repoint leaves behind. Nothing else recovers it: the
// credential is gone from "openai", so the migration reports nothing to move,
// while every settings replay keeps pointing the engine at a provider that
// holds no credential.
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

// selectionStrandedOnLegacyOpenAI reports whether the saved selection names a
// provider that cannot serve it: the ChatGPT credential lives on Codex while
// the selection still points at "openai", which holds no credential at all. An
// API key on "openai" makes that selection serviceable and deliberate, so it is
// left alone, and so is a selection that already names another provider.
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

// codexCatalogEntry assembles the Codex catalog entry the same way the Settings
// catalog does. Two details make the plain provider list useless on its own:
// Crush/Catwalk never advertises Codex, so the entry comes from the local
// overlay, and the subscription models live in provider config because the
// engine writes them together with the OAuth token. Reading only the provider
// list yields an entry with no models, which is indistinguishable from being
// signed out.
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

// repointSavedModelAtCodex points the saved model selection at Codex after the
// credential moved there. It has to run in the same pass as the move: the saved
// selection still names "openai", which no longer holds a credential, and every
// later settings replay rewrites the engine's model preference from it -- so a
// skipped repoint silently restores the broken routing.
//
// The selection is only taken over when it was the ChatGPT one before the split
// (empty or "openai") or already Codex with a possibly stale model id. A
// provider the user picked deliberately is left alone.
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
	// The saved model id comes from the pre-split provider, so it is checked
	// against the account catalog instead of being trusted.
	modelID, err := selectChatGPTModel([]crushapi.Provider{entry}, strings.TrimSpace(a.cfg.Model))
	if err != nil {
		a.warnCodexMigration("could not pick a Codex model", "err", err)
		return
	}
	effort, think := crushReasoning(a.cfg.Thinking)
	selected := crushapi.SelectedModel{Provider: codexProviderID, Model: modelID, ReasoningEffort: effort, Think: think}
	for _, modelType := range []string{"large", "small"} {
		if err := svc.api.SetPreferredModel(a.ctx, workspaceID, crushapi.ConfigScopeGlobal, modelType, selected); err != nil {
			a.warnCodexMigration("could not repoint the saved model at the Codex provider", "model", modelType, "err", err)
			return
		}
	}
	a.cfg.Provider = codexProviderID
	a.cfg.Model = modelID
	// Codex only answers on the endpoint the engine picked for the credential, so
	// an endpoint carried over from the API-key provider would be rejected on the
	// next settings replay.
	a.cfg.CustomURL = ""
	cfgCopy := *a.cfg
	if err := appconfig.Save(&cfgCopy); err != nil {
		a.warnCodexMigration("could not save the Codex provider selection", "err", err)
	}
}

// warnCodexMigration logs a best-effort migration problem. The logger is nil
// before startup wires it, and a migration must never panic the caller.
func (a *App) warnCodexMigration(msg string, args ...any) {
	if a.log == nil {
		return
	}
	a.log.Warn(msg, args...)
}

// redirectStrandedChatGPTSelection rewrites a settings payload that still names
// the pre-split "openai" provider once the ChatGPT credential lives on Codex.
// The UI replays the selection it loaded at boot, so a repair made during
// connect is otherwise overwritten seconds later by that replay, and the engine
// is pointed back at a provider that holds no credential.
//
// Deliberate API-key setups are left alone: a payload carrying a key, a
// provider-only write, and an "openai" provider that already holds a key.
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
	// The saved model id predates the split, so it is checked against the
	// account catalog instead of being carried over blindly.
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
	// Codex only answers on the endpoint the engine picked for the credential, so
	// an endpoint inherited from the API-key provider must not travel with the
	// redirected selection.
	s.CustomURL = ""
	return s, nil
}

// chatGPTRedirectCandidate reports whether a settings payload may be repointed
// at Codex at all. It keeps every deliberate API-key action out of the
// credential lookup that follows, so configuring "openai" with a key never
// turns into a Codex selection.
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
