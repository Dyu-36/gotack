package main

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"

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
			body = `{"providers":{"openai":{"id":"openai","name":"OpenAI","oauth":{"access_token":"valid-token","account_email":"user@test.local","chatgpt_plan_type":"plus"}}}}`
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
			body = `{"providers":{"openai":{"id":"openai","name":"OpenAI","disable":true}}}`
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
