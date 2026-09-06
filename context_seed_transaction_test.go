package main

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/Dyu-36/gotack/internal/contextseed"
	"github.com/Dyu-36/gotack/internal/crushapi"
	"github.com/Dyu-36/gotack/internal/session"
	"github.com/Dyu-36/gotack/internal/workspace"
)

func newContextLeaseTestApp(t *testing.T, seeder *contextseed.Seeder, fake *contextRegistrationAPI) *App {
	t.Helper()
	api := crushapi.NewClient(&http.Client{Transport: fake})
	app := NewApp()
	t.Cleanup(app.releaseAllContextLeases)
	app.ctx = context.Background()
	app.contextSeeder = seeder
	app.swapConn(func(c *conn) *conn {
		c.api = api
		c.ws = workspace.NewService(api)
		c.sess = session.NewService(api, c.ws)
		return c
	})
	scope, started := app.link.BeginConnect(context.Background())
	if !started || !app.link.CommitAttach(scope, crushapi.Endpoint{}, "test") {
		t.Fatal("link rejected test connect scope")
	}
	app.link.MarkRunning()
	return app
}

func TestRegisterContextPathsRefreshFailureRollsBackAcknowledgedConfig(t *testing.T) {
	dataDir := t.TempDir()
	seeder := contextseed.New(dataDir, nil)
	if err := os.MkdirAll(seeder.ContextDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	core := filepath.Join(seeder.ContextDir(), "TACK_CORE.md")
	if err := os.WriteFile(core, []byte("gen one"), 0o644); err != nil {
		t.Fatal(err)
	}
	fake := &contextRegistrationAPI{t: t}
	app := newContextLeaseTestApp(t, seeder, fake)
	app.registerContextPaths("ws-rollback")
	if len(fake.contextPath) != 1 {
		t.Fatalf("initial context path = %v", fake.contextPath)
	}
	acknowledged := fake.contextPath[0]

	if err := os.WriteFile(core, []byte("gen two"), 0o644); err != nil {
		t.Fatal(err)
	}
	fake.calls = nil
	fake.failNextRefresh = true
	app.registerContextPaths("ws-rollback")

	if len(fake.contextPath) != 1 || filepath.Clean(fake.contextPath[0]) != filepath.Clean(acknowledged) {
		t.Fatalf("refresh failure left config on unacknowledged generation: got %v want %q", fake.contextPath, acknowledged)
	}
	wantCalls := []string{"get", "set", "refresh", "set"}
	if !equalStrings(fake.calls, wantCalls) {
		t.Fatalf("refresh rollback calls = %v, want %v", fake.calls, wantCalls)
	}
	if _, err := os.Stat(acknowledged); err != nil {
		t.Fatalf("acknowledged generation lost after refresh failure: %v", err)
	}
}

func TestClearContextPathRefreshFailureRestoresAcknowledgedConfig(t *testing.T) {
	dataDir := t.TempDir()
	seeder := contextseed.New(dataDir, nil)
	if err := os.MkdirAll(seeder.ContextDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(seeder.ContextDir(), "TACK_CORE.md"), []byte("gen one"), 0o644); err != nil {
		t.Fatal(err)
	}
	fake := &contextRegistrationAPI{t: t}
	app := newContextLeaseTestApp(t, seeder, fake)
	app.registerContextPaths("ws-clear")
	acknowledged := fake.contextPath[0]

	fake.calls = nil
	fake.failNextRefresh = true
	app.clearContextPath(context.Background(), app.getConn().api, "ws-clear")
	if len(fake.contextPath) != 1 || filepath.Clean(fake.contextPath[0]) != filepath.Clean(acknowledged) {
		t.Fatalf("failed removal refresh lost acknowledged config: got %v want %q", fake.contextPath, acknowledged)
	}
	wantCalls := []string{"get", "remove", "refresh", "set"}
	if !equalStrings(fake.calls, wantCalls) {
		t.Fatalf("clear rollback calls = %v, want %v", fake.calls, wantCalls)
	}
}

func TestRegisterContextPathsRollbackFailurePinsAcknowledgedAndUncertainGenerations(t *testing.T) {
	dataDir := t.TempDir()
	seeder := contextseed.New(dataDir, nil)
	if err := os.MkdirAll(seeder.ContextDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	core := filepath.Join(seeder.ContextDir(), "TACK_CORE.md")
	if err := os.WriteFile(core, []byte("gen one"), 0o644); err != nil {
		t.Fatal(err)
	}
	fake := &contextRegistrationAPI{t: t}
	app := newContextLeaseTestApp(t, seeder, fake)
	app.registerContextPaths("ws-uncertain")
	acknowledged := fake.contextPath[0]

	if err := os.WriteFile(core, []byte("gen two"), 0o644); err != nil {
		t.Fatal(err)
	}
	fake.calls = nil
	fake.failNextRefresh = true
	fake.failSetAfterRefresh = true
	app.registerContextPaths("ws-uncertain")
	if len(fake.contextPath) != 1 {
		t.Fatalf("uncertain config path = %v", fake.contextPath)
	}
	uncertain := fake.contextPath[0]
	if filepath.Clean(uncertain) == filepath.Clean(acknowledged) {
		t.Fatal("fixture did not advance to uncertain generation")
	}
	if got, want := fake.calls, []string{"get", "set", "refresh", "set"}; !equalStrings(got, want) {
		t.Fatalf("rollback-failure calls = %v want %v", got, want)
	}

	if err := os.WriteFile(core, []byte("gen three"), 0o644); err != nil {
		t.Fatal(err)
	}
	gen3, err := seeder.BuildPromptSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	pruner := contextseed.New(dataDir, nil)
	if err := pruner.PrunePromptSnapshotsChecked(gen3); err != nil {
		t.Fatal(err)
	}
	for label, path := range map[string]string{"acknowledged": acknowledged, "uncertain": uncertain} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("%s generation was pruned while rollback state was uncertain: %v", label, err)
		}
	}

	// A later successful refresh establishes a single acknowledged generation
	// and releases both the old acknowledged lease and the uncertain lease.
	fake.calls = nil
	app.registerContextPaths("ws-uncertain")
	if len(fake.contextPath) != 1 {
		t.Fatalf("recovered config path = %v", fake.contextPath)
	}
	recovered := fake.contextPath[0]
	if err := pruner.PrunePromptSnapshotsChecked(recovered); err != nil {
		t.Fatal(err)
	}
	for label, path := range map[string]string{"old acknowledged": acknowledged, "old uncertain": uncertain} {
		if filepath.Clean(path) == filepath.Clean(recovered) {
			continue
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("%s generation should be prunable after recovery: %v", label, err)
		}
	}
}

