package config

import (
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/stretchr/testify/require"
)

func TestProviderConfigForModelDoesNotMutateSharedRoutes(t *testing.T) {
	t.Parallel()

	routeHeaders := map[string]string{"X-Route": "route", "X-Shared": "route"}
	userHeaders := map[string]string{"X-User": "user", "X-Shared": "user"}
	routes := map[string]ModelRoute{
		"future-model": {
			API:     "openai-responses",
			BaseURL: "https://route.example/v1",
			Headers: routeHeaders,
		},
	}
	provider := ProviderConfig{
		ID:           "custom",
		Type:         catwalk.TypeOpenAICompat,
		BaseURL:      "https://provider.example/v1",
		ExtraHeaders: userHeaders,
		ModelRoutes:  routes,
	}

	got, err := provider.ForModel("future-model")
	require.NoError(t, err)
	require.Equal(t, catwalk.TypeOpenAI, got.Type)
	require.Equal(t, "https://route.example/v1", got.BaseURL)
	require.Equal(t, map[string]string{
		"X-Route":  "route",
		"X-Shared": "user",
		"X-User":   "user",
	}, got.ExtraHeaders)

	got.ExtraHeaders["X-Route"] = "changed"
	require.Equal(t, "route", routeHeaders["X-Route"])
	require.Equal(t, "user", userHeaders["X-Shared"])
	require.Equal(t, "openai-responses", routes["future-model"].API)
	require.Equal(t, "route", routes["future-model"].Headers["X-Route"])
	require.Equal(t, catwalk.TypeOpenAICompat, provider.Type)
	require.Equal(t, "https://provider.example/v1", provider.BaseURL)
}

func TestProviderConfigForModelSelectsMixedProtocols(t *testing.T) {
	t.Parallel()

	provider := ProviderConfig{
		ID:   "custom",
		Type: catwalk.TypeOpenAICompat,
		ModelRoutes: map[string]ModelRoute{
			"future-response": {
				API:     "openai-responses",
				BaseURL: "https://responses.example/v1",
			},
			"legacy-chat": {
				API:     "openai-completions",
				BaseURL: "https://chat.example/v1",
			},
			"claude-alias": {
				API:     "anthropic-messages",
				BaseURL: "https://anthropic.example",
			},
		},
	}

	response, err := provider.ForModel("future-response")
	require.NoError(t, err)
	require.Equal(t, catwalk.TypeOpenAI, response.Type)
	require.Equal(t, "https://responses.example/v1", response.BaseURL)
	require.True(t, response.UsesResponsesAPI("future-response", false))

	chat, err := provider.ForModel("legacy-chat")
	require.NoError(t, err)
	require.Equal(t, catwalk.TypeOpenAICompat, chat.Type)
	require.Equal(t, "https://chat.example/v1", chat.BaseURL)
	require.False(t, chat.UsesResponsesAPI("legacy-chat", true), "explicit chat route must override the model-name heuristic")

	ant, err := provider.ForModel("claude-alias")
	require.NoError(t, err)
	require.Equal(t, catwalk.TypeAnthropic, ant.Type)
	require.Equal(t, "https://anthropic.example", ant.BaseURL)

	unrouted, err := provider.ForModel("other-model")
	require.NoError(t, err)
	require.Equal(t, catwalk.TypeOpenAICompat, unrouted.Type)
	require.Equal(t, provider.BaseURL, unrouted.BaseURL)
}

func TestProviderConfigForModelSupportsKnownProtocolNames(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		api    string
		typeID catwalk.Type
	}{
		{"openai-responses", catwalk.TypeOpenAI},
		{"openai-completions", catwalk.TypeOpenAICompat},
		{"anthropic-messages", catwalk.TypeAnthropic},
		{"google-generative-ai", catwalk.TypeGoogle},
		{"google-vertex", catwalk.TypeVertexAI},
		{"bedrock-converse-stream", catwalk.TypeBedrock},
		{"azure-openai-responses", catwalk.TypeAzure},
	} {
		t.Run(tc.api, func(t *testing.T) {
			t.Parallel()
			provider := ProviderConfig{
				ID:   "custom",
				Type: catwalk.TypeOpenAICompat,
				ModelRoutes: map[string]ModelRoute{
					"model": {API: tc.api},
				},
			}

			got, err := provider.ForModel("model")
			require.NoError(t, err)
			require.Equal(t, tc.typeID, got.Type)
		})
	}
}

func TestProviderConfigForModelRejectsUnknownProtocol(t *testing.T) {
	t.Parallel()

	provider := ProviderConfig{
		ID:   "custom",
		Type: catwalk.TypeOpenAICompat,
		ModelRoutes: map[string]ModelRoute{
			"model": {API: "future-api"},
		},
	}

	got, err := provider.ForModel("model")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported catalog api")
	require.Equal(t, catwalk.TypeOpenAICompat, got.Type, "error path must not mutate the input config")
}

func TestProviderConfigForModelLeavesSubscriptionRoutingAuthoritative(t *testing.T) {
	t.Parallel()

	token := &oauth.Token{AccessToken: "subscription-access-token"}
	provider := ProviderConfig{
		ID:           "codex",
		Type:         catwalk.TypeOpenAI,
		BaseURL:      "https://chatgpt.com/backend-api/codex",
		OAuthToken:   token,
		ExtraHeaders: map[string]string{"X-User": "kept"},
		ModelRoutes: map[string]ModelRoute{
			"future-model": {
				API:     "unsupported-route-for-subscription",
				BaseURL: "https://catalog.example",
				Headers: map[string]string{"X-Route": "ignored"},
			},
		},
	}

	got, err := provider.ForModel("future-model")
	require.NoError(t, err)
	require.Equal(t, catwalk.TypeOpenAI, got.Type)
	require.Equal(t, "https://chatgpt.com/backend-api/codex", got.BaseURL)
	require.Equal(t, map[string]string{"X-User": "kept"}, got.ExtraHeaders)
	require.False(t, got.UsesResponsesAPI("future-model", false))
	require.True(t, got.UsesResponsesAPI("gpt-5.6-terra", true), "legacy subscription model-name behavior must remain active")
}
