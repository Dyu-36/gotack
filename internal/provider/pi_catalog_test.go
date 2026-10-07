package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/Dyu-36/gotack/internal/engineapi"
	"github.com/Dyu-36/gotack/internal/modelcatalog"
)

func TestPiCatalogMapsAliasesMixedTransportsAndMetadata(t *testing.T) {
	models := []modelcatalog.Model{
		{
			ID: "mistral-small", Name: "Mistral Small", API: "mistral-conversations", Provider: "mistral",
			BaseURL: "https://api.mistral.ai", Input: []string{"text", "image"}, Reasoning: true,
			ContextWindow: 128000, MaxTokens: 8192,
			Cost:             modelcatalog.Cost{Input: 0.2, Output: 0.6, CacheRead: 0.02, CacheWrite: 0.1},
			ThinkingLevelMap: map[string]json.RawMessage{"off": json.RawMessage("null"), "low": json.RawMessage(`"low"`), "medium": json.RawMessage(`"medium"`)},
			Headers:          map[string]string{"X-Pi-Route": "mistral"},
		},
		{ID: "openai", Name: "OpenAI", API: "openai-responses", BaseURL: "https://api.openai.com/v1", Input: []string{"text"}},
		{ID: "claude", Name: "Claude", API: "anthropic-messages", BaseURL: "https://api.anthropic.com", Input: []string{"text"}},
		{ID: "unsupported", API: "pi-messages", BaseURL: "https://pi.dev", Input: []string{"text"}},
	}
	got, routes := supportedPiModels("mistral", models, "https://api.mistral.ai/v1", false)
	if len(got) != 3 {
		t.Fatalf("supportedPiModels() returned %d models, want 3 supported transports", len(got))
	}
	byID := make(map[string]engineapi.Model, len(got))
	for _, model := range got {
		byID[model.ID] = model
	}
	model := byID["mistral-small"]
	if model.ID != "mistral-small" || !model.SupportsVision || !model.CanReason {
		t.Fatalf("mapped model = %#v", model)
	}
	if model.DefaultReasoningEffort != "medium" || !slices.Equal(model.ReasoningLevels, []string{"none", "low", "medium"}) {
		t.Fatalf("reasoning metadata = %#v", model)
	}
	if model.CostPer1MIn != 0.2 || model.CostPer1MOut != 0.6 || model.CostPer1MInCached != 0.02 || model.CostPer1MOutCached != 0.1 {
		t.Fatalf("cost metadata = %#v", model)
	}
	route := routes[model.ID]
	if route.API != "openai-completions" || route.BaseURL != "" || route.Headers["X-Pi-Route"] != "mistral" {
		t.Fatalf("route = %#v", route)
	}
	if routes["openai"].API != "openai-responses" || routes["claude"].API != "anthropic-messages" || routes["unsupported"].API != "" {
		t.Fatalf("mixed transport routes = %#v", routes)
	}
	if normalizePiProviderID("openai-codex") != CodexID || normalizePiProviderID("github-copilot") != "copilot" ||
		normalizePiProviderID("amazon-bedrock") != "bedrock" || normalizePiProviderID("google-vertex") != "vertexai" {
		t.Fatal("Pi provider aliases were not normalized")
	}
	if normalizePiProviderID("moonshotai-cn") != "moonshotai-cn" {
		t.Fatal("China Moonshot provider ID must retain its own endpoint identity")
	}
	if got := canonicalPiAPI("openai-codex", "openai-codex-responses"); got != "" {
		t.Fatalf("Codex Pi API should be account-scoped, got %q", got)
	}
}

