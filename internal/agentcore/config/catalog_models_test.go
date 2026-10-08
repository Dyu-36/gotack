//go:build gotacktest

package config

import (
	"context"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/stretchr/testify/require"
)

func TestConfigureProvidersCatalogModelsDoesNotRestoreBundledDefaults(t *testing.T) {
	knownProviders := []catwalk.Provider{{
		ID:          catwalk.InferenceProviderOpenAI,
		Name:        "OpenAI",
		Type:        catwalk.TypeOpenAI,
		APIKey:      "$OPENAI_API_KEY",
		APIEndpoint: "https://api.openai.com/v1",
		Models: []catwalk.Model{{
			ID:   "bundled-default-model",
			Name: "Bundled default",
		}},
	}}

	cfg := &Config{}
	cfg.setDefaults(t.TempDir(), "")
	cfg.Providers.Set(string(catwalk.InferenceProviderOpenAI), ProviderConfig{
		CatalogModels: true,
		Models: []catwalk.Model{{
			ID:   "catalog-model",
			Name: "Catalog model",
		}},
	})
	env := testEnv(map[string]string{"OPENAI_API_KEY": "test-key"})
	resolver := NewShellVariableResolver(env)

	err := cfg.configureProviders(context.Background(), testStore(cfg), env, resolver, knownProviders)
	require.NoError(t, err)

	provider, ok := cfg.Providers.Get(string(catwalk.InferenceProviderOpenAI))
	require.True(t, ok)
	require.Len(t, provider.Models, 1)
	require.Equal(t, "catalog-model", provider.Models[0].ID)
}
