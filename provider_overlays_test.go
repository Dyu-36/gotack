package main

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Dyu-36/gotack/internal/crushapi"
)

func findProvider(t *testing.T, providers []crushapi.Provider, id string) crushapi.Provider {
	t.Helper()
	for _, provider := range providers {
		if provider.ID == id {
			return provider
		}
	}
	t.Fatalf("provider %q is missing from %#v", id, providers)
	return crushapi.Provider{}
}

func TestMergeLocalProviderOverlaysAddsLocalProvidersWhenMissing(t *testing.T) {
	providers, overlays := mergeLocalProviderOverlays([]crushapi.Provider{{ID: "openai", Name: "OpenAI"}})
	if len(providers) != 3 {
		t.Fatalf("provider count = %d, want 3", len(providers))
	}
	mistral := findProvider(t, providers, mistralProviderID)
	if mistral.Type != openAICompatType || mistral.APIEndpoint != mistralDefaultEndpoint {
		t.Fatalf("Mistral overlay = %#v", mistral)
	}
	if len(mistral.Models) < 3 || !mistral.Models[0].SupportsVision {
		t.Fatalf("Mistral models = %#v", mistral.Models)
	}
	// Codex ships without models: its catalog is account-scoped and arrives
	// with the ChatGPT sign-in.
	codex := findProvider(t, providers, codexProviderID)
	if codex.Type != codexProviderType || codex.APIEndpoint != codexBackendURL || len(codex.Models) != 0 {
		t.Fatalf("Codex overlay = %#v", codex)
	}
	// The public OpenAI provider must survive untouched next to Codex.
	if openai := findProvider(t, providers, openAIProviderID); openai.Name != "OpenAI" {
		t.Fatalf("OpenAI provider = %#v", openai)
	}
	if !overlays[mistralProviderID] || !overlays[codexProviderID] {
		t.Fatalf("local overlays = %#v", overlays)
	}
}

func TestMergeLocalProviderOverlaysLetsUpstreamMistralWin(t *testing.T) {
	upstream := crushapi.Provider{
		ID:          mistralProviderID,
		Name:        "Mistral upstream",
		Type:        "openai-compat",
		APIEndpoint: "https://upstream.example/v1",
		Models:      []crushapi.Model{{ID: "upstream-model", Name: "Upstream model"}},
	}
	providers, overlays := mergeLocalProviderOverlays([]crushapi.Provider{upstream})
	mistral := findProvider(t, providers, mistralProviderID)
	if mistral.Name != upstream.Name || mistral.APIEndpoint != upstream.APIEndpoint || mistral.Models[0].ID != "upstream-model" {
		t.Fatalf("upstream Mistral was changed: %#v", mistral)
	}
	if overlays[mistralProviderID] {
		t.Fatal("upstream Mistral must not be marked as a local overlay")
	}
}

func TestMergeProviderModelsKeepsConfiguredMetadataFirst(t *testing.T) {
	configured := []crushapi.Model{{ID: "mistral-medium-3-5", Name: "Configured", SupportsVision: false}}
	fallback := []crushapi.Model{
		{ID: "mistral-medium-3-5", Name: "Overlay", SupportsVision: true},
		{ID: "mistral-small-2603", Name: "Small", SupportsVision: true},
	}
	models := mergeProviderModels(configured, fallback)
	if len(models) != 2 || models[0].Name != "Configured" || models[0].SupportsVision || models[1].ID != "mistral-small-2603" {
		t.Fatalf("merged models = %#v", models)
	}
}

func TestPrepareLocalProviderConfigSeedsCrushWireFields(t *testing.T) {
	var setKeys []string
	transport := catalogRoundTripper(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/workspaces/ws/providers":
			return jsonHTTPResponse(http.StatusOK, `[{"id":"openai","name":"OpenAI"}]`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/workspaces/ws/config":
			return jsonHTTPResponse(http.StatusOK, `{"providers":{}}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/workspaces/ws/config/set":
			var payload struct {
				Key   string          `json:"key"`
				Value json.RawMessage `json:"value"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode config set: %v", err)
			}
			setKeys = append(setKeys, payload.Key)
			return jsonHTTPResponse(http.StatusOK, `{}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return jsonHTTPResponse(http.StatusNotFound, `{}`), nil
		}
	})
	api := crushapi.NewClient(&http.Client{Transport: transport})
	seeded, err := prepareLocalProviderConfig(context.Background(), api, "ws", crushapi.ConfigScopeGlobal, mistralProviderID)
	if err != nil {
		t.Fatalf("prepareLocalProviderConfig() error = %v", err)
	}
	if !seeded {
		t.Fatal("prepareLocalProviderConfig() did not seed Mistral")
	}
	want := []string{
		"providers.mistral.discover_models",
		"providers.mistral.name",
		"providers.mistral.type",
		"providers.mistral.models",
		"providers.mistral.base_url",
		"providers.mistral.api_key",
	}
	if len(setKeys) != len(want) {
		t.Fatalf("set keys = %#v, want %#v", setKeys, want)
	}
	for i := range want {
		if setKeys[i] != want[i] {
			t.Fatalf("set keys = %#v, want %#v", setKeys, want)
		}
	}
}

// Codex must be seeded without an api_key or a models list: the credential is
// an OAuth token and the catalog is written by the ChatGPT sign-in.
func TestPrepareLocalProviderConfigOmitsCredentialFieldsForCodex(t *testing.T) {
	var setKeys []string
	transport := catalogRoundTripper(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/workspaces/ws/providers":
			return jsonHTTPResponse(http.StatusOK, `[{"id":"openai","name":"OpenAI"}]`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/workspaces/ws/config":
			return jsonHTTPResponse(http.StatusOK, `{"providers":{}}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/workspaces/ws/config/set":
			var payload struct {
				Key string `json:"key"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode config set: %v", err)
			}
			setKeys = append(setKeys, payload.Key)
			return jsonHTTPResponse(http.StatusOK, `{}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return jsonHTTPResponse(http.StatusNotFound, `{}`), nil
		}
	})
	api := crushapi.NewClient(&http.Client{Transport: transport})
	seeded, err := prepareLocalProviderConfig(context.Background(), api, "ws", crushapi.ConfigScopeGlobal, codexProviderID)
	if err != nil {
		t.Fatalf("prepareLocalProviderConfig() error = %v", err)
	}
	if !seeded {
		t.Fatal("prepareLocalProviderConfig() did not seed Codex")
	}
	want := []string{
		"providers.codex.discover_models",
		"providers.codex.name",
		"providers.codex.type",
		"providers.codex.base_url",
	}
	if len(setKeys) != len(want) {
		t.Fatalf("set keys = %#v, want %#v", setKeys, want)
	}
	for i := range want {
		if setKeys[i] != want[i] {
			t.Fatalf("set keys = %#v, want %#v", setKeys, want)
		}
	}
}

// Finalizing an OAuth-only provider must not call back into the engine: there
// is no API key to validate and no catalog to discover.
func TestFinalizeLocalProviderConfigSkipsCodex(t *testing.T) {
	transport := catalogRoundTripper(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		return jsonHTTPResponse(http.StatusNotFound, `{}`), nil
	})
	api := crushapi.NewClient(&http.Client{Transport: transport})
	if err := finalizeLocalProviderConfig(context.Background(), api, "ws", crushapi.ConfigScopeGlobal, codexProviderID); err != nil {
		t.Fatalf("finalizeLocalProviderConfig() error = %v", err)
	}
}