func TestMergePiModelsUpdatesAndPrunesManagedEntries(t *testing.T) {
	priorRoute := engineapi.ModelRoute{API: "openai-completions", Headers: map[string]string{"X-Route": "old"}}
	updatedRoute := engineapi.ModelRoute{API: "openai-responses", Headers: map[string]string{"X-Route": "new"}}
	customRoute := engineapi.ModelRoute{API: "anthropic-messages", Headers: map[string]string{"X-Route": "custom"}}
	managed := engineapi.Model{ID: "managed", Name: "Old name", ContextWindow: 10}
	stale := engineapi.Model{ID: "stale", Name: "Stale model"}
	baseline := engineapi.Model{ID: "old-default", Name: "Old default"}
	userEdited := engineapi.Model{ID: "managed", Name: "Edited by user", ContextWindow: 10}
	custom := engineapi.Model{ID: "custom-model", Name: "Custom model"}
	newModel := engineapi.Model{ID: "new-model", Name: "New model", CanReason: true}

	previous := map[string]piManagedModel{
		"managed": {Model: piModelFromEngineModel(managed), Route: priorRoute},
		"stale":   {Model: piModelFromEngineModel(stale), Route: priorRoute},
	}
	current := []engineapi.Model{managed, stale, baseline, userEdited, custom}
	currentRoutes := map[string]engineapi.ModelRoute{
		"managed":        priorRoute,
		"stale":          priorRoute,
		"managed-edited": customRoute,
		"custom-model":   customRoute,
	}
	// Use a separately named edited model so its ID does not collide with the
	// earlier pristine entry in the test fixture.
	current = []engineapi.Model{managed, stale, baseline, userEdited, custom}
	userEdited.ID = "edited-model"
	current[3] = userEdited
	currentRoutes["edited-model"] = customRoute
	delete(currentRoutes, "managed-edited")

	got, manifest, routes := mergePiModels(
		current,
		[]engineapi.Model{baseline},
		previous,
		[]engineapi.Model{newModel},
		currentRoutes,
		map[string]engineapi.ModelRoute{"new-model": updatedRoute},
		map[string]bool{"stale": true},
	)
	ids := make([]string, 0, len(got))
	for _, model := range got {
		ids = append(ids, model.ID)
	}
	slices.Sort(ids)
	want := []string{"custom-model", "edited-model", "new-model", "stale"}
	if !slices.Equal(ids, want) {
		t.Fatalf("merged model IDs = %v, want %v", ids, want)
	}
	if got[0].ID == "" {
		t.Fatal("merged models contain an empty ID")
	}
	if routes["edited-model"].Headers["X-Route"] != "custom" || routes["new-model"].Headers["X-Route"] != "new" {
		t.Fatalf("merged routes = %#v", routes)
	}
	if _, exists := manifest["stale"]; !exists {
		t.Fatal("selected removed model should remain managed until selection changes")
	}
	if _, exists := manifest["old-default"]; exists {
		t.Fatal("removed engine default should not become Pi-managed")
	}
	if _, exists := manifest["edited-model"]; exists {
		t.Fatal("user-edited model should not remain Pi-managed")
	}
	if _, exists := manifest["new-model"]; !exists {
		t.Fatal("new Pi model is not marked as managed")
	}
	afterSwitch, afterManifest, _ := mergePiModels(got, nil, manifest, []engineapi.Model{newModel}, routes, map[string]engineapi.ModelRoute{"new-model": updatedRoute}, nil)
	for _, model := range afterSwitch {
		if model.ID == "stale" {
			t.Fatal("removed model remained after its selection changed")
		}
	}
	if _, exists := afterManifest["stale"]; exists {
		t.Fatal("removed model remained managed after its selection changed")
	}
}

func TestPiCatalogPreservesCustomEndpointAndUsesPerModelEndpointWhenUnmodified(t *testing.T) {
	models := []modelcatalog.Model{
		{ID: "a", API: "openai-completions", BaseURL: "https://one.example/v1", Input: []string{"text"}},
		{ID: "b", API: "openai-responses", BaseURL: "https://two.example/v1", Input: []string{"text"}},
	}
	got, routes := supportedPiModels("example", models, "https://one.example/v1", false)
	if len(got) != 2 || routes["b"].BaseURL != "https://two.example/v1" {
		t.Fatalf("per-model endpoint route = %#v, models = %#v", routes["b"], got)
	}
	got, routes = supportedPiModels("example", models, "https://custom.example/v1", true)
	if len(got) != 2 || routes["a"].BaseURL != "" || routes["b"].BaseURL != "" {
		t.Fatalf("custom endpoint should be inherited by routes: %#v", routes)
	}
	if !hasCustomPiEndpoint("https://custom.example/v1", "https://custom.example/v1", "https://one.example/v1") {
		t.Fatal("a configured endpoint that matches runtime metadata but differs from Pi must be preserved as a custom override")
	}
	if hasCustomPiEndpoint("https://one.example/v1", "https://engine-default.example/v1", "https://one.example/v1") {
		t.Fatal("the current Pi endpoint should not be treated as a custom override")
	}
}

