package contextseed

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSnapshotLeaseProtectsGenerationAcrossSeederInstances(t *testing.T) {
	dataDir := t.TempDir()
	publisher := New(dataDir, nil)
	seedContextWithFile(t, publisher, "PROFILE.md", "gen one")
	gen1, lease, err := publisher.BuildPromptSnapshotLease()
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Release()
	gen2 := snapshotWithProfile(t, publisher, "gen two")
	gen3 := snapshotWithProfile(t, publisher, "gen three")
	pruner := New(dataDir, nil)
	if err := pruner.PrunePromptSnapshotsChecked(gen3); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(gen1); err != nil {
		t.Fatalf("leased gen1 pruned by second Seeder: %v", err)
	}
	if _, err := os.Stat(gen2); !os.IsNotExist(err) {
		t.Fatalf("unleased gen2 should be prunable across Seeder instances: %v", err)
	}
	if err := lease.Release(); err != nil {
		t.Fatal(err)
	}
	if err := pruner.PrunePromptSnapshotsChecked(gen3); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(gen1); !os.IsNotExist(err) {
		t.Fatalf("released gen1 should be prunable: %v", err)
	}
}

func TestSnapshotLeaseMetadataLivesOutsidePromptGeneration(t *testing.T) {
	s := New(t.TempDir(), nil)
	seedContextWithFile(t, s, "PROFILE.md", "preference")
	gen, lease, err := s.BuildPromptSnapshotLease()
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Release()
	if filepath.Dir(lease.path) != filepath.Join(s.PromptContextRoot(), snapshotLeaseDirName) {
		t.Fatalf("lease metadata is not in its own directory: %q", lease.path)
	}
	if filepath.Dir(lease.path) == gen || filepath.Clean(lease.path) == filepath.Clean(gen) {
		t.Fatalf("lease metadata leaked into the prompt: %q", lease.path)
	}
}

func TestSnapshotLeaseReleaseIsIdempotent(t *testing.T) {
	s := New(t.TempDir(), nil)
	seedContextWithFile(t, s, "PROFILE.md", "preference")
	_, lease, err := s.BuildPromptSnapshotLease()
	if err != nil {
		t.Fatal(err)
	}
	if err := lease.Release(); err != nil {
		t.Fatal(err)
	}
	if err := lease.Release(); err != nil {
		t.Fatalf("second Release = %v", err)
	}
}
