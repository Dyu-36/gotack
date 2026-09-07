package main

import (
	"context"

	"github.com/Dyu-36/gotack/internal/crushapi"
	providerdomain "github.com/Dyu-36/gotack/internal/provider"
)

const (
	mistralProviderID      = providerdomain.MistralID
	mistralDefaultEndpoint = providerdomain.MistralDefaultEndpoint
	openAICompatType       = providerdomain.OpenAICompatType
)

type localProviderSpec = providerdomain.LocalSpec

func mistralProviderSpec() localProviderSpec {
	return providerdomain.MistralSpec()
}

func mergeLocalProviderOverlays(providers []crushapi.Provider) ([]crushapi.Provider, map[string]bool) {
	return providerdomain.MergeLocalOverlays(providers)
}

func mergeProviderModels(primary, fallback []crushapi.Model) []crushapi.Model {
	return providerdomain.MergeModels(primary, fallback)
}

func prepareLocalProviderConfig(ctx context.Context, api *crushapi.Client, workspaceID string, scope int, providerID string) (bool, error) {
	return providerdomain.PrepareLocal(ctx, api, workspaceID, scope, providerID)
}

func localProviderIdentityMatches(configured crushapi.ProviderConfig, spec localProviderSpec) bool {
	return providerdomain.IdentityMatches(configured, spec)
}

func finalizeLocalProviderConfig(ctx context.Context, api *crushapi.Client, workspaceID string, scope int, providerID string) error {
	return providerdomain.FinalizeLocal(ctx, api, workspaceID, scope, providerID)
}
