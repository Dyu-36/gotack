package main

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/crushapi"
	"github.com/Dyu-36/gotack/internal/session"
	"github.com/Dyu-36/gotack/internal/workspace"
)

func TestGetChatGPTOAuthStatus_Connected(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("AppData", configRoot)
	t.Setenv("XDG_CONFIG_HOME", configRoot)

	transport := catalogRoundTripper(func(req *http.Request) (*http.Response, error) {
		var body string
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/v1/workspaces":
			body = `{"id":"catalog-ws","path":"` + filepath.ToSlash(configRoot) + `"}`
		case req.Method == http.MethodGet && req.URL.Path == "/v1/workspaces/catalog-ws/config":
			body = `{"providers":{"codex":{"id":"codex","name":"ChatGPT (Codex)","oauth":{"access_token":"valid-token","account_id":"acc-123","account_email":"user@test.local","chatgpt_plan_type":"plus"}}}}`
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
			return jsonHTTPResponse(http.StatusNotFound, `{}`), nil
		}
		return jsonHTTPResponse(http.StatusOK, body), nil
	})

	api := crushapi.NewClient(&http.Client{Transport: transport})
	ws := workspace.NewService(api)
	app := NewApp()
	app.ctx = context.Background()
	app.swapConn(func(c *conn) *conn {
		c.api = api
		c.ws = ws
		c.sess = session.NewService(api, ws)
		return c
	})

	scope, started := app.link.BeginConnect(context.Background())
	if !started {
		t.Fatal("fresh link must accept a connect attempt")
	}
	if !app.link.CommitAttach(scope, crushapi.Endpoint{}, "test") {
		t.Fatal("commit attach rejected a live scope")
	}
	app.link.MarkRunning()

	status, err := app.GetChatGPTOAuthStatus()
	if err != nil {
		t.Fatalf("GetChatGPTOAuthStatus() error = %v", err)
	}

	if !status.Connected {
		t.Errorf("got connected = false, want true")
	}
	if status.Email != "user@test.local" {
		t.Errorf("got email = %q, want user@test.local", status.Email)
	}
	if status.Plan != "plus" {
		t.Errorf("got plan = %q, want plus", status.Plan)
	}
}

func TestGetChatGPTOAuthStatus_Disconnected(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("AppData", configRoot)
	t.Setenv("XDG_CONFIG_HOME", configRoot)

	transport := catalogRoundTripper(func(req *http.Request) (*http.Response, error) {
		var body string
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/v1/workspaces":
			body = `{"id":"catalog-ws","path":"` + filepath.ToSlash(configRoot) + `"}`
		case req.Method == http.MethodGet && req.URL.Path == "/v1/workspaces/catalog-ws/config":
			body = `{"providers":{"codex":{"id":"codex","name":"ChatGPT (Codex)","disable":true}}}`
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
			return jsonHTTPResponse(http.StatusNotFound, `{}`), nil
		}
		return jsonHTTPResponse(http.StatusOK, body), nil
	})

	api := crushapi.NewClient(&http.Client{Transport: transport})
	ws := workspace.NewService(api)
	app := NewApp()
	app.ctx = context.Background()
	app.swapConn(func(c *conn) *conn {
		c.api = api
		c.ws = ws
		c.sess = session.NewService(api, ws)
		return c
	})

	scope, started := app.link.BeginConnect(context.Background())
	if !started {
		t.Fatal("fresh link must accept a connect attempt")
	}
	if !app.link.CommitAttach(scope, crushapi.Endpoint{}, "test") {
		t.Fatal("commit attach rejected a live scope")
	}
	app.link.MarkRunning()

	status, err := app.GetChatGPTOAuthStatus()
	if err != nil {
		t.Fatalf("GetChatGPTOAuthStatus() error = %v", err)
	}

	if status.Connected {
		t.Errorf("got connected = true, want false")
	}
}

func TestSelectChatGPTModelUsesLiveCatalog(t *testing.T) {
	providers := []crushapi.Provider{{
		ID:                  codexProviderID,
		DefaultLargeModelID: "gpt-default",
		Models:              []crushapi.Model{{ID: "gpt-default"}, {ID: "gpt-existing"}},
	}}

	model, err := selectChatGPTModel(providers, "gpt-existing")
	if err != nil || model != "gpt-existing" {
		t.Fatalf("existing model: got %q, %v", model, err)
	}
	model, err = selectChatGPTModel(providers, "gpt-retired")
	if err != nil || model != "gpt-default" {
		t.Fatalf("fallback model: got %q, %v", model, err)
	}
}

