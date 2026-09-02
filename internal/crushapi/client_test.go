// client_test.go -- role: focused tests for HTTP transport behavior.
package crushapi

import (
	"compress/gzip"
	"context"
	"encoding/json"
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
			_, _ = zw.Write([]byte(`[{"id":"opencode-go","name":"OpenCode Go","models":[{"id":"text-model","name":"Text Model","supports_attachments":false},{"id":"vision-model","name":"Vision Model","supports_attachments":true}]}]`))

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
	if len(providers) != 1 || providers[0].ID != "opencode-go" || len(providers[0].Models) != 2 {
		t.Fatalf("ListProviders() = %#v", providers)
	}
	if providers[0].Models[0].SupportsVision {
		t.Fatal("text-model supports_attachments=false was promoted to vision")
	}
	if !providers[0].Models[1].SupportsVision {
		t.Fatal("supports_attachments=true was not decoded as vision support")
	}
}

func TestSetPermissionsSkipPostsWorkspaceFlag(t *testing.T) {
	var gotMethod, gotPath string
	var gotSkip bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		var payload struct {
			Skip bool `json:"skip"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode permission skip request: %v", err)
		}
		gotSkip = payload.Skip
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}
	client := NewClient(&http.Client{Transport: transport})
	if err := client.SetPermissionsSkip(context.Background(), "ws-1", true); err != nil {
		t.Fatalf("SetPermissionsSkip() error = %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/v1/workspaces/ws-1/permissions/skip" || !gotSkip {
		t.Fatalf("permission skip request = %s %s skip=%v", gotMethod, gotPath, gotSkip)
	}
}

func TestInitAgentPostsInteractiveFlag(t *testing.T) {
	var gotMethod, gotPath string
	var gotInteractive bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		var payload struct {
			Interactive bool `json:"interactive"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode agent init request: %v", err)
		}
		gotInteractive = payload.Interactive
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}
	client := NewClient(&http.Client{Transport: transport})
	if err := client.InitAgent(context.Background(), "ws-1", true); err != nil {
		t.Fatalf("InitAgent() error = %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/v1/workspaces/ws-1/agent/init" || !gotInteractive {
		t.Fatalf("agent init request = %s %s interactive=%v", gotMethod, gotPath, gotInteractive)
	}
}

func TestRefreshPromptContextPostsNarrowAgentRoute(t *testing.T) {
	var gotMethod, gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}
	client := NewClient(&http.Client{Transport: transport})
	if err := client.RefreshPromptContext(context.Background(), "ws-1"); err != nil {
		t.Fatalf("RefreshPromptContext() error = %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/v1/workspaces/ws-1/agent/refresh-prompt" {
		t.Fatalf("prompt refresh request = %s %s", gotMethod, gotPath)
	}
}

func TestEnsureAgentInitializesOnlyWhenNotReady(t *testing.T) {
	for _, tc := range []struct {
		name     string
		ready    bool
		wantInit int
	}{
		{name: "already ready", ready: true, wantInit: 0},
		{name: "not ready", ready: false, wantInit: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			getCount, initCount := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodGet && r.URL.Path == "/v1/workspaces/ws-1/agent":
					getCount++
					_ = json.NewEncoder(w).Encode(map[string]bool{"is_ready": tc.ready})
				case r.Method == http.MethodPost && r.URL.Path == "/v1/workspaces/ws-1/agent/init":
					initCount++
					var payload struct {
						Interactive bool `json:"interactive"`
					}
					if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
						t.Errorf("decode agent init request: %v", err)
					}
					if !payload.Interactive {
						t.Error("agent init should be interactive")
					}
					w.WriteHeader(http.StatusOK)
				default:
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
					http.Error(w, "unexpected request", http.StatusNotFound)
				}
			}))
			defer server.Close()

			transport := http.DefaultTransport.(*http.Transport).Clone()
			transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
			}
			client := NewClient(&http.Client{Transport: transport})
			if err := client.EnsureAgent(context.Background(), "ws-1", true); err != nil {
				t.Fatalf("EnsureAgent() error = %v", err)
			}
			if getCount != 1 || initCount != tc.wantInit {
				t.Fatalf("requests: get=%d init=%d, want get=1 init=%d", getCount, initCount, tc.wantInit)
			}
		})
	}
}

func TestSendPromptWithAttachmentsPostsAgentPayload(t *testing.T) {
	var got struct {
		SessionID   string       `json:"session_id"`
		RunID       string       `json:"run_id"`
		Prompt      string       `json:"prompt"`
		Attachments []Attachment `json:"attachments"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/workspaces/ws-1/agent" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected request", http.StatusNotFound)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode prompt request: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}
	client := NewClient(&http.Client{Transport: transport})
	attachments := []Attachment{{
		FilePath: "photo.png",
		FileName: "photo.png",
		MimeType: "image/png",
		Content:  []byte("png-data"),
	}}
	if err := client.SendPromptWithAttachments(context.Background(), "ws-1", "session-1", "describe", "run-1", attachments); err != nil {
		t.Fatalf("SendPromptWithAttachments() error = %v", err)
	}

	if got.SessionID != "session-1" || got.RunID != "run-1" || got.Prompt != "describe" {
		t.Fatalf("prompt metadata = %#v", got)
	}
	if len(got.Attachments) != 1 || got.Attachments[0].FileName != "photo.png" || string(got.Attachments[0].Content) != "png-data" {
		t.Fatalf("attachments = %#v", got.Attachments)
	}
}

func TestSendPromptWithAttachmentsAndBudgetPostsOnlyExplicitBudget(t *testing.T) {
	var got struct {
		SessionID      string `json:"session_id"`
		Prompt         string `json:"prompt"`
		MaxInputTokens int64  `json:"max_input_tokens"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode prompt request: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}
	client := NewClient(&http.Client{Transport: transport})
	if err := client.SendPromptWithAttachmentsAndBudget(context.Background(), "ws-1", "review-1", "review", "run-1", nil, 600_000); err != nil {
		t.Fatalf("SendPromptWithAttachmentsAndBudget() error = %v", err)
	}
	if got.SessionID != "review-1" || got.Prompt != "review" || got.MaxInputTokens != 600_000 {
		t.Fatalf("prompt payload = %#v", got)
	}
}
