package main

import (
	"github.com/Dyu-36/gotack/internal/engineapi"
	providerdomain "github.com/Dyu-36/gotack/internal/provider"
)

func resolvedProviderCredential(config engineapi.ProviderConfig) (kind, value string, ok bool) {
	return providerdomain.ResolvedCredential(config)
}

func preferredModelsUseProvider(models map[string]engineapi.SelectedModel, providerID string) bool {
	return providerdomain.PreferredModelsUseProvider(models, providerID)
}

func providerReasoning(value string) (effort string, think bool) {
	return providerdomain.Reasoning(value)
}