func TestPiCatalogExcludesOpenRouterBatchVariants(t *testing.T) {
	t.Parallel()
	models := []modelcatalog.Model{
		{ID: "deepseek/deepseek-v4.1-flash", API: "openai-completions", Type: "chat", Input: []string{"text"}},
		{ID: "deepseek/deepseek-v4.1-flash:batch", API: "openai-completions", Type: "chat", Input: []string{"text"}},
		{ID: "deepseek/deepseek-v4.1-flash:batch:nitro", API: "openai-completions", Type: "chat"},
		{ID: "deepseek/deepseek-v4.1-flash:nitro:batch", API: "openai-completions", Type: "chat"},
		{ID: "vendor/model:free", API: "openai-completions", Type: "chat"},
		{ID: "vendor/model:nitro", API: "openai-completions", Type: "chat"},
	}
	got, routes := supportedPiModels("openrouter", models, "https://openrouter.ai/api/v1", false)
	var ids []string
	for _, model := range got {
		ids = append(ids, model.ID)
	}
	want := []string{"deepseek/deepseek-v4.1-flash", "vendor/model:free", "vendor/model:nitro"}
	if !slices.Equal(ids, want) || len(routes) != len(want) {
		t.Fatalf("chat models = %v, routes = %v; want %v", ids, routes, want)
	}
}

func TestPiCatalogRemovesSavedOpenRouterBatchModels(t *testing.T) {
	for _, operation := range []string{"list", "prepare", "disabled"} {
		t.Run(operation, func(t *testing.T) {
			t.Parallel()
			const baseID = "deepseek/deepseek-v4.1-flash"
			const batchID = baseID + ":batch"
			base := modelcatalog.Model{ID: baseID, API: "openai-completions", BaseURL: "https://openrouter.ai/api/v1", Type: "chat"}
			batch := base
			batch.ID = batchID
			source := piTestCatalogSource{"openrouter": {baseID: base, batchID: batch}}
			current := []engineapi.Model{{ID: batchID, Name: "Saved batch model"}, {ID: "custom:free", Name: "Custom free model"}}
			cfg := engineapi.WorkspaceConfig{
				Providers: map[string]engineapi.ProviderConfig{"openrouter": {
					Name: "OpenRouter", Type: "openai-compat", APIKey: "test-key", BaseURL: base.BaseURL,
					Disable: operation == "disabled", Models: current,
					ModelRoutes: map[string]engineapi.ModelRoute{batchID: {API: "openai-completions"}},
				}},
				Models: map[string]engineapi.SelectedModel{"large": {Provider: "openrouter", Model: batchID}},
			}
			var writtenModels []engineapi.Model
			var writtenRoutes map[string]engineapi.ModelRoute
			transport := piTestTransport(func(req *http.Request) (*http.Response, error) {
				var body any
				switch req.Method + " " + req.URL.Path {
				case "GET /v1/workspaces/ws/providers":
					body = []engineapi.Provider{{ID: "openrouter", Name: "OpenRouter", Type: "openai-compat", Models: current[:1], DefaultLargeModelID: batchID}}
				case "GET /v1/workspaces/ws/config":
					body = cfg
				case "POST /v1/workspaces/ws/config/set-batch":
					var payload struct {
						Fields map[string]json.RawMessage `json:"fields"`
					}
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatal(err)
					}
					if err := json.Unmarshal(payload.Fields["providers.openrouter.models"], &writtenModels); err != nil {
						t.Fatal(err)
					}
					if err := json.Unmarshal(payload.Fields["providers.openrouter.model_routes"], &writtenRoutes); err != nil {
						t.Fatal(err)
					}
					body = map[string]any{}
				default:
					t.Fatalf("unexpected engine request: %s %s", req.Method, req.URL.Path)
				}
				data, err := json.Marshal(body)
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(data)))}, err
			})
			api := engineapi.NewClient(&http.Client{Transport: transport})
			catalog := NewPiCatalog(source, "")
			if operation == "prepare" {
				if err := catalog.Prepare(t.Context(), api, "ws", "openrouter"); err != nil {
					t.Fatal(err)
				}
			} else {
				providers, err := catalog.List(t.Context(), api, "ws")
				if err != nil {
					t.Fatal(err)
				}
				entry, ok := findProvider(providers, "openrouter")
				if !ok || entry.DefaultLargeModelID == batchID {
					t.Fatalf("OpenRouter defaults still select batch: %#v", entry)
				}
				assertPiChatModelIDs(t, entry.Models, []string{"custom:free", baseID})
			}
			if operation == "disabled" {
				if writtenModels != nil {
					t.Fatal("disabled provider should not be synchronized")
				}
				return
			}
			assertPiChatModelIDs(t, writtenModels, []string{"custom:free", baseID})
			if _, exists := writtenRoutes[batchID]; exists {
				t.Fatal("saved batch route was not removed")
			}
			if cfg.Providers["openrouter"].Models[0].ID != batchID {
				t.Fatal("filter mutated the original config slice")
			}
		})
	}
}

