// client_test.go -- role: focused tests for HTTP transport behavior.
package crushapi

import (
	"compress/gzip"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientDecodesGzipWorkspaceAndProviders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			http.Error(w, "client did not advertise gzip", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Content-Type", "application/json")
		zw := gzip.NewWriter(w)
		defer zw.Close()

		switch {
		case r.Method == http.MethodPost && r.URL.Path == workspacesPath:
			_, _ = zw.Write([]byte(`{"id":"ws-1","path":"D:/repo"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/workspaces/ws-1/providers":
			_, _ = zw.Write([]byte(`[{"id":"anthropic","name":"Anthropic","models":[{"id":"claude","name":"Claude"}]}]`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = zw.Write([]byte(`{"message":"not found"}`))
		}
	}))
	defer server.Close()

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}
	client := NewClient(&http.Client{Transport: transport})

	workspace, err := client.CreateWorkspace(context.Background(), "D:/repo", false)
	if err != nil {
		t.Fatalf("CreateWorkspace() error = %v", err)
	}
	if workspace.ID != "ws-1" || workspace.Path != "D:/repo" {
		t.Fatalf("CreateWorkspace() = %#v", workspace)
	}

	providers, err := client.ListProviders(context.Background(), workspace.ID)
	if err != nil {
		t.Fatalf("ListProviders() error = %v", err)
	}
	if len(providers) != 1 || providers[0].ID != "anthropic" || len(providers[0].Models) != 1 {
		t.Fatalf("ListProviders() = %#v", providers)
	}
}
