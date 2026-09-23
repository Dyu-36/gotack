package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/engine"
	"github.com/Dyu-36/gotack/internal/engineapi"
	"github.com/Dyu-36/gotack/internal/session"
	"github.com/Dyu-36/gotack/internal/workspace"
)

func bridgeTestRuntime(t *testing.T) (*engineapi.Client, string) {
	t.Helper()
	binary := os.Getenv("GOTACK_TEST_ENGINE")
	if binary == "" {
		if os.Getenv("GOTACK_REQUIRE_ENGINE") == "1" {
			t.Fatal("GOTACK_TEST_ENGINE must identify the built pinned engine")
		}
		t.Skip("set GOTACK_TEST_ENGINE to run the real host/engine contract tests")
	}
	absolute, err := filepath.Abs(binary)
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(absolute); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("engine executable is unavailable: %s (%v)", absolute, err)
	}
	root := t.TempDir()
	for _, variable := range []string{"APPDATA", "LOCALAPPDATA", "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_RUNTIME_DIR"} {
		t.Setenv(variable, root)
	}
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	sup := engine.NewSupervisor(slog.Default(), absolute)
	if _, exists := sup.Locate(ctx); exists {
		t.Fatal("the test IPC endpoint is already occupied; refusing to use another engine")
	}
	ep, err := sup.Start()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := sup.Stop(); err != nil {
			t.Errorf("stop test engine: %v", err)
		}
	})
	hc, err := engineapi.Dial(ep)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(hc.CloseIdleConnections)
	api := engineapi.NewClient(hc)
	vi, err := engine.WaitForHealthy(ctx, api, 45*time.Second)
	if err != nil {
		t.Fatalf("engine handshake: %v", err)
	}
	if vi.Commit != strings.TrimSpace(packagedEngineCommit) {
		t.Fatalf("wrong engine: expected %s, got %s", strings.TrimSpace(packagedEngineCommit), vi.Commit)
	}
	t.Logf("verified engine commit=%s version=%s platform=%s", vi.Commit, vi.Version, vi.Platform)
	workspaceRoot := filepath.Join(root, "workspace")
	if err := os.MkdirAll(workspaceRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	return api, workspaceRoot
}

func TestBridgeSmoke(t *testing.T) {
	api, root := bridgeTestRuntime(t)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	ws, err := api.CreateWorkspace(ctx, root, true)
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	if ws.ID == "" {
		t.Fatal("missing workspace ID")
	}
	if _, err := api.ListProviders(ctx, ws.ID); err != nil {
		t.Fatalf("provider contract: %v", err)
	}
	sess, err := api.CreateSession(ctx, ws.ID, "smoke")
	if err != nil || sess.ID == "" {
		t.Fatalf("create session: %+v %v", sess, err)
	}
	msgs, err := api.Messages(ctx, ws.ID, sess.ID)
	if err != nil || len(msgs) != 0 {
		t.Fatalf("new session messages: %d, %v", len(msgs), err)
	}
	sess.Title = "renamed smoke"
	renamed, err := api.SaveSession(ctx, ws.ID, sess)
	if err != nil || renamed.Title != sess.Title {
		t.Fatalf("rename session: %+v %v", renamed, err)
	}
	loaded, err := api.GetSession(ctx, ws.ID, sess.ID)
	if err != nil || loaded.Title != sess.Title {
		t.Fatalf("session persistence: %+v %v", loaded, err)
	}
	events, stop, err := api.Stream(ctx, ws.ID)
	if err != nil {
		t.Fatalf("SSE contract: %v", err)
	}
	defer stop()
	select {
	case _, open := <-events:
		if !open {
			t.Fatal("SSE stream closed immediately")
		}
	case <-time.After(200 * time.Millisecond):
	}
	if err := api.DeleteSession(ctx, ws.ID, sess.ID); err != nil {
		t.Fatalf("delete session: %v", err)
	}
	sessions, err := api.ListSessions(ctx, ws.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, candidate := range sessions {
		if candidate.ID == sess.ID {
			t.Fatal("deleted session still exists")
		}
	}
}

func TestBridgeServicesSmoke(t *testing.T) {
	api, root := bridgeTestRuntime(t)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	wsSvc := workspace.NewService(api)
	desc, err := wsSvc.Open(ctx, root)
	if err != nil {
		t.Fatalf("workspace service: %v", err)
	}
	cur, ok := wsSvc.Current()
	if !ok || cur.WorkspaceID != desc.WorkspaceID {
		t.Fatalf("current workspace = %+v, available=%v", cur, ok)
	}
	sessSvc := session.NewService(api, wsSvc)
	sess, err := sessSvc.Create(ctx, "service-smoke")
	if err != nil {
		t.Fatalf("session service: %v", err)
	}
	if _, err := sessSvc.Messages(ctx, sess.ID); err != nil {
		t.Fatal(err)
	}
	skillPath := filepath.Join(root, "skills")
	if err := api.SetConfigField(ctx, desc.WorkspaceID, engineapi.ConfigScopeWorkspace, "options.skills_paths", []string{skillPath}); err != nil {
		t.Fatalf("config mutation contract: %v", err)
	}
	cfg, err := api.GetWorkspaceConfig(ctx, desc.WorkspaceID)
	if err != nil {
		t.Fatalf("config read contract: %v", err)
	}
	if !slices.Contains(cfg.SkillsPaths(), skillPath) {
		t.Fatalf("config read contract dropped requested skills path %q: %v", skillPath, cfg.SkillsPaths())
	}
	if len(appconfig.Defaults().RecentWorkspaces) != 0 {
		t.Fatal("workspace service mutated desktop defaults")
	}
}
