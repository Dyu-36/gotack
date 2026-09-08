package main

import (
	"github.com/Dyu-36/gotack/internal/engineapi"
	providerdomain "github.com/Dyu-36/gotack/internal/provider"
)

func resolvedProviderCredential(config engineapi.ProviderConfig) (kind, value string, ok bool) {
	return providerdomain.ResolvedCredential(config)
}

func providerReasoning(value string) (effort string, think bool) {
	return providerdomain.Reasoning(value)
}
