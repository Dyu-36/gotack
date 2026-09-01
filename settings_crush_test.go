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

// The webview replays the selection it read at boot. When that snapshot still
// names the pre-split "openai" provider, applying it verbatim would point the
// engine back at a provider that holds no credential and undo the Codex
// migration seconds after it ran.
func TestApplyEffectiveCrushSettingsRedirectsStaleSelection(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("AppData", configRoot)
	t.Setenv("XDG_CONFIG_HOME", configRoot)

	const migratedConfig = `{"providers":{"codex":{"id":"codex","name":"ChatGPT (Codex)","base_url":"https://chatgpt.com/backend-api/codex","models":[{"id":"gpt-subscription"},{"id":"gpt-other"}],"oauth":{"access_token":"tok","account_id":"acc-123"}},"openai":{"id":"openai","disable":true}}}`

	var modelWrites []string
	transport := catalogRoundTripper(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/workspaces":
			return jsonHTTPResponse(http.StatusOK, `[]`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/workspaces":
			return jsonHTTPResponse(http.StatusOK, `{"id":"ws-1","path":"`+filepath.ToSlash(configRoot)+`"}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/workspaces/ws-1/config":
			return jsonHTTPResponse(http.StatusOK, migratedConfig), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/workspaces/ws-1/providers":
			// Crush never advertises Codex; the entry comes from the local overlay
			// and the subscription models from provider config.
			return jsonHTTPResponse(http.StatusOK, `[{"id":"openai","name":"OpenAI"}]`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/workspaces/ws-1/config/model":
			var payload struct {
				ModelType string `json:"model_type"`
				Model     struct {
					Provider string `json:"provider"`
					Model    string `json:"model"`
				} `json:"model"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode preferred model: %v", err)
			}
			modelWrites = append(modelWrites, payload.ModelType+":"+payload.Model.Provider+"/"+payload.Model.Model)
			return jsonHTTPResponse(http.StatusOK, `{}`), nil
		default:
			// Provider seeding and agent initialisation are not what this test pins
			// down; they only have to succeed.
			return jsonHTTPResponse(http.StatusOK, `{}`), nil
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
	if _, err := ws.Open(context.Background(), configRoot); err != nil {
		t.Fatalf("open workspace: %v", err)
	}

	app.cfg = &appconfig.Config{Provider: openAIProviderID, Model: "gpt-subscription", Thinking: "high"}
	stale := SettingsInfo{
		Provider:  openAIProviderID,
		Model:     "gpt-subscription",
		Thinking:  "high",
		CustomURL: codexBackendURL,
	}

	effective, err := app.applyEffectiveCrushSettings(stale, "")
	if err != nil {
		t.Fatalf("applyEffectiveCrushSettings() error = %v", err)
	}

	if effective.Provider != codexProviderID || effective.Model != "gpt-subscription" {
		t.Fatalf("effective selection = %q/%q, want codex/gpt-subscription", effective.Provider, effective.Model)
	}
	// A Codex credential is only valid against the endpoint the engine picked for
	// it, so an endpoint inherited from the API-key provider must not survive.
	if effective.CustomURL != "" {
		t.Fatalf("effective endpoint = %q, want it cleared", effective.CustomURL)
	}
	wantModels := []string{"large:" + codexProviderID + "/gpt-subscription", "small:" + codexProviderID + "/gpt-subscription"}
	if !slices.Equal(modelWrites, wantModels) {
		t.Fatalf("model writes = %#v, want %#v", modelWrites, wantModels)
	}
}

// A deliberate API-key setup on "openai" must never be turned into a Codex
// selection, so the redirect refuses before it reads any credential.
func TestChatGPTRedirectCandidate(t *testing.T) {
	cases := []struct {
		name     string
		settings SettingsInfo
		apiKey   string
		want     bool
	}{
		{"stale selection replayed by the ui", SettingsInfo{Provider: openAIProviderID}, "", true},
		{"legacy id spelled with padding", SettingsInfo{Provider: " openai "}, "", true},
		{"credential named explicitly", SettingsInfo{Provider: openAIProviderID, CredentialProvider: openAIProviderID}, "", true},
		{"api key being stored", SettingsInfo{Provider: openAIProviderID}, "sk-test", false},
		{"provider-only enable", SettingsInfo{Provider: openAIProviderID, ProviderOnly: true}, "", false},
		{"another provider selected", SettingsInfo{Provider: "mistral"}, "", false},
		{"already on codex", SettingsInfo{Provider: codexProviderID}, "", false},
		{"credential for another provider", SettingsInfo{Provider: openAIProviderID, CredentialProvider: "mistral"}, "", false},
		{"no selection at all", SettingsInfo{}, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := chatGPTRedirectCandidate(tc.settings, tc.apiKey); got != tc.want {
				t.Fatalf("chatGPTRedirectCandidate() = %v, want %v", got, tc.want)
			}
		})
	}
}
