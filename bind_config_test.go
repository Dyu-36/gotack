package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
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
	return f(req)
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
	if len(providers) != 1 || providers[0].ID != "anthropic" || len(providers[0].Models) != 1 {
		t.Fatalf("ListProviders() = %#v", providers)
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
