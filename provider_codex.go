package main

import (
	"strings"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/crushapi"
	providerdomain "github.com/Dyu-36/gotack/internal/provider"
)

const (
	openAIProviderID  = providerdomain.OpenAIID
	codexProviderID   = providerdomain.CodexID
	codexProviderName = providerdomain.CodexName
	codexBackendURL   = providerdomain.CodexBackendURL
	codexProviderType = providerdomain.CodexType
)

func selectionStrandedOnLegacyOpenAI(cfg crushapi.WorkspaceConfig, savedProvider string) bool {
	return providerdomain.SelectionStrandedOnLegacyOpenAI(cfg, savedProvider)
}

func chatGPTRedirectCandidate(settings SettingsInfo, apiKey string) bool {
	return providerdomain.ChatGPTRedirectCandidate(providerSettingsFromInfo(settings), apiKey)
}

func selectChatGPTModel(providers []crushapi.Provider, current string) (string, error) {
	return providerdomain.SelectChatGPTModel(providers, current)
}

func (a *App) migrateChatGPTProviderCredential(svc *bridgeServices) {
	workspaceID, err := a.configWorkspaceID(a.ctx, svc)
	if err != nil {
		a.log.Warn("could not resolve a workspace for the Codex credential migration", "err", err)
		return
	}
	moved, err := providerdomain.MigrateChatGPTOAuthToCodex(a.ctx, svc.api, workspaceID)
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
	if !providerdomain.SelectionStrandedOnLegacyOpenAI(cfg, a.cfg.Provider) {
		return
	}
	if a.log != nil {
		a.log.Info("repointing the saved model at the Codex provider that owns the credential")
	}
	a.repointSavedModelAtCodex(svc, workspaceID)
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

	modelID, err := providerdomain.SelectCodexModel(a.ctx, svc.api, workspaceID, a.cfg.Model, a.cfg.Thinking)
	if err != nil {
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

func (a *App) warnCodexMigration(message string, args ...any) {
	if a.log != nil {
		a.log.Warn(message, args...)
	}
}

func (a *App) redirectStrandedChatGPTSelection(svc *bridgeServices, workspaceID string, settings SettingsInfo, apiKey string) (SettingsInfo, error) {
	redirected, err := providerdomain.RedirectStrandedChatGPTSelection(a.ctx, svc.api, workspaceID, providerSettingsFromInfo(settings), apiKey)
	if err != nil {
		return settings, err
	}
	if settings.Provider != redirected.Provider && a.log != nil {
		a.log.Info("redirected a stale ChatGPT selection at the Codex provider", "model", redirected.Model)
	}
	return settingsInfoFromProvider(redirected), nil
}
