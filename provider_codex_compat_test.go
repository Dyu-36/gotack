package main

import (
	"github.com/Dyu-36/gotack/internal/engineapi"
	providerdomain "github.com/Dyu-36/gotack/internal/provider"
)

func selectionStrandedOnLegacyOpenAI(cfg engineapi.WorkspaceConfig, savedProvider string) bool {
	return providerdomain.SelectionStrandedOnLegacyOpenAI(cfg, savedProvider)
}

func chatGPTRedirectCandidate(settings SettingsInfo, apiKey string) bool {
	return providerdomain.ChatGPTRedirectCandidate(providerSettingsFromInfo(settings), apiKey)
}

func selectChatGPTModel(providers []engineapi.Provider, current string) (string, error) {
	return providerdomain.SelectChatGPTModel(providers, current)
}
