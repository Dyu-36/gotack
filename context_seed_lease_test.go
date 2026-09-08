package main

import (
	"os"
	"testing"

	"github.com/Dyu-36/gotack/internal/contextseed"
)

func TestRegisterContextPathsHoldsAcknowledgedWorkspaceGeneration(t *testing.T) {
	dataDir := t.TempDir()
	seeder := contextseed.New(dataDir, nil)
	writeContextLeaseProfile(t, seeder, "gen one")
	fake := &contextRegistrationAPI{t: t}
	app := newContextLeaseTestApp(t, seeder, fake)
	app.registerContextPaths("ws-lease")
	if len(fake.contextPath) != 1 {
		t.Fatalf("registered path = %v", fake.contextPath)
	}
	gen1 := fake.contextPath[0]
	writeContextLeaseProfile(t, seeder, "gen two")
	if _, err := seeder.BuildPromptSnapshot(); err != nil {
		t.Fatal(err)
	}
	writeContextLeaseProfile(t, seeder, "gen three")
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
