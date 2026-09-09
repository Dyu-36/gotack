package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/engineapi"
	"github.com/Dyu-36/gotack/internal/workspace"
)

func TestDefaultWorkspacePath(t *testing.T) {
	got := filepath.Clean(defaultWorkspacePath())
	if runtime.GOOS == "windows" {
		if got != filepath.Clean(`C:\`) {
			t.Fatalf("defaultWorkspacePath() = %q, want C:\\", got)
		}
	} else if got != filepath.Clean(string(filepath.Separator)) {
		t.Fatalf("defaultWorkspacePath() = %q, want filesystem root", got)
	}
	if !isDefaultWorkspace(defaultWorkspacePath()) {
		t.Fatal("default workspace path is not recognized as default")
	}
}

func TestWorkspaceActivationAlwaysSkipsPermissions(t *testing.T) {
	for _, tc := range []struct {
		name string
		data string
	}{
		{name: "nil config"},
		{name: "default config", data: `{}`},
		{name: "legacy false", data: `{"auto_approve":false}`},
		{name: "legacy true", data: `{"auto_approve":true}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var cfg *appconfig.Config
			if tc.data != "" {
				cfg = appconfig.Defaults()
				if err := json.Unmarshal([]byte(tc.data), cfg); err != nil {
					t.Fatal(err)
				}
			}
			for _, assistant := range []bool{false, true} {
				name := "workspace"
				if assistant {
					name = "assistant"
				}
				t.Run(name, func(t *testing.T) {
					dir := t.TempDir()
					if assistant {
						dir = defaultWorkspacePath()
					}
					var created bool
					var skipRequests []bool
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("Content-Type", "application/json")
						switch {
						case r.Method == http.MethodGet && r.URL.Path == "/v1/workspaces":
							items := []engineapi.Workspace{}
							if created {
								items = append(items, engineapi.Workspace{ID: "ws-1", Path: dir})
							}
							_ = json.NewEncoder(w).Encode(items)
						case r.Method == http.MethodPost && r.URL.Path == "/v1/workspaces":
							var payload struct {
								Path string `json:"path"`
								YOLO bool   `json:"yolo"`
							}
							if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
								t.Errorf("decode workspace request: %v", err)
							}
							if !payload.YOLO {
								t.Error("new workspace did not enable YOLO")
							}
							created = true
							_ = json.NewEncoder(w).Encode(engineapi.Workspace{ID: "ws-1", Path: payload.Path})
						case r.Method == http.MethodPost && r.URL.Path == "/v1/workspaces/ws-1/permissions/skip":
							var payload struct {
								Skip bool `json:"skip"`
							}
							if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
								t.Errorf("decode permission request: %v", err)
							}
							skipRequests = append(skipRequests, payload.Skip)
							w.WriteHeader(http.StatusNoContent)
						default:
							t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
							http.NotFound(w, r)
						}
					}))
					defer server.Close()
					transport := http.DefaultTransport.(*http.Transport).Clone()
					transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
						return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
					}
					defer transport.CloseIdleConnections()
					api := engineapi.NewClient(&http.Client{Transport: transport})
					svc := &bridgeServices{api: api, ws: workspace.NewService(api)}
					app := &App{ctx: context.Background(), cfg: cfg}
					for attempt := 0; attempt < 2; attempt++ {
						var info WorkspaceInfo
						var err error
						if assistant {
							info, err = app.activateAssistantWorkspace(svc)
						} else {
							info, err = app.activateWorkspace(svc, dir, false)
						}
						if err != nil {
							t.Fatalf("activate workspace: %v", err)
						}
						if info.WorkspaceID != "ws-1" || info.IsDefault != assistant {
							t.Fatalf("unexpected workspace: %+v", info)
						}
						if len(skipRequests) != attempt+1 || !skipRequests[attempt] {
							t.Fatalf("activation %d did not enable permission skipping: %v", attempt, skipRequests)
						}
					}
				})
			}
		})
	}
}
