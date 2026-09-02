package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/crushapi"
	"github.com/Dyu-36/gotack/internal/session"
	"github.com/Dyu-36/gotack/internal/workspace"
)

type catalogRoundTripper func(*http.Request) (*http.Response, error)

func (f catalogRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := f(req)
	if resp != nil && resp.Request == nil {
		resp.Request = req
	}
	return resp, err
}

func TestListProvidersWithoutCurrentWorkspace(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("AppData", configRoot)
	t.Setenv("XDG_CONFIG_HOME", configRoot)

	var createdPath string
	transport := catalogRoundTripper(func(req *http.Request) (*http.Response, error) {
		var body string
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/v1/workspaces":
			var payload struct {
				Path string `json:"path"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Errorf("decode workspace request: %v", err)
			}
			createdPath = payload.Path
			body = `{"id":"catalog-ws","path":"` + filepath.ToSlash(payload.Path) + `"}`
		case req.Method == http.MethodGet && req.URL.Path == "/v1/workspaces/catalog-ws/providers":
			body = `[{"id":"anthropic","name":"Anthropic","models":[{"id":"claude","name":"Claude"}]}]`
		case req.Method == http.MethodGet && req.URL.Path == "/v1/workspaces/catalog-ws/config":
			body = `{"providers":{"anthropic":{"id":"anthropic","name":"Anthropic","api_key":"secret"}}}`
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
	// Drive the link through the same transitions connect() uses so the
	// services above count as a live engine connection.
	scope, started := app.link.BeginConnect(context.Background())
	if !started {
		t.Fatal("fresh link must accept a connect attempt")
	}
	if !app.link.CommitAttach(scope, crushapi.Endpoint{}, "test") {
		t.Fatal("commit attach rejected a live scope")
	}
	app.link.MarkRunning()

	providers, err := app.ListProviders()
	if err != nil {
		t.Fatalf("ListProviders() error = %v", err)
	}
	if len(providers) != 3 || providers[0].ID != "anthropic" || len(providers[0].Models) != 1 {
		t.Fatalf("ListProviders() = %#v", providers)
	}
	mistral := findProvider(t, providers, mistralProviderID)
	if mistral.APIEndpoint != mistralDefaultEndpoint || len(mistral.Models) == 0 {
		t.Fatalf("Mistral overlay = %#v", mistral)
	}
	// Codex is listed without a credential so Settings can start the ChatGPT
	// sign-in from the provider list.
	codex := findProvider(t, providers, codexProviderID)
	if codex.APIEndpoint != codexBackendURL || codex.CredentialKind != "" {
		t.Fatalf("Codex overlay = %#v", codex)
	}
	wantPath := filepath.Join(appconfig.Dir(), "catalog-workspace")
	if createdPath != wantPath {
		t.Fatalf("catalog workspace path = %q, want %q", createdPath, wantPath)
	}
	if _, ok := ws.Current(); ok {
		t.Fatal("ListProviders() changed the current workspace")
	}
}

func jsonHTTPResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestResolvedProviderCredentialRejectsUnsetEnvTemplate(t *testing.T) {
	t.Setenv("MINIMAX_API_KEY", "")
	kind, value, ok := resolvedProviderCredential(crushapi.ProviderConfig{APIKey: "$MINIMAX_API_KEY"})
	if ok || kind != "" || value != "" {
		t.Fatalf("unset env template reported usable: kind=%q value=%q ok=%v", kind, value, ok)
	}
}

func TestResolvedProviderCredentialResolvesEnvTemplate(t *testing.T) {
	t.Setenv("MINIMAX_API_KEY", "minimax-secret")
	kind, value, ok := resolvedProviderCredential(crushapi.ProviderConfig{APIKey: "$MINIMAX_API_KEY"})
	if !ok || kind != "api_key" || value != "minimax-secret" {
		t.Fatalf("resolved env credential = kind=%q value=%q ok=%v", kind, value, ok)
	}
}

func TestResolvedProviderCredentialAcceptsLiteralKey(t *testing.T) {
	kind, value, ok := resolvedProviderCredential(crushapi.ProviderConfig{APIKey: "literal-secret"})
	if !ok || kind != "api_key" || value != "literal-secret" {
		t.Fatalf("literal credential = kind=%q value=%q ok=%v", kind, value, ok)
	}
}

func TestCrushReasoningPreservesMax(t *testing.T) {
	effort, think := crushReasoning("max")
	if effort != "max" || !think {
		t.Fatalf("crushReasoning(max) = effort=%q think=%v", effort, think)
	}
}

func TestDeleteProviderIsSafetyFirstAndRetryableAtEveryFailureBoundary(t *testing.T) {
	cases := []struct {
		name          string
		fail          string
		wantMutations []string
	}{
		{"config read", "read", nil},
		{"local save", "save", nil},
		{"disable", "disable", []string{"disable"}},
		{"model clear", "models", []string{"disable", "models"}},
		{"api key removal", "api_key", []string{"disable", "models", "api_key"}},
		{"oauth removal", "oauth", []string{"disable", "models", "api_key", "oauth"}},
		{"success", "", []string{"disable", "models", "api_key", "oauth"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			configBase := t.TempDir()
			workspacePath := t.TempDir()
			t.Setenv("AppData", configBase)
			t.Setenv("XDG_CONFIG_HOME", configBase)

			state := struct {
				fail          string
				mutations     []string
				disabled      bool
				modelsPresent bool
				apiKeyPresent bool
				oauthPresent  bool
			}{
				fail:          tc.fail,
				modelsPresent: true,
				apiKeyPresent: true,
				oauthPresent:  true,
			}
			failedResponse := func() *http.Response {
				return jsonHTTPResponse(http.StatusInternalServerError, `{"error":"injected failure"}`)
			}
			transport := catalogRoundTripper(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/v1/workspaces":
					return jsonHTTPResponse(http.StatusOK, `[]`), nil
				case req.Method == http.MethodPost && req.URL.Path == "/v1/workspaces":
					return jsonHTTPResponse(http.StatusOK, `{"id":"ws-1","path":"`+filepath.ToSlash(workspacePath)+`"}`), nil
				case req.Method == http.MethodGet && req.URL.Path == "/v1/workspaces/ws-1/config":
					if state.fail == "read" {
						return failedResponse(), nil
					}
					if state.modelsPresent {
						return jsonHTTPResponse(http.StatusOK, `{"providers":{"anthropic":{"api_key":"secret","oauth":{"access_token":"token"}}},"models":{"large":{"provider":"anthropic","model":"claude"},"small":{"provider":"anthropic","model":"claude"}}}`), nil
					}
					return jsonHTTPResponse(http.StatusOK, `{"providers":{"anthropic":{"disable":true}}}`), nil
				case req.Method == http.MethodPost && req.URL.Path == "/v1/workspaces/ws-1/config/set":
					var payload struct {
						Key   string `json:"key"`
						Value bool   `json:"value"`
					}
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("decode disable request: %v", err)
					}
					if payload.Key != "providers.anthropic.disable" || !payload.Value {
						t.Fatalf("disable payload = %#v", payload)
					}
					state.mutations = append(state.mutations, "disable")
					if state.fail == "disable" {
						return failedResponse(), nil
					}
					state.disabled = true
					return jsonHTTPResponse(http.StatusOK, `{}`), nil
				case req.Method == http.MethodPost && req.URL.Path == "/v1/workspaces/ws-1/config/models":
					var payload struct {
						Models map[string]*json.RawMessage `json:"models"`
					}
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("decode model removal: %v", err)
					}
					if len(payload.Models) != 2 || payload.Models["large"] != nil || payload.Models["small"] != nil {
						t.Fatalf("model removal payload = %#v", payload.Models)
					}
					state.mutations = append(state.mutations, "models")
					if state.fail == "models" {
						return failedResponse(), nil
					}
					state.modelsPresent = false
					return jsonHTTPResponse(http.StatusOK, `{}`), nil
				case req.Method == http.MethodPost && req.URL.Path == "/v1/workspaces/ws-1/config/remove":
					var payload struct {
						Key string `json:"key"`
					}
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("decode config removal: %v", err)
					}
					var step string
					switch payload.Key {
					case "providers.anthropic.api_key":
						step = "api_key"
					case "providers.anthropic.oauth":
						step = "oauth"
					default:
						t.Fatalf("unexpected removal key %q", payload.Key)
					}
					state.mutations = append(state.mutations, step)
					if state.fail == step {
						return failedResponse(), nil
					}
					if step == "api_key" {
						state.apiKeyPresent = false
					} else {
						state.oauthPresent = false
					}
					return jsonHTTPResponse(http.StatusOK, `{}`), nil
				default:
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
					return nil, nil
				}
			})

			api := crushapi.NewClient(&http.Client{Transport: transport})
			ws := workspace.NewService(api)
			app := NewApp()
			app.ctx = context.Background()
			app.cfg = &appconfig.Config{Provider: "anthropic", Model: "claude", CustomURL: "https://api.example"}
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

			blockedConfigPath := filepath.Join(configBase, "gotack")
			if tc.fail == "save" {
				if err := os.WriteFile(blockedConfigPath, []byte("not a directory"), 0o600); err != nil {
					t.Fatalf("create config blocker: %v", err)
				}
			}

			err := app.DeleteProvider("anthropic")
			if tc.fail == "" {
				if err != nil {
					t.Fatalf("DeleteProvider() error = %v", err)
				}
			} else if err == nil {
				t.Fatal("DeleteProvider() succeeded despite injected failure")
			}
			if got, want := strings.Join(state.mutations, ","), strings.Join(tc.wantMutations, ","); got != want {
				t.Fatalf("first-attempt mutations = %q, want %q", got, want)
			}

			if tc.fail != "" {
				if tc.fail == "read" || tc.fail == "save" {
					if app.cfg.Provider != "anthropic" || app.cfg.Model != "claude" {
						t.Fatalf("local selection changed before safe persistence: %#v", app.cfg)
					}
				} else if app.cfg.Provider != "" || app.cfg.Model != "" || app.cfg.CustomURL != "" {
					t.Fatalf("local selection was not made safe: %#v", app.cfg)
				}
				if tc.fail == "save" {
					if err := os.Remove(blockedConfigPath); err != nil {
						t.Fatalf("remove config blocker: %v", err)
					}
				}
				state.fail = ""
				if err := app.DeleteProvider("anthropic"); err != nil {
					t.Fatalf("retry DeleteProvider() error = %v", err)
				}
			}

			if app.cfg.Provider != "" || app.cfg.Model != "" || app.cfg.CustomURL != "" {
				t.Fatalf("final local selection = %#v", app.cfg)
			}
			if !state.disabled || state.modelsPresent || state.apiKeyPresent || state.oauthPresent {
				t.Fatalf("final engine state = disabled:%v models:%v api-key:%v oauth:%v", state.disabled, state.modelsPresent, state.apiKeyPresent, state.oauthPresent)
			}
			persisted, err := appconfig.Load()
			if err != nil {
				t.Fatalf("load persisted config: %v", err)
			}
			if persisted.Provider != "" || persisted.Model != "" || persisted.CustomURL != "" {
				t.Fatalf("persisted selection = %#v", persisted)
			}
		})
	}
}
