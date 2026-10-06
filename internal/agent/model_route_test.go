package agent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"charm.land/fantasy/providers/openai"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

func TestBuildProviderUsesPerModelProtocolAndEndpoint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		modelID      string
		api          string
		wantPath     string
		responseBody string
		checkOptions bool
	}{
		{
			name:     "future model uses responses endpoint and encrypted reasoning include",
			modelID:  "future-model-v99",
			api:      "openai-responses",
			wantPath: "/responses",
			responseBody: `{"id":"resp_test","object":"response","created_at":0,"status":"completed",` +
				`"model":"future-model-v99","output":[],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`,
			checkOptions: true,
		},
		{
			name:     "explicit completions route overrides old model-name heuristic",
			modelID:  "gpt-5.6-terra",
			api:      "openai-completions",
			wantPath: "/chat/completions",
			responseBody: `{"id":"chatcmpl_test","object":"chat.completion","created":0,` +
				`"model":"gpt-5.6-terra","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},` +
				`"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`,
		},
		{
			name:     "anthropic route keeps its custom endpoint",
			modelID:  "claude-deployment-alias",
			api:      "anthropic-messages",
			wantPath: "/v1/messages",
			responseBody: `{"id":"msg_test","type":"message","role":"assistant","content":[{"type":"text","text":"ok"}],` +
				`"model":"claude-deployment-alias","stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":1,"output_tokens":1}}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var requestBody map[string]any
			var requestPath string
			var catalogHeader, userHeader string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requestPath = r.URL.Path
				catalogHeader = r.Header.Get("X-Catalog-Route")
				userHeader = r.Header.Get("X-User-Route")
				if r.Body != nil {
					_ = json.NewDecoder(r.Body).Decode(&requestBody)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.responseBody))
			}))
			defer server.Close()

			cfg, err := config.Init(t.TempDir(), t.TempDir(), false)
			require.NoError(t, err)
			coord := &coordinator{cfg: cfg}
			providerCfg := config.ProviderConfig{
				ID:           "custom-provider",
				Type:         catwalk.TypeOpenAICompat,
				BaseURL:      "https://ignored.example/v1",
				APIKey:       "test-token",
				ExtraHeaders: map[string]string{"X-User-Route": "user"},
				ModelRoutes: map[string]config.ModelRoute{
					tc.modelID: {
						API:     tc.api,
						BaseURL: server.URL,
						Headers: map[string]string{"X-Catalog-Route": "catalog", "X-User-Route": "catalog"},
					},
				},
			}
			selected := config.SelectedModel{Provider: providerCfg.ID, Model: tc.modelID}
			provider, err := coord.buildProvider(providerCfg, selected)
			require.NoError(t, err)
			languageModel, err := provider.LanguageModel(t.Context(), tc.modelID)
			require.NoError(t, err)

			var providerOptions fantasy.ProviderOptions
			if tc.checkOptions {
				model := Model{
					CatwalkCfg: catwalk.Model{
						ID:              tc.modelID,
						CanReason:       true,
						ReasoningLevels: []string{"high"},
					},
					ModelCfg: config.SelectedModel{
						Provider:        providerCfg.ID,
						Model:           tc.modelID,
						ReasoningEffort: "high",
					},
				}
				providerOptions, err = getProviderOptions(model, providerCfg)
				require.NoError(t, err)
				options, ok := providerOptions[openai.Name].(*openai.ResponsesProviderOptions)
				require.True(t, ok, "responses route should configure responses provider options")
				require.Contains(t, options.Include, openai.IncludeReasoningEncryptedContent)
			}

			_, err = languageModel.Generate(t.Context(), fantasy.Call{
				Prompt:          fantasy.Prompt{fantasy.NewUserMessage("hello")},
				MaxOutputTokens: int64Ptr(16),
				ProviderOptions: providerOptions,
			})
			require.NoError(t, err)
			require.Equal(t, tc.wantPath, requestPath)
			require.Equal(t, "catalog", catalogHeader)
			require.Equal(t, "user", userHeader, "user-configured header must override the catalog default")
			if tc.checkOptions {
				includes, ok := requestBody["include"].([]any)
				require.True(t, ok, "Responses request should carry include: %#v", requestBody)
				require.Contains(t, includes, string(openai.IncludeReasoningEncryptedContent))
			}
		})
	}
}

func int64Ptr(value int64) *int64 { return &value }

func TestBuildProviderPreservesMixedRouteIsolation(t *testing.T) {
	t.Parallel()

	providerCfg := config.ProviderConfig{
		ID:      "custom-provider",
		Type:    catwalk.TypeOpenAICompat,
		BaseURL: "https://provider.example/v1",
		ModelRoutes: map[string]config.ModelRoute{
			"response-model": {
				API:     "openai-responses",
				BaseURL: "https://responses.example/v1",
			},
			"anthropic-model": {
				API:     "anthropic-messages",
				BaseURL: "https://anthropic.example",
			},
		},
	}

	cfg, err := config.Init(t.TempDir(), t.TempDir(), false)
	require.NoError(t, err)
	coord := &coordinator{cfg: cfg}

	responseProvider, err := coord.buildProvider(providerCfg, config.SelectedModel{Provider: providerCfg.ID, Model: "response-model"})
	require.NoError(t, err)
	anthropicProvider, err := coord.buildProvider(providerCfg, config.SelectedModel{Provider: providerCfg.ID, Model: "anthropic-model"})
	require.NoError(t, err)

	responseModel, err := responseProvider.LanguageModel(t.Context(), "response-model")
	require.NoError(t, err)
	anthropicModel, err := anthropicProvider.LanguageModel(t.Context(), "anthropic-model")
	require.NoError(t, err)
	require.NotNil(t, responseModel)
	require.NotNil(t, anthropicModel)

	responseCfg, err := providerCfg.ForModel("response-model")
	require.NoError(t, err)
	anthropicCfg, err := providerCfg.ForModel("anthropic-model")
	require.NoError(t, err)
	require.NotEqual(t, responseCfg.Type, anthropicCfg.Type)
	require.Equal(t, "https://responses.example/v1", responseCfg.BaseURL)
	require.Equal(t, "https://anthropic.example", anthropicCfg.BaseURL)
	require.Equal(t, catwalk.TypeOpenAICompat, providerCfg.Type, "building routes must not mutate shared provider config")
	require.Equal(t, "https://provider.example/v1", providerCfg.BaseURL)
}

func TestBuildCopilotAnthropicRouteUsesBearerAndInitiatorTransport(t *testing.T) {
	var authorization string
	var apiKeyHeader string
	var initiator string
	var requestPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization = r.Header.Get("Authorization")
		apiKeyHeader = r.Header.Get("X-Api-Key")
		initiator = r.Header.Get("X-Initiator")
		requestPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"msg_test","type":"message","role":"assistant",` +
			`"content":[{"type":"text","text":"ok"}],"model":"copilot-claude-alias",` +
			`"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":1,"output_tokens":1}}`))
	}))
	defer server.Close()

	cfg, err := config.Init(t.TempDir(), t.TempDir(), false)
	require.NoError(t, err)
	coord := &coordinator{cfg: cfg}
	providerCfg := config.ProviderConfig{
		ID:      string(catwalk.InferenceProviderCopilot),
		Type:    catwalk.TypeOpenAICompat,
		APIKey:  "test-token",
		BaseURL: "https://ignored.example/v1",
		ModelRoutes: map[string]config.ModelRoute{
			"copilot-claude-alias": {
				API:     "anthropic-messages",
				BaseURL: server.URL,
			},
		},
	}
	model := config.SelectedModel{Provider: providerCfg.ID, Model: "copilot-claude-alias"}
	provider, err := coord.buildProvider(providerCfg, model)
	require.NoError(t, err)
	languageModel, err := provider.LanguageModel(t.Context(), model.Model)
	require.NoError(t, err)
	_, err = languageModel.Generate(t.Context(), fantasy.Call{
		Prompt:          fantasy.Prompt{fantasy.NewUserMessage("hello")},
		MaxOutputTokens: int64Ptr(16),
	})
	require.NoError(t, err)

	require.Equal(t, "/v1/messages", requestPath)
	require.Equal(t, "Bearer test-token", authorization)
	require.Empty(t, apiKeyHeader, "Copilot auth must not be sent as X-Api-Key")
	require.Equal(t, "user", initiator, "Copilot transport should mark a user-only request")
}
