package contextseed

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCommittedSnapshotCorruptionFailsClosed(t *testing.T) {
	for _, mutation := range []string{"extra", "missing", "modified"} {
		t.Run(mutation, func(t *testing.T) {
			s := New(t.TempDir(), nil)
			gen := snapshotWithProfile(t, s, "same preference")
			switch mutation {
			case "extra":
				writeContextFixture(t, filepath.Join(gen, "extra.md"), "unexpected")
			case "missing":
				if err := os.Remove(profilePath(gen)); err != nil {
					t.Fatal(err)
				}
			case "modified":
				writeContextFixture(t, profilePath(gen), "tampered")
			}
			if _, err := s.BuildPromptSnapshot(); err == nil {
				t.Fatal("corrupted committed revision was silently reused")
			}
		})
	}
}

func TestStagedSnapshotLinksAreRejected(t *testing.T) {
	s := New(t.TempDir(), nil)
	old := snapshotWithProfile(t, s, "old preference")
	outside := filepath.Join(t.TempDir(), "outside.md")
	writeContextFixture(t, outside, "outside data")
	probe := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(outside, probe); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	seedContextWithFile(t, s, "PROFILE.md", "new preference")
	withStagedHook(t, func(stage string) {
		if err := os.Remove(profilePath(stage)); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, profilePath(stage)); err != nil {
			t.Fatal(err)
		}
	})
	if _, err := s.BuildPromptSnapshot(); err == nil {
		t.Fatal("linked staged file was published")
	}
	if string(mustRead(t, profilePath(old))) != profilePayload("old preference") {
		t.Fatal("staged link modified the old revision")
	}
}