func TestRegisterContextPathsReconnectCapturesServerGenerationBeforeRefresh(t *testing.T) {
	dataDir := t.TempDir()
	seeder := contextseed.New(dataDir, nil)
	if err := os.MkdirAll(seeder.ContextDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	core := filepath.Join(seeder.ContextDir(), "TACK_CORE.md")
	if err := os.WriteFile(core, []byte("gen one"), 0o644); err != nil {
		t.Fatal(err)
	}
	fake := &contextRegistrationAPI{t: t}
	first := newContextLeaseTestApp(t, seeder, fake)
	first.registerContextPaths("ws-reconnect")
	serverGeneration := fake.contextPath[0]
	first.releaseAllContextLeases()

	// Simulate a host reconnect/restart: server config survives, but the new App
	// has no in-memory acknowledged lease map.
	second := newContextLeaseTestApp(t, seeder, fake)
	if got := second.contextLeaseGeneration("ws-reconnect"); got != "" {
		t.Fatalf("new app unexpectedly inherited lease %q", got)
	}
	if err := os.WriteFile(core, []byte("gen two"), 0o644); err != nil {
		t.Fatal(err)
	}
	fake.calls = nil
	fake.failNextRefresh = true
	second.registerContextPaths("ws-reconnect")
	if len(fake.contextPath) != 1 || filepath.Clean(fake.contextPath[0]) != filepath.Clean(serverGeneration) {
		t.Fatalf("reconnect rollback did not restore server generation: got %v want %q", fake.contextPath, serverGeneration)
	}
	if got := second.contextLeaseGeneration("ws-reconnect"); filepath.Clean(got) != filepath.Clean(serverGeneration) {
		t.Fatalf("reconnect did not adopt lease for restored server generation: got %q want %q", got, serverGeneration)
	}

	if err := os.WriteFile(core, []byte("gen three"), 0o644); err != nil {
		t.Fatal(err)
	}
	gen3, err := seeder.BuildPromptSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if err := contextseed.New(dataDir, nil).PrunePromptSnapshotsChecked(gen3); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(serverGeneration); err != nil {
		t.Fatalf("restored server generation was pruned after reconnect: %v", err)
	}
}
