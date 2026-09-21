package workspace

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/Dyu-36/gotack/internal/engineapi"
)

func TestOpenCreatesWorkspaceWithPermissionsSkipped(t *testing.T) {
	dir := t.TempDir()
	var gotPath string
	var gotYOLO bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/workspaces":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[]`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/workspaces":
			var payload struct {
				Path string `json:"path"`
				YOLO bool   `json:"yolo"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("decode workspace request: %v", err)
			}
			gotPath, gotYOLO = payload.Path, payload.YOLO
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"id": "ws-1", "path": payload.Path})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}
	api := engineapi.NewClient(&http.Client{Transport: transport})
	service := NewService(api)
	if _, err := service.Open(context.Background(), dir); err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if !gotYOLO {
		t.Fatal("CreateWorkspace() did not enable YOLO/permission-skip mode")
	}
	if filepath.Clean(gotPath) != filepath.Clean(dir) {
		t.Fatalf("CreateWorkspace() path = %q, want %q", gotPath, dir)
	}
}

func TestOpenWithDataDirPassesPersistenceDirectory(t *testing.T) {
	workspaceDir := t.TempDir()
	dataDir := t.TempDir()
	var gotDataDir string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/workspaces":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[]`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/workspaces":
			var payload struct {
				Path    string `json:"path"`
				DataDir string `json:"data_dir"`
				YOLO    bool   `json:"yolo"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("decode workspace request: %v", err)
			}
			gotDataDir = payload.DataDir
			if !payload.YOLO {
				t.Error("OpenWithDataDir did not enable YOLO")
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"id": "ws-data", "path": payload.Path})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}
	api := engineapi.NewClient(&http.Client{Transport: transport})
	service := NewService(api)
	if _, err := service.OpenWithDataDir(context.Background(), workspaceDir, dataDir); err != nil {
		t.Fatalf("OpenWithDataDir() error = %v", err)
	}
	if gotDataDir != dataDir {
		t.Fatalf("data_dir = %q, want %q", gotDataDir, dataDir)
	}
}
