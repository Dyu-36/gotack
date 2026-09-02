package main

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/crushapi"
	"github.com/Dyu-36/gotack/internal/session"
	"github.com/Dyu-36/gotack/internal/workspace"
)

func TestApplyEffectiveCrushSettingsRedirectsStaleSelection(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("AppData", configRoot)
	t.Setenv("XDG_CONFIG_HOME", configRoot)

	const migratedConfig = `{"providers":{"codex":{"id":"codex","name":"ChatGPT (Codex)","base_url":"https://chatgpt.com/backend-api/codex","models":[{"id":"gpt-subscription"},{"id":"gpt-other"}],"oauth":{"access_token":"tok","account_id":"acc-123"}},"openai":{"id":"openai","disable":true}}}`

	var (
		modelRequests int
		modelWrites   []string
	)
	transport := catalogRoundTripper(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/workspaces":
			return jsonHTTPResponse(http.StatusOK, `[]`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/workspaces":
			return jsonHTTPResponse(http.StatusOK, `{"id":"ws-1","path":"`+filepath.ToSlash(configRoot)+`"}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/workspaces/ws-1/config":
			return jsonHTTPResponse(http.StatusOK, migratedConfig), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/workspaces/ws-1/providers":

			return jsonHTTPResponse(http.StatusOK, `[{"id":"openai","name":"OpenAI"}]`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/workspaces/ws-1/config/models":
			var payload struct {
				Models map[string]struct {
					Provider string `json:"provider"`
					Model    string `json:"model"`
				} `json:"models"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode preferred models: %v", err)
			}
			modelRequests++
			for _, modelType := range []string{"large", "small"} {
				model := payload.Models[modelType]
				modelWrites = append(modelWrites, modelType+":"+model.Provider+"/"+model.Model)
			}
			return jsonHTTPResponse(http.StatusOK, `{}`), nil
		default:

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

	if effective.CustomURL != "" {
		t.Fatalf("effective endpoint = %q, want it cleared", effective.CustomURL)
	}
	wantModels := []string{"large:" + codexProviderID + "/gpt-subscription", "small:" + codexProviderID + "/gpt-subscription"}
	if modelRequests != 1 || !slices.Equal(modelWrites, wantModels) {
		t.Fatalf("model requests = %d, writes = %#v, want %#v", modelRequests, modelWrites, wantModels)
	}
}

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

func TestApplyCrushSettingsValidatesBeforeMutation(t *testing.T) {
	cases := []struct {
		name     string
		settings SettingsInfo
		apiKey   string
	}{
		{"Codex rejects API keys", SettingsInfo{Provider: codexProviderID}, "sk-test"},
		{"OAuth rejects custom endpoints", SettingsInfo{Provider: codexProviderID, CustomURL: "https://example.invalid"}, ""},
		{"config paths reject unsafe provider ids", SettingsInfo{Provider: "bad.id", ProviderOnly: true}, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			workspacePath := t.TempDir()
			var mutationPhaseRequests []string
			opened := false
			transport := catalogRoundTripper(func(req *http.Request) (*http.Response, error) {
				if opened {
					mutationPhaseRequests = append(mutationPhaseRequests, req.Method+" "+req.URL.Path)
				}
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/v1/workspaces":
					return jsonHTTPResponse(http.StatusOK, `[]`), nil
				case req.Method == http.MethodPost && req.URL.Path == "/v1/workspaces":
					return jsonHTTPResponse(http.StatusOK, `{"id":"ws-1","path":"`+filepath.ToSlash(workspacePath)+`"}`), nil
				default:
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
			if !started || !app.link.CommitAttach(scope, crushapi.Endpoint{}, "test") {
				t.Fatal("could not attach test engine")
			}
			app.link.MarkRunning()
			if _, err := ws.Open(context.Background(), workspacePath); err != nil {
				t.Fatalf("open workspace: %v", err)
			}
			opened = true

			if err := app.applyCrushSettings(tc.settings, tc.apiKey); err == nil {
				t.Fatal("applyCrushSettings() succeeded, want validation error")
			}
			if len(mutationPhaseRequests) != 0 {
				t.Fatalf("validation performed engine requests: %#v", mutationPhaseRequests)
			}
		})
	}
}

func TestApplyCrushSettingsMakesProviderReadyBeforeModelSelection(t *testing.T) {
	cases := []struct {
		name          string
		settings      SettingsInfo
		apiKey        string
		fail          string
		wantMutations []string
	}{
		{
			name:          "credential failure",
			settings:      SettingsInfo{Provider: "anthropic", Model: "claude"},
			apiKey:        "sk-test",
			fail:          "credential",
			wantMutations: []string{"credential"},
		},
		{
			name:          "endpoint failure",
			settings:      SettingsInfo{Provider: "anthropic", Model: "claude", CustomURL: "https://api.example/v1"},
			fail:          "endpoint",
			wantMutations: []string{"endpoint"},
		},
		{
			name:          "discovery finalization failure",
			settings:      SettingsInfo{Provider: mistralProviderID, Model: "mistral-medium-3-5"},
			apiKey:        "mistral-key",
			fail:          "finalize",
			wantMutations: []string{"seed", "credential", "finalize"},
		},
		{
			name:          "ready provider selected last",
			settings:      SettingsInfo{Provider: "anthropic", Model: "claude", CustomURL: "https://api.example/v1"},
			apiKey:        "sk-test",
			wantMutations: []string{"credential", "endpoint", "models"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			workspacePath := t.TempDir()
			var (
				mutations           []string
				localProviderSeeded bool
			)
			fail := tc.fail
			transport := catalogRoundTripper(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/v1/workspaces":
					return jsonHTTPResponse(http.StatusOK, `[]`), nil
				case req.Method == http.MethodPost && req.URL.Path == "/v1/workspaces":
					return jsonHTTPResponse(http.StatusOK, `{"id":"ws-1","path":"`+filepath.ToSlash(workspacePath)+`"}`), nil
				case req.Method == http.MethodGet && req.URL.Path == "/v1/workspaces/ws-1/providers":
					return jsonHTTPResponse(http.StatusOK, `[{"id":"anthropic","name":"Anthropic"}]`), nil
				case req.Method == http.MethodGet && req.URL.Path == "/v1/workspaces/ws-1/config":
					if localProviderSeeded {
						return jsonHTTPResponse(http.StatusOK, `{"providers":{"anthropic":{"api_key":"existing"},"mistral":{"name":"Mistral AI","type":"openai-compat","base_url":"https://api.mistral.ai/v1"}}}`), nil
					}
					return jsonHTTPResponse(http.StatusOK, `{"providers":{"anthropic":{"api_key":"existing"}}}`), nil
				case req.Method == http.MethodPost && req.URL.Path == "/v1/workspaces/ws-1/config/set-batch":
					mutations = append(mutations, "seed")
					localProviderSeeded = true
					return jsonHTTPResponse(http.StatusOK, `{}`), nil
				case req.Method == http.MethodPost && req.URL.Path == "/v1/workspaces/ws-1/config/provider-key":
					mutations = append(mutations, "credential")
					if fail == "credential" {
						return jsonHTTPResponse(http.StatusInternalServerError, `{"error":"credential failure"}`), nil
					}
					return jsonHTTPResponse(http.StatusOK, `{}`), nil
				case req.Method == http.MethodPost && req.URL.Path == "/v1/workspaces/ws-1/config/set":
					var payload struct {
						Key string `json:"key"`
					}
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("decode config mutation: %v", err)
					}
					step := "endpoint"
					if strings.HasSuffix(payload.Key, ".discover_models") {
						step = "finalize"
					}
					mutations = append(mutations, step)
					if fail == step {
						return jsonHTTPResponse(http.StatusInternalServerError, `{"error":"config failure"}`), nil
					}
					return jsonHTTPResponse(http.StatusOK, `{}`), nil
				case req.Method == http.MethodPost && req.URL.Path == "/v1/workspaces/ws-1/config/models":
					mutations = append(mutations, "models")
					return jsonHTTPResponse(http.StatusOK, `{}`), nil
				case req.Method == http.MethodGet && req.URL.Path == "/v1/workspaces/ws-1/agent":
					return jsonHTTPResponse(http.StatusOK, `{"is_ready":true}`), nil
				default:
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
					return nil, nil
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
			if !started || !app.link.CommitAttach(scope, crushapi.Endpoint{}, "test") {
				t.Fatal("could not attach test engine")
			}
			app.link.MarkRunning()
			if _, err := ws.Open(context.Background(), workspacePath); err != nil {
				t.Fatalf("open workspace: %v", err)
			}

			err := app.applyCrushSettings(tc.settings, tc.apiKey)
			if tc.fail == "" {
				if err != nil {
					t.Fatalf("applyCrushSettings() error = %v", err)
				}
			} else if err == nil {
				t.Fatal("applyCrushSettings() succeeded despite injected failure")
			}
			if !slices.Equal(mutations, tc.wantMutations) {
				t.Fatalf("mutations = %#v, want %#v", mutations, tc.wantMutations)
			}
			if tc.fail == "finalize" {
				fail = ""
				mutations = nil
				if err := app.applyCrushSettings(tc.settings, tc.apiKey); err != nil {
					t.Fatalf("retry applyCrushSettings() error = %v", err)
				}
				wantRetry := []string{"credential", "finalize", "models"}
				if !slices.Equal(mutations, wantRetry) {
					t.Fatalf("retry mutations = %#v, want %#v", mutations, wantRetry)
				}
			}
		})
	}
}
