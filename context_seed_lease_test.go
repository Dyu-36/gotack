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

func TestRegisterContextPathsHoldsAcknowledgedWorkspaceGeneration(t *testing.T) {
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
	api := crushapi.NewClient(&http.Client{Transport: fake})
	app := NewApp()
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
	app.registerContextPaths("ws-lease")
	if len(fake.contextPath) != 1 {
		t.Fatalf("registered path = %v", fake.contextPath)
	}
	gen1 := fake.contextPath[0]

	if err := os.WriteFile(core, []byte("gen two"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := seeder.BuildPromptSnapshot(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(core, []byte("gen three"), 0o644); err != nil {
		t.Fatal(err)
	}
	gen3, err := seeder.BuildPromptSnapshot()
	if err != nil {
		t.Fatal(err)
	}

	otherProcessView := contextseed.New(dataDir, nil)
	if err := otherProcessView.PrunePromptSnapshotsChecked(gen3); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(gen1); err != nil {
		t.Fatalf("acknowledged workspace generation was pruned: %v", err)
	}

	app.releaseAllContextLeases()
	if err := otherProcessView.PrunePromptSnapshotsChecked(gen3); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(gen1); !os.IsNotExist(err) {
		t.Fatalf("workspace generation should prune after teardown release: %v", err)
	}
}
