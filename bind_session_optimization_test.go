package main

import (
	"context"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"unicode/utf8"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/engineapi"
	"github.com/Dyu-36/gotack/internal/workspace"
)

func TestToolInputPreviewTruncatesWithoutBreakingUTF8(t *testing.T) {
	exact := strings.Repeat("x", maxToolInputPreview)
	if got := toolInputPreview(exact); got != exact {
		t.Fatalf("exact preview changed: len=%d", len(got))
	}

	long := strings.Repeat("界", maxToolInputPreview+1)
	got := toolInputPreview(long)
	if !utf8.ValidString(got) || !strings.HasSuffix(got, "…") {
		t.Fatalf("preview is not valid truncated UTF-8: %q", got[len(got)-12:])
	}
	if count := utf8.RuneCountInString(strings.TrimSuffix(got, "…")); count != maxToolInputPreview {
		t.Fatalf("preview runes = %d, want %d", count, maxToolInputPreview)
	}
}

func TestCurrentModelVisionCachesCatalogLookup(t *testing.T) {
	workspacePath := t.TempDir()
	var providerCalls atomic.Int32
	transport := catalogRoundTripper(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/workspaces":
			return jsonHTTPResponse(http.StatusOK, `[{"id":"ws-1","path":`+strconvQuote(workspacePath)+`}]`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/workspaces/ws-1/providers":
			providerCalls.Add(1)
			return jsonHTTPResponse(http.StatusOK, `[{"id":"openai","models":[{"id":"vision-model","supports_attachments":true}]}]`), nil
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	api := engineapi.NewClient(&http.Client{Transport: transport})
	ws := workspace.NewService(api)
	if _, err := ws.Open(context.Background(), workspacePath); err != nil {
		t.Fatalf("open workspace: %v", err)
	}
	a := NewApp()
	a.ctx = context.Background()
	a.cfg = &appconfig.Config{Provider: "openai", Model: "vision-model"}
	svc := &bridgeServices{api: api, ws: ws}
	if !a.isCurrentModelVision(svc) || !a.isCurrentModelVision(svc) {
		t.Fatal("vision capability was not preserved")
	}
	if got := providerCalls.Load(); got != 1 {
		t.Fatalf("provider catalog calls = %d, want 1", got)
	}
	a.vision.Clear()
	if !a.isCurrentModelVision(svc) {
		t.Fatal("vision capability was not reloaded after cache clear")
	}
	if got := providerCalls.Load(); got != 2 {
		t.Fatalf("provider catalog calls after clear = %d, want 2", got)
	}
}