func assertPiChatModelIDs(t *testing.T, models []engineapi.Model, want []string) {
	t.Helper()
	ids := make([]string, 0, len(models))
	for _, model := range models {
		ids = append(ids, model.ID)
	}
	if !slices.Equal(ids, want) {
		t.Fatalf("chat model IDs = %v, want %v", ids, want)
	}
}

type piTestCatalogSource modelcatalog.Catalog

func (s piTestCatalogSource) Load(context.Context) (modelcatalog.Catalog, error) {
	return modelcatalog.Catalog(s), nil
}

type piTestTransport func(*http.Request) (*http.Response, error)

func (f piTestTransport) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestExplicitConfiguredModelsHidesMissingPiDefaultsButKeepsCustomAndSelected(t *testing.T) {
	defaults := []engineapi.Model{
		{ID: "missing-pi", Name: "Old default"},
		{ID: "edited-default", Name: "Original name"},
	}
	configured := []engineapi.Model{
		{ID: "missing-pi", Name: "Old default"},
		{ID: "edited-default", Name: "User name"},
		{ID: "custom", Name: "Custom model"},
	}
	got := explicitConfiguredModels(configured, defaults, map[string]bool{"missing-pi": true})
	ids := make([]string, 0, len(got))
	for _, model := range got {
		ids = append(ids, model.ID)
	}
	if !slices.Equal(ids, []string{"missing-pi", "edited-default", "custom"}) {
		t.Fatalf("explicitConfiguredModels() IDs = %v", ids)
	}
	got = explicitConfiguredModels(configured, defaults, nil)
	ids = ids[:0]
	for _, model := range got {
		ids = append(ids, model.ID)
	}
	if !slices.Equal(ids, []string{"edited-default", "custom"}) {
		t.Fatalf("unselected missing default was not hidden: %v", ids)
	}
}

func TestApplyCredentialStatusUsesResolvedWorkspaceCredential(t *testing.T) {
	provider := engineapi.Provider{ID: "openai", Configured: false}
	applyCredentialStatus(&provider, engineapi.ProviderConfig{APIKey: "sk-workspace"}, true)
	if !provider.Configured || provider.CredentialKind != "api_key" {
		t.Fatalf("resolved API key status = configured %v, kind %q", provider.Configured, provider.CredentialKind)
	}

	t.Setenv("GOTACK_PI_TEST_MISSING_CREDENTIAL", "")
	applyCredentialStatus(&provider, engineapi.ProviderConfig{APIKey: "$GOTACK_PI_TEST_MISSING_CREDENTIAL"}, true)
	if provider.Configured || provider.CredentialKind != "" {
		t.Fatalf("unresolved environment reference status = configured %v, kind %q", provider.Configured, provider.CredentialKind)
	}

	provider.Configured = true
	provider.CredentialKind = "oauth"
	applyCredentialStatus(&provider, engineapi.ProviderConfig{APIKey: "sk-disabled", Disable: true}, true)
	if provider.Configured || provider.CredentialKind != "" {
		t.Fatalf("disabled provider status = configured %v, kind %q", provider.Configured, provider.CredentialKind)
	}

	provider.Configured = true
	provider.CredentialKind = "runtime"
	applyCredentialStatus(&provider, engineapi.ProviderConfig{}, false)
	if !provider.Configured || provider.CredentialKind != "runtime" {
		t.Fatalf("missing workspace config should preserve runtime status: %+v", provider)
	}
}
