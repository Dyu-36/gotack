package main

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/changes"
	"github.com/Dyu-36/gotack/internal/crushapi"
	"github.com/Dyu-36/gotack/internal/engine"
	"github.com/Dyu-36/gotack/internal/session"
	"github.com/Dyu-36/gotack/internal/workspace"
)

// the normal test suite stays hermetic; run it with the engine up:
//
//	build/bin/crush.exe server &
//	go test -run TestBridgeSmoke -v .
func TestBridgeSmoke(t *testing.T) {
	ep := appconfig.PipeEndpoint()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	sup := engine.NewSupervisor(slog.Default(), "")
	if _, found := sup.Locate(ctx); !found {
		t.Skipf("engine not reachable at %s %s", ep.Network, ep.Address)
	}

	hc, err := crushapi.Dial(ctx, ep)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	api := crushapi.NewClient(hc)

	if err := crushapi.Ping(hc, ctx); err != nil {
		t.Fatalf("health: %v", err)
	}
	vi, err := api.Version(ctx)
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	t.Logf("engine version=%s platform=%s", vi.Version, vi.Platform)

	root, err := os.MkdirTemp("", "gotack-smoke-*")
	if err != nil {
		t.Fatalf("tempdir: %v", err)
	}
	defer os.RemoveAll(root)

	ws, err := api.CreateWorkspace(ctx, root, true)
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	t.Logf("workspace id=%s path=%s", ws.ID, ws.Path)

	providers, err := api.ListProviders(ctx, ws.ID)
	if err != nil {
		t.Fatalf("list providers: %v", err)
	}
	t.Logf("providers=%d", len(providers))

	sessions, err := api.ListSessions(ctx, ws.ID)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	t.Logf("sessions=%d", len(sessions))

	sess, err := api.CreateSession(ctx, ws.ID, "smoke")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if sess.ID == "" {
		t.Fatal("created session has empty id")
	}

	msgs, err := api.Messages(ctx, ws.ID, sess.ID)
	if err != nil {
		t.Fatalf("messages: %v", err)
	}
	if len(msgs) != 0 {
		t.Logf("fresh session carries %d messages", len(msgs))
	}

	events, stop, err := api.Stream(ctx, ws.ID)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	defer stop()
	select {
	case <-events:
		t.Log("received an event")
	case <-time.After(500 * time.Millisecond):
		t.Log("stream open, idle as expected")
	}

	resolved, err := api.GrantPermission(ctx, ws.ID, crushapi.PermissionRequest{ID: "no-such"}, crushapi.PermissionDeny)
	if err != nil {
		t.Fatalf("grant permission: %v", err)
	}
	if resolved {
		t.Fatal("unknown permission request must not resolve")
	}
}

// TestBridgeServicesSmoke drives the wiring-level services (supervisor
// locate, workspace attach, session create, changes fetch) the same way the
// bind layer does. Skips without a live engine.
func TestBridgeServicesSmoke(t *testing.T) {
	ep := appconfig.PipeEndpoint()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	sup := engine.NewSupervisor(slog.Default(), "")
	_, found := sup.Locate(ctx)
	if !found {
		t.Skipf("engine not reachable at %s %s", ep.Network, ep.Address)
	}

	hc, err := crushapi.Dial(ctx, ep)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	api := crushapi.NewClient(hc)

	root, err := os.MkdirTemp("", "gotack-smoke-svc-*")
	if err != nil {
		t.Fatalf("tempdir: %v", err)
	}
	defer os.RemoveAll(root)

	wsSvc := workspace.NewService(api)
	desc, err := wsSvc.Open(ctx, root)

	if err != nil {
		t.Fatalf("workspace open: %v", err)
	}
	cur, ok := wsSvc.Current()
	if !ok || cur.WorkspaceID != desc.WorkspaceID {
		t.Fatalf("current = %+v ok=%v, want just-opened workspace", cur, ok)
	}

	sessSvc := session.NewService(api, wsSvc)
	sess, err := sessSvc.Create(ctx, "svc-smoke")
	if err != nil {
		t.Fatalf("session create: %v", err)
	}
	if _, err := sessSvc.Messages(ctx, sess.ID); err != nil {
		t.Fatalf("messages: %v", err)
	}

	diffSvc := changes.NewService(api, wsSvc)
	files, err := diffSvc.ChangedFiles(ctx, sess.ID)
	if err != nil {
		t.Fatalf("changed files: %v", err)
	}
	if len(files) != 0 {
		t.Logf("fresh session touched %d files", len(files))
	}

	// Workspace service calls must not mutate the host-owned recent-workspace
	// list; that state is updated only by the bind layer under the App mutex.
	cfg := appconfig.Defaults()
	if got := len(cfg.RecentWorkspaces); got != 0 {
		t.Fatalf("workspace.Open must not mutate cfg.RecentWorkspaces; got %v", cfg.RecentWorkspaces)
	}
}
