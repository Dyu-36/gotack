package contextseed

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func withStagedHook(t *testing.T, hook func(string)) {
	t.Helper()
	previous := beforeValidateStagedSnapshot
	beforeValidateStagedSnapshot = hook
	t.Cleanup(func() { beforeValidateStagedSnapshot = previous })
}

func TestStagedSnapshotRejectsExtraMissingAndModifiedFiles(t *testing.T) {
	for _, mutation := range []string{"extra", "missing", "modified"} {
		t.Run(mutation, func(t *testing.T) {
			s := New(t.TempDir(), nil)
			old := snapshotWithProfile(t, s, "old preference")
			seedContextWithFile(t, s, "PROFILE.md", "new preference")
			withStagedHook(t, func(stage string) {
				switch mutation {
				case "extra":
					writeContextFixture(t, filepath.Join(stage, "extra.md"), "unexpected")
				case "missing":
					if err := os.Remove(profilePath(stage)); err != nil {
						t.Fatal(err)
					}
				case "modified":
					writeContextFixture(t, profilePath(stage), "tampered")
				}
			})
			if _, err := s.BuildPromptSnapshot(); err == nil {
				t.Fatal("invalid staged tree was published")
			}
			if string(mustRead(t, profilePath(old))) != profilePayload("old preference") {
				t.Fatal("failed publication changed the previous revision")
			}
			entries, err := os.ReadDir(s.PromptContextRoot())
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), ".staging-") {
					t.Fatal("failed publication leaked staging files")
				}
			}
		})
	}
}

func TestSnapshotReadErrorRetainsPreviouslyCommittedBytes(t *testing.T) {
	s := New(t.TempDir(), nil)
	old := snapshotWithProfile(t, s, "old preference")
	seedContextWithFile(t, s, "PROFILE.md", "new preference")
	previous := beforeReadSnapshotFile
	beforeReadSnapshotFile = func(_, _ string) error { return errors.New("injected read failure") }
	t.Cleanup(func() { beforeReadSnapshotFile = previous })
	if _, err := s.BuildPromptSnapshot(); err == nil {
		t.Fatal("read failure published a partial revision")
	}
	if string(mustRead(t, profilePath(old))) != profilePayload("old preference") {
		t.Fatal("read failure changed committed bytes")
	}
}

func TestSnapshotRejectsLinkedPersonalSource(t *testing.T) {
	s := New(t.TempDir(), nil)
	old := snapshotWithProfile(t, s, "old preference")
	outside := filepath.Join(t.TempDir(), "outside.md")
	writeContextFixture(t, outside, "outside data")
	source := filepath.Join(s.ContextDir(), "PROFILE.md")
	if err := os.Remove(source); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, source); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := s.BuildPromptSnapshot(); err == nil {
		t.Fatal("linked personal source was followed")
	}
	if string(mustRead(t, profilePath(old))) != profilePayload("old preference") {
		t.Fatal("linked source damaged the committed revision")
	}
}

func TestConcurrentSnapshotPublishersReuseOneCompleteRevision(t *testing.T) {
	s := New(t.TempDir(), nil)
	seedContextWithFile(t, s, "PROFILE.md", "same preference")
	const count = 16
	paths := make([]string, count)
	errs := make([]error, count)
	var wg sync.WaitGroup
	for i := range paths {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			paths[i], errs[i] = New(s.dataDir, nil).BuildPromptSnapshot()
		}(i)
	}
	wg.Wait()
	for i := range paths {
		if errs[i] != nil || paths[i] != paths[0] {
			t.Fatalf("publisher %d returned inconsistent revision: %q %v", i, paths[i], errs[i])
		}
		if string(mustRead(t, profilePath(paths[i]))) != profilePayload("same preference") {
			t.Fatal("publisher served incomplete bytes")
		}
	}
}

func TestReaderSetRetentionIsBoundedByReaders(t *testing.T) {
	s := New(t.TempDir(), nil)
	one := snapshotWithProfile(t, s, "generation one")
	two := snapshotWithProfile(t, s, "generation two")
	three := snapshotWithProfile(t, s, "generation three")
	s.AcquireReader("workspace", "run-a", one)
	s.AcquireReader("workspace", "run-b", one)
	s.AcquireReader("workspace", "run-c", two)
	if err := s.PrunePromptSnapshotsChecked(three); err != nil {
		t.Fatal(err)
	}
	for _, gen := range []string{one, two, three} {
		if _, err := os.Stat(gen); err != nil {
			t.Fatalf("live reader/current generation pruned: %v", err)
		}
	}
	s.ReleaseReader("workspace", "run-a")
	if err := s.PrunePromptSnapshotsChecked(three); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(one); err != nil {
		t.Fatal("second reader did not protect the old generation")
	}
	s.ReleaseReader("workspace", "run-b")
	s.ReleaseReader("workspace", "run-c")
	if err := s.PrunePromptSnapshotsChecked(three); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(one); !os.IsNotExist(err) {
		t.Fatal("released generation was not pruned")
	}
	if _, err := os.Stat(two); err != nil {
		t.Fatal("previous-generation safety floor was removed")
	}
	four := snapshotWithProfile(t, s, "generation four")
	if err := s.PrunePromptSnapshotsChecked(four); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(two); !os.IsNotExist(err) {
		t.Fatal("aged-out safety floor leaked")
	}
	for _, gen := range []string{three, four} {
		if _, err := os.Stat(gen); err != nil {
			t.Fatal("current/previous generation was pruned")
		}
	}
}

func TestAcquireReleaseReaderAreSafe(t *testing.T) {
	s := New(t.TempDir(), nil)
	s.AcquireReader("ws", "run", "gen-a")
	s.AcquireReader("ws", "run", "gen-b")
	copy := s.RegisteredReaders()
	if copy[readerKey("ws", "run")] != "gen-b" {
		t.Fatal("reader registration did not replace the same key")
	}
	delete(copy, readerKey("ws", "run"))
	if len(s.RegisteredReaders()) != 1 {
		t.Fatal("caller mutated the live registry")
	}
	s.ReleaseReader("ws", "run")
	s.ReleaseReader("ws", "run")
	s.ReleaseReader("unknown", "key")
	if len(s.RegisteredReaders()) != 0 {
		t.Fatal("released reader remains registered")
	}
}