// A pre-split install keeps its ChatGPT login on the "openai" provider. Asking
// for status must move that credential to Codex instead of reporting the user
// as signed out and asking for a second sign-in.
func TestGetChatGPTOAuthStatusMigratesLegacyCredential(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("AppData", configRoot)
	t.Setenv("XDG_CONFIG_HOME", configRoot)

	const legacyConfig = `{"providers":{"openai":{"id":"openai","name":"OpenAI","api_key":"valid-token","base_url":"https://chatgpt.com/backend-api/codex","oauth":{"access_token":"valid-token","account_id":"acc-123","account_email":"user@test.local","chatgpt_plan_type":"plus"}}}}`
	const codexConfig = `{"providers":{"codex":{"id":"codex","name":"ChatGPT (Codex)","oauth":{"access_token":"valid-token","account_id":"acc-123","account_email":"user@test.local","chatgpt_plan_type":"plus"}}}}`

	var (
		moved        bool
		credentialTo []string
		removedKeys  []string
		modelWrites  []string
	)
	transport := catalogRoundTripper(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/v1/workspaces":
			return jsonHTTPResponse(http.StatusOK, `{"id":"catalog-ws","path":"`+filepath.ToSlash(configRoot)+`"}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/workspaces/catalog-ws/config":
			if moved {
				return jsonHTTPResponse(http.StatusOK, codexConfig), nil
			}
			return jsonHTTPResponse(http.StatusOK, legacyConfig), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/workspaces/catalog-ws/providers":
			return jsonHTTPResponse(http.StatusOK, `[{"id":"codex","name":"ChatGPT (Codex)","default_large_model_id":"gpt-5-codex","models":[{"id":"gpt-5-codex"}]}]`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/workspaces/catalog-ws/config/set-batch":
			return jsonHTTPResponse(http.StatusOK, `{}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/workspaces/catalog-ws/config/models":
			var payload struct {
				Models map[string]struct {
					Provider string `json:"provider"`
					Model    string `json:"model"`
				} `json:"models"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode preferred models: %v", err)
			}
			for _, modelType := range []string{"large", "small"} {
				model := payload.Models[modelType]
				modelWrites = append(modelWrites, modelType+":"+model.Provider+"/"+model.Model)
			}
			return jsonHTTPResponse(http.StatusOK, `{}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/workspaces/catalog-ws/config/provider-key":
			var payload struct {
				ProviderID string `json:"provider_id"`
				Kind       string `json:"kind"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode provider key: %v", err)
			}
			credentialTo = append(credentialTo, payload.ProviderID+":"+payload.Kind)
			moved = true
			return jsonHTTPResponse(http.StatusOK, `{}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/workspaces/catalog-ws/config/remove":
			var payload struct {
				Key string `json:"key"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode config remove: %v", err)
			}
			removedKeys = append(removedKeys, payload.Key)
			return jsonHTTPResponse(http.StatusOK, `{}`), nil
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
			return jsonHTTPResponse(http.StatusNotFound, `{}`), nil
		}
	})

	api := crushapi.NewClient(&http.Client{Transport: transport})
	ws := workspace.NewService(api)
	app := NewApp()
	app.ctx = context.Background()
	app.swapConn(func(c *conn) *conn {
		c.api = api
		c.ws = ws
		c.sess = session.NewService(api, ws)
		return c
	})

	scope, started := app.link.BeginConnect(context.Background())
	if !started {
		t.Fatal("fresh link must accept a connect attempt")
	}
	if !app.link.CommitAttach(scope, crushapi.Endpoint{}, "test") {
		t.Fatal("commit attach rejected a live scope")
	}
	app.link.MarkRunning()

	// An upgraded install still names the pre-split provider, and its saved model
	// id predates the account-scoped Codex catalog.
	app.cfg = &appconfig.Config{
		Provider:  openAIProviderID,
		Model:     "gpt-retired",
		Thinking:  "high",
		CustomURL: codexBackendURL,
	}

	status, err := app.GetChatGPTOAuthStatus()
	if err != nil {
		t.Fatalf("GetChatGPTOAuthStatus() error = %v", err)
	}
	if !status.Connected || status.Email != "user@test.local" || status.Plan != "plus" {
		t.Fatalf("status = %#v", status)
	}
	if len(credentialTo) != 1 || credentialTo[0] != codexProviderID+":oauth" {
		t.Fatalf("credential writes = %#v, want one oauth write to codex", credentialTo)
	}
	for _, key := range []string{
		"providers.openai.oauth",
		"providers.openai.api_key",
		"providers.openai.base_url",
		"providers.openai.models",
	} {
		if !slices.Contains(removedKeys, key) {
			t.Fatalf("removed keys = %#v, want %q", removedKeys, key)
		}
	}
	// Without the repoint both preferences would keep pointing at a provider that
	// no longer holds a credential, and the next settings replay would restore it.
	wantModels := []string{"large:" + codexProviderID + "/gpt-5-codex", "small:" + codexProviderID + "/gpt-5-codex"}
	if !slices.Equal(modelWrites, wantModels) {
		t.Fatalf("model writes = %#v, want %#v", modelWrites, wantModels)
	}
	if app.cfg.Provider != codexProviderID || app.cfg.Model != "gpt-5-codex" {
		t.Fatalf("saved selection = %q/%q, want codex/gpt-5-codex", app.cfg.Provider, app.cfg.Model)
	}
	if app.cfg.CustomURL != "" {
		t.Fatalf("custom endpoint = %q, want it cleared for an OAuth provider", app.cfg.CustomURL)
	}
}

// A provider the user selected deliberately must survive the migration: only
// the pre-split ChatGPT selection is taken over by Codex.
func TestRepointSavedModelAtCodexKeepsDeliberateProvider(t *testing.T) {
	transport := catalogRoundTripper(func(req *http.Request) (*http.Response, error) {
		t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		return jsonHTTPResponse(http.StatusNotFound, `{}`), nil
	})

	api := crushapi.NewClient(&http.Client{Transport: transport})
	app := NewApp()
	app.ctx = context.Background()
	app.cfg = &appconfig.Config{Provider: "anthropic", Model: "claude-sonnet"}

	app.repointSavedModelAtCodex(&bridgeServices{api: api, ws: workspace.NewService(api)}, "catalog-ws")

	if app.cfg.Provider != "anthropic" || app.cfg.Model != "claude-sonnet" {
		t.Fatalf("saved selection = %q/%q, want anthropic/claude-sonnet", app.cfg.Provider, app.cfg.Model)
	}
}

// A build that moved the credential without repointing the selection leaves an
// install the migration no longer sees. The repair gate must recognise exactly
// that state and nothing else.
func TestSelectionStrandedOnLegacyOpenAI(t *testing.T) {
	credential := json.RawMessage(`{"access_token":"tok","account_id":"acc-123"}`)
	withCodex := func(legacy crushapi.ProviderConfig) crushapi.WorkspaceConfig {
		return crushapi.WorkspaceConfig{Providers: map[string]crushapi.ProviderConfig{
			codexProviderID:  {OAuth: credential},
			openAIProviderID: legacy,
		}}
	}

	cases := []struct {
		name     string
		cfg      crushapi.WorkspaceConfig
		provider string
		want     bool
	}{
		{"half-migrated install", withCodex(crushapi.ProviderConfig{Disable: true}), openAIProviderID, true},
		{"no saved provider yet", withCodex(crushapi.ProviderConfig{}), "", true},
		{"openai still holds an api key", withCodex(crushapi.ProviderConfig{APIKey: "sk-test"}), openAIProviderID, false},
		{"deliberately chosen provider", withCodex(crushapi.ProviderConfig{}), "anthropic", false},
		{"already repointed", withCodex(crushapi.ProviderConfig{}), codexProviderID, false},
		{"codex holds no credential", crushapi.WorkspaceConfig{Providers: map[string]crushapi.ProviderConfig{openAIProviderID: {}}}, openAIProviderID, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := selectionStrandedOnLegacyOpenAI(tc.cfg, tc.provider); got != tc.want {
				t.Fatalf("selectionStrandedOnLegacyOpenAI() = %v, want %v", got, tc.want)
			}
		})
	}
}
