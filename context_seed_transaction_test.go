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

func writeContextLeaseProfile(t *testing.T, s *contextseed.Seeder, body string) {
	t.Helper()
	if err := os.MkdirAll(s.ContextDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.ContextDir(), "PROFILE.md"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestRegisterContextPathsRefreshFailureRollsBackAcknowledgedConfig(t *testing.T) {
	seeder := contextseed.New(t.TempDir(), nil)
	writeContextLeaseProfile(t, seeder, "gen one")
	fake := &contextRegistrationAPI{t: t}
	app := newContextLeaseTestApp(t, seeder, fake)
	app.registerContextPaths("ws-rollback")
	if len(fake.contextPath) != 1 {
		t.Fatal("initial registration failed")
	}
	acknowledged := fake.contextPath[0]
	writeContextLeaseProfile(t, seeder, "gen two")
	fake.calls = nil
	fake.failNextRefresh = true
	app.registerContextPaths("ws-rollback")
	if !equalStrings(fake.contextPath, []string{acknowledged}) {
		t.Fatal("refresh failure left an unacknowledged generation configured")
	}
	if !equalStrings(fake.calls, []string{"get", "set", "refresh", "set"}) {
		t.Fatalf("refresh rollback calls = %v", fake.calls)
	}
	if _, err := os.Stat(acknowledged); err != nil {
		t.Fatal("acknowledged generation lost after refresh failure")
	}
}

func TestClearContextPathRefreshFailureRestoresAcknowledgedConfig(t *testing.T) {
	seeder := contextseed.New(t.TempDir(), nil)
	writeContextLeaseProfile(t, seeder, "gen one")
	fake := &contextRegistrationAPI{t: t}
	app := newContextLeaseTestApp(t, seeder, fake)
	app.registerContextPaths("ws-clear")
	acknowledged := fake.contextPath[0]
	fake.calls = nil
	fake.failNextRefresh = true
	app.clearContextPath(context.Background(), app.getConn().api, "ws-clear")
	if !equalStrings(fake.contextPath, []string{acknowledged}) {
		t.Fatal("failed removal refresh lost acknowledged config")
	}
	if !equalStrings(fake.calls, []string{"get", "remove", "refresh", "set"}) {
		t.Fatalf("clear rollback calls = %v", fake.calls)
	}
}

func TestRegisterContextPathsRollbackFailurePinsAcknowledgedAndUncertainGenerations(t *testing.T) {
	dataDir := t.TempDir()
	seeder := contextseed.New(dataDir, nil)
	writeContextLeaseProfile(t, seeder, "gen one")
	fake := &contextRegistrationAPI{t: t}
	app := newContextLeaseTestApp(t, seeder, fake)
	app.registerContextPaths("ws-uncertain")
	acknowledged := fake.contextPath[0]
	writeContextLeaseProfile(t, seeder, "gen two")
	fake.calls = nil
	fake.failNextRefresh = true
	fake.failSetAfterRefresh = true
	app.registerContextPaths("ws-uncertain")
	if len(fake.contextPath) != 1 {
		t.Fatal("uncertain config path missing")
	}
	uncertain := fake.contextPath[0]
	if uncertain == acknowledged || !equalStrings(fake.calls, []string{"get", "set", "refresh", "set"}) {
		t.Fatal("fixture did not enter the rollback-failure state")
	}
	writeContextLeaseProfile(t, seeder, "gen three")
	gen3, err := seeder.BuildPromptSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	pruner := contextseed.New(dataDir, nil)
	if err := pruner.PrunePromptSnapshotsChecked(gen3); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{acknowledged, uncertain} {
		if _, err := os.Stat(path); err != nil {
			t.Fatal("rollback uncertainty lost a potentially active generation")
		}
	}
	app.registerContextPaths("ws-uncertain")
	recovered := fake.contextPath[0]
	if err := pruner.PrunePromptSnapshotsChecked(recovered); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{acknowledged, uncertain} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("old generation leaked after successful recovery")
		}
	}
}

func TestRegisterContextPathsReconnectCapturesServerGenerationBeforeRefresh(t *testing.T) {
	dataDir := t.TempDir()
	seeder := contextseed.New(dataDir, nil)
	writeContextLeaseProfile(t, seeder, "gen one")
	fake := &contextRegistrationAPI{t: t}
	first := newContextLeaseTestApp(t, seeder, fake)
	first.registerContextPaths("ws-reconnect")
	serverGeneration := fake.contextPath[0]
	first.releaseAllContextLeases()
	second := newContextLeaseTestApp(t, seeder, fake)
	if second.contextLeaseGeneration("ws-reconnect") != "" {
		t.Fatal("new app inherited an in-memory lease")
	}
	writeContextLeaseProfile(t, seeder, "gen two")
	fake.failNextRefresh = true
	second.registerContextPaths("ws-reconnect")
	if !equalStrings(fake.contextPath, []string{serverGeneration}) || second.contextLeaseGeneration("ws-reconnect") != serverGeneration {
		t.Fatal("reconnect failed to restore and pin the server generation")
	}
	writeContextLeaseProfile(t, seeder, "gen three")
	gen3, err := seeder.BuildPromptSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if err := contextseed.New(dataDir, nil).PrunePromptSnapshotsChecked(gen3); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(serverGeneration); err != nil {
		t.Fatal("restored server generation was pruned after reconnect")
	}
}
