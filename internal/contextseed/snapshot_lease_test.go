package contextseed

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSnapshotLeaseProtectsGenerationAcrossSeederInstances(t *testing.T) {
	dataDir := t.TempDir()
	publisher := New(dataDir, nil)
	seedContextWithFile(t, publisher, "TACK_CORE.md", "gen one")
	gen1, lease, err := publisher.BuildPromptSnapshotLease()
	if err != nil {
		t.Fatalf("BuildPromptSnapshotLease = %v", err)
	}
	defer lease.Release()

	seedContextWithFile(t, publisher, "TACK_CORE.md", "gen two")
	gen2, err := publisher.BuildPromptSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	seedContextWithFile(t, publisher, "TACK_CORE.md", "gen three")
	gen3, err := publisher.BuildPromptSnapshot()
	if err != nil {
		t.Fatal(err)
	}

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

	lease.Release()
	if err := pruner.PrunePromptSnapshotsChecked(gen3); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(gen1); !os.IsNotExist(err) {
		t.Fatalf("released gen1 should be prunable: %v", err)
	}
}

func TestSnapshotLeaseMetadataLivesOutsidePromptGeneration(t *testing.T) {
	s := New(t.TempDir(), nil)
	seedContextWithFile(t, s, "TACK_CORE.md", "policy")
	gen, lease, err := s.BuildPromptSnapshotLease()
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Release()
	if filepath.Dir(lease.path) != filepath.Join(s.PromptContextRoot(), snapshotLeaseDirName) {
		t.Fatalf("lease path %q is not in lease metadata directory", lease.path)
	}
	if filepath.Dir(lease.path) == gen || filepath.Clean(lease.path) == filepath.Clean(gen) {
		t.Fatalf("lease metadata leaked into prompt generation: %q", lease.path)
	}
}

func TestSnapshotLeaseReleaseIsIdempotent(t *testing.T) {
	s := New(t.TempDir(), nil)
	seedContextWithFile(t, s, "TACK_CORE.md", "policy")
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
