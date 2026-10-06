package main

import (
	"context"
	"net/http"
	"testing"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/engineapi"
	"github.com/Dyu-36/gotack/internal/session"
	"github.com/Dyu-36/gotack/internal/workspace"
)

func TestListProvidersCapabilityOverrides(t *testing.T) {
	root := t.TempDir()
	t.Setenv("AppData", root)
	t.Setenv("XDG_CONFIG_HOME", root)
	yes, no := true, false
	configFor := func(model string, vision, reason *bool) *appconfig.Config {
		return &appconfig.Config{
			ModelCapabilities: map[string]appconfig.ModelCapabilityOverride{
				model: {SupportsVision: vision, CanReason: reason},
			},
		}
	}
	tests := []struct {
		name   string
		config *appconfig.Config
		vision bool
		reason bool
	}{
		{name: "nil configuration", vision: true},
		{name: "no override", config: &appconfig.Config{}, vision: true},
		{name: "disable vision", config: configFor("model", &no, nil)},
		{name: "enable reasoning", config: configFor("model", nil, &yes), vision: true, reason: true},
		{name: "empty override", config: configFor("model", nil, nil), vision: true},
		{name: "unrelated override", config: configFor("other", &no, &yes), vision: true},
		{name: "vision enable preserves text only model", config: configFor("text", &yes, nil), vision: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := engineapi.NewClient(&http.Client{Transport: catalogRoundTripper(func(req *http.Request) (*http.Response, error) {
				switch req.URL.Path {
				case "/v1/workspaces":
					return jsonHTTPResponse(http.StatusOK, `{"id":"catalog","path":"catalog"}`), nil
				case "/v1/workspaces/catalog/providers":
					return jsonHTTPResponse(http.StatusOK, `[{"id":"test","models":[
						{"id":"model","supports_vision":true},
						{"id":"text","supports_vision":false,"can_reason":true}
					]}]`), nil
				case "/v1/workspaces/catalog/config":
					return jsonHTTPResponse(http.StatusOK, `{}`), nil
				default:
					t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
					return jsonHTTPResponse(http.StatusNotFound, `{}`), nil
				}
			})})
			ws := workspace.NewService(api)
			app := NewApp()
			app.ctx = context.Background()
			app.cfg = tt.config
			app.swapConn(func(c *conn) *conn {
				c.API, c.Workspace, c.Session = api, ws, session.NewService(api, ws)
				return c
			})
			scope, started := app.host.Link.BeginConnect(app.ctx)
			if !started || !app.host.Link.CommitAttach(scope, engineapi.Endpoint{}, "test") {
				t.Fatal("failed to prepare catalog connection")
			}
			app.host.Link.MarkRunning()
			app.vision.Store(visionCacheKey{workspaceID: "old", providerID: "old", modelID: "old"}, true)
			providers, err := app.ListProviders()
			if err != nil {
				t.Fatal(err)
			}
			model := findProvider(t, providers, "test").Models[0]
			if model.SupportsVision != tt.vision || model.CanReason != tt.reason {
				t.Fatalf("capabilities = %t/%t, want %t/%t", model.SupportsVision, model.CanReason, tt.vision, tt.reason)
			}
			textModel := findProvider(t, providers, "test").Models[1]
			if textModel.SupportsVision || !textModel.CanReason {
				t.Fatalf("text model capabilities changed: %#v", textModel)
			}
			if _, ok := app.vision.Load(visionCacheKey{workspaceID: "old", providerID: "old", modelID: "old"}); ok {
				t.Fatal("catalog refresh retained a stale capability")
			}
			cached, ok := app.vision.Load(visionCacheKey{workspaceID: "catalog", providerID: "test", modelID: "model"})
			if !ok || cached != tt.vision {
				t.Fatalf("cached capability = %v/%t, want %t/true", cached, ok, tt.vision)
			}
		})
	}
}
