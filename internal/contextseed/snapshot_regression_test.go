package contextseed

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// withStagedHook installs a one-shot test hook that fires after
// BuildPromptSnapshot finishes writing the staging tree but before
// validateStagedSnapshot runs. The hook is restored when the helper
// returns so a failing test does not poison sibling tests.
func withStagedHook(t *testing.T, fn func(staging string)) {
	t.Helper()
	previous := beforeValidateStagedSnapshot
	beforeValidateStagedSnapshot = fn
	t.Cleanup(func() { beforeValidateStagedSnapshot = previous })
}

// TestBuildPromptSnapshotRejectsModifiedStagedByte guards the IP-02
// "manifest mismatching revision is rejected" acceptance scenario:
// the staged tree has its bytes mutated between collect and
// validate. BuildPromptSnapshot must reject the publish and the
// previously committed revision must remain fully intact.
func TestBuildPromptSnapshotRejectsModifiedStagedByte(t *testing.T) {
	seeder := New(t.TempDir(), nil)
	seedContextWithFile(t, seeder, "TACK_CORE.md", "committed policy")
	committed, err := seeder.BuildPromptSnapshot()
	if err != nil {
		t.Fatalf("BuildPromptSnapshot = %v", err)
	}
	beforeContent := string(mustRead(t, filepath.Join(committed, "TACK_CORE.md")))

	// Trigger a second publish on different content so a new staging
	// dir is opened. The hook mutates one byte of the staged tree.
	if err := os.WriteFile(filepath.Join(seeder.ContextDir(), "TACK_CORE.md"), []byte("new policy"), 0o644); err != nil {
		t.Fatal(err)
	}
	withStagedHook(t, func(staging string) {
		_ = os.WriteFile(filepath.Join(staging, "TACK_CORE.md"), []byte("corrupted"), 0o644)
	})

	if _, err := seeder.BuildPromptSnapshot(); err == nil {
		t.Fatal("BuildPromptSnapshot accepted a corrupted staged byte")
	} else if !strings.Contains(err.Error(), "does not match manifest") &&
		!strings.Contains(err.Error(), "digest") {
		t.Fatalf("BuildPromptSnapshot error = %v, want digest / content mismatch", err)
	}

	got := string(mustRead(t, filepath.Join(committed, "TACK_CORE.md")))
	if got != beforeContent {
		t.Fatalf("committed revision was modified: got %q, want %q", got, beforeContent)
	}
}

// TestBuildPromptSnapshotRejectsMissingManifestFile guards the IP-02
// "snapshot missing a manifest file is rejected" scenario: a file
// disappears from the staged tree before validate runs.
func TestBuildPromptSnapshotRejectsMissingManifestFile(t *testing.T) {
	seeder := New(t.TempDir(), nil)
	seedContextWithFile(t, seeder, "TACK_CORE.md", "committed policy")
	committed, err := seeder.BuildPromptSnapshot()
	if err != nil {
		t.Fatalf("BuildPromptSnapshot = %v", err)
	}
	beforeContent := string(mustRead(t, filepath.Join(committed, "TACK_CORE.md")))

	seedContextWithFile(t, seeder, "guide.md", "guide text")
	if _, err := seeder.BuildPromptSnapshot(); err != nil {
		t.Fatalf("BuildPromptSnapshot first refresh = %v", err)
	}

	seedContextWithFile(t, seeder, "guide.md", "guide v2")
	withStagedHook(t, func(staging string) {
		_ = os.Remove(filepath.Join(staging, "guide.md"))
	})

	if _, err := seeder.BuildPromptSnapshot(); err == nil {
		t.Fatal("BuildPromptSnapshot accepted a missing manifest file")
	} else if !strings.Contains(err.Error(), "missing file") &&
		!strings.Contains(err.Error(), "file count") {
		t.Fatalf("BuildPromptSnapshot error = %v, want missing-file diagnostic", err)
	}

	got := string(mustRead(t, filepath.Join(committed, "TACK_CORE.md")))
	if got != beforeContent {
		t.Fatalf("committed TACK_CORE.md was modified: got %q, want %q", got, beforeContent)
	}
}

// TestBuildPromptSnapshotRejectsExtraStagedFile guards the IP-02
// "snapshot has an extra file not in manifest is rejected" scenario:
// the staging walker is fed a transient file that the manifest does
// not declare.
func TestBuildPromptSnapshotRejectsExtraStagedFile(t *testing.T) {
	seeder := New(t.TempDir(), nil)
	seedContextWithFile(t, seeder, "TACK_CORE.md", "committed policy")
	committed, err := seeder.BuildPromptSnapshot()
	if err != nil {
		t.Fatalf("BuildPromptSnapshot = %v", err)
	}
	beforeContent := string(mustRead(t, filepath.Join(committed, "TACK_CORE.md")))

	seedContextWithFile(t, seeder, "guide.md", "guide text")
	if _, err := seeder.BuildPromptSnapshot(); err != nil {
		t.Fatalf("BuildPromptSnapshot first refresh = %v", err)
	}

	seedContextWithFile(t, seeder, "guide.md", "guide v2")
	withStagedHook(t, func(staging string) {
		_ = os.WriteFile(filepath.Join(staging, "extra.md"), []byte("stray"), 0o644)
	})

	if _, err := seeder.BuildPromptSnapshot(); err == nil {
		t.Fatal("BuildPromptSnapshot accepted an extra staged file")
	} else if !strings.Contains(err.Error(), "file count") {
		t.Fatalf("BuildPromptSnapshot error = %v, want file-count diagnostic", err)
	}

	got := string(mustRead(t, filepath.Join(committed, "TACK_CORE.md")))
	if got != beforeContent {
		t.Fatalf("committed TACK_CORE.md was modified: got %q, want %q", got, beforeContent)
	}
}

// TestBuildPromptSnapshotFailsClosedOnPerFileReadError guards the
// IP-02 "read errors during collectSnapshot fail closed" scenario.
// A source file is replaced with a path whose read fails (a broken
// symlink or a directory in place of a file): BuildPromptSnapshot
// must surface the read error rather than silently skip the file.
// The directory-level variant is already covered by
// TestFailedSnapshotRefreshKeepsCommittedRevision in
// snapshot_identity_test.go.
func TestBuildPromptSnapshotFailsClosedOnPerFileReadError(t *testing.T) {
	seeder := New(t.TempDir(), nil)
	seedContextWithFile(t, seeder, "TACK_CORE.md", "committed policy")
	committed, err := seeder.BuildPromptSnapshot()
	if err != nil {
		t.Fatalf("BuildPromptSnapshot = %v", err)
	}
	beforeContent := string(mustRead(t, filepath.Join(committed, "TACK_CORE.md")))

	// The contract distinguishes memory-file exclusions (documented
	// per-file policy) from context-file read errors (must fail
	// closed). To exercise the context-file path deterministically
	// without racing the walk, the test registers a collect hook
	// that drops a specific source file just before the walker
	// visits it. collectSnapshot must then surface a read error
	// rather than silently skipping the file.
	guidePath := filepath.Join(seeder.ContextDir(), "guide.md")
	if err := os.WriteFile(guidePath, []byte("guide text"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Rotate the snapshot identity so the publisher walks the
	// source tree instead of reusing the existing committed
	// revision.
	if err := os.WriteFile(filepath.Join(seeder.ContextDir(), "TACK_CORE.md"), []byte("policy v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := seeder.BuildPromptSnapshot(); err != nil {
		t.Fatalf("BuildPromptSnapshot first refresh = %v", err)
	}
	// Use the per-file read hook to inject a transient read error
	// on the specific file we want to fail. The hook fires after
	// the file has been read but before it lands in the manifest,
	// so BuildPromptSnapshot must surface the error and not
	// publish a partial manifest.
	previousRead := beforeReadSnapshotFile
	beforeReadSnapshotFile = func(path, rel string) error {
		if filepath.Base(path) == "guide.md" {
			return fmt.Errorf("injected read error for %s", rel)
		}
		return nil
	}
	t.Cleanup(func() { beforeReadSnapshotFile = previousRead })
	if err := os.WriteFile(filepath.Join(seeder.ContextDir(), "TACK_CORE.md"), []byte("policy v3"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := seeder.BuildPromptSnapshot(); err == nil {
		t.Fatal("BuildPromptSnapshot succeeded when collectSnapshot read failed")
	} else if !strings.Contains(err.Error(), "read context file") &&
		!strings.Contains(err.Error(), "injected") {
		t.Fatalf("BuildPromptSnapshot error = %v, want read-context-file diagnostic", err)
	}

	got := string(mustRead(t, filepath.Join(committed, "TACK_CORE.md")))
	if got != beforeContent {
		t.Fatalf("committed TACK_CORE.md was modified: got %q, want %q", got, beforeContent)
	}
}

// TestBuildPromptSnapshotConcurrentRefreshSerializes guards the IP-02
// "concurrent publish/refresh races do not corrupt state" scenario:
// multiple goroutines call BuildPromptSnapshot on the same source.
// Every successful publish must point at a real directory; every
// failure must not corrupt the previously committed revision.
func TestBuildPromptSnapshotConcurrentRefreshSerializes(t *testing.T) {
	seeder := New(t.TempDir(), nil)
	seedContextWithFile(t, seeder, "TACK_CORE.md", "committed policy")
	committed, err := seeder.BuildPromptSnapshot()
	if err != nil {
		t.Fatalf("BuildPromptSnapshot = %v", err)
	}
	beforeContent := string(mustRead(t, filepath.Join(committed, "TACK_CORE.md")))

	const publishers = 8
	var wg sync.WaitGroup
	wg.Add(publishers)
	results := make([]string, publishers)
	errs := make([]error, publishers)
	start := make(chan struct{})
	for i := 0; i < publishers; i++ {
		go func(idx int) {
			defer wg.Done()
			<-start
			dir, err := seeder.BuildPromptSnapshot()
			results[idx] = dir
			errs[idx] = err
		}(i)
	}
	close(start)
	wg.Wait()

	successCount := 0
	var firstSuccess string
	for i, err := range errs {
		if err == nil {
			successCount++
			if firstSuccess == "" {
				firstSuccess = results[i]
			}
		}
	}
	if successCount == 0 {
		t.Fatalf("all publishers failed; results=%v errs=%v", results, errs)
	}
	if firstSuccess == "" {
		t.Fatal("no successful publish captured")
	}

	for i, dir := range results {
		if errs[i] == nil {
			info, statErr := os.Stat(dir)
			if statErr != nil || !info.IsDir() {
				t.Fatalf("publisher %d reported %q but it is not a directory (stat=%v)", i, dir, statErr)
			}
		}
	}
	got := string(mustRead(t, filepath.Join(committed, "TACK_CORE.md")))
	if got != beforeContent {
		t.Fatalf("committed revision was modified: got %q, want %q", got, beforeContent)
	}
}

// TestReaderSetRetentionKeepsSlowReaderThroughThreeGenerations
// guards the IP-02 "slow readers through multiple generations are
// not pruned underneath" scenario. A reader is acquired on the
// first generation, then two more generations are published; the
// reader's generation is kept alive across both refreshes.
func TestReaderSetRetentionKeepsSlowReaderThroughThreeGenerations(t *testing.T) {
	seeder := New(t.TempDir(), nil)
	seedContextWithFile(t, seeder, "TACK_CORE.md", "gen one")
	gen1, err := seeder.BuildPromptSnapshot()
	if err != nil {
		t.Fatalf("BuildPromptSnapshot = %v", err)
	}

	// Acquire a reader on gen1 before any refresh runs.
	seeder.AcquireReader("ws", "run", gen1)
	defer seeder.ReleaseReader("ws", "run")

	seedContextWithFile(t, seeder, "TACK_CORE.md", "gen two")
	gen2, err := seeder.BuildPromptSnapshot()
	if err != nil {
		t.Fatalf("BuildPromptSnapshot second = %v", err)
	}
	seedContextWithFile(t, seeder, "TACK_CORE.md", "gen three")
	gen3, err := seeder.BuildPromptSnapshot()
	if err != nil {
		t.Fatalf("BuildPromptSnapshot third = %v", err)
	}

	// Prune while the reader still holds gen1: gen1 must survive
	// even though it is no longer the current generation.
	seeder.PrunePromptSnapshots(gen3)
	if _, err := os.Stat(gen1); err != nil {
		t.Fatalf("gen1 was pruned while a reader still holds it: %v", err)
	}
	if _, err := os.Stat(gen2); err != nil {
		t.Fatalf("gen2 was pruned before its turn in the reader-set rotation: %v", err)
	}
	if _, err := os.Stat(gen3); err != nil {
		t.Fatalf("current generation was pruned: %v", err)
	}

	// Release gen1 and prune again: gen1 must now be prunable but
	// gen2 + gen3 must still survive (gen2 is now the
	// previousSnapshot safety floor and gen3 is the current
	// committed revision).
	seeder.ReleaseReader("ws", "run")
	seeder.PrunePromptSnapshots(gen3)
	if _, err := os.Stat(gen1); !os.IsNotExist(err) {
		t.Fatalf("gen1 should have been released: stat err = %v", err)
	}
	if _, err := os.Stat(gen2); err != nil {
		t.Fatalf("gen2 was pruned too early: %v", err)
	}
	if _, err := os.Stat(gen3); err != nil {
		t.Fatalf("current generation was pruned: %v", err)
	}
}

// TestReaderSetRetentionIsBoundedByReaders guards the IP-02
// "retention is bounded by the actual reader set" scenario: the
// arbitrary two-generation cap is gone, the reader registry drives
// retention. The safety floor (the previous committed revision) is
// permitted by the contract ("If a bounded cap is kept for safety,
// it MUST be derived from the reader set, not arbitrary"). After
// all readers release and the safety floor ages out via a fresh
// commit, only the currently committed revision survives.
func TestReaderSetRetentionIsBoundedByReaders(t *testing.T) {
	seeder := New(t.TempDir(), nil)
	seedContextWithFile(t, seeder, "TACK_CORE.md", "gen one")
	gen1, err := seeder.BuildPromptSnapshot()
	if err != nil {
		t.Fatalf("BuildPromptSnapshot = %v", err)
	}
	seedContextWithFile(t, seeder, "TACK_CORE.md", "gen two")
	gen2, err := seeder.BuildPromptSnapshot()
	if err != nil {
		t.Fatalf("BuildPromptSnapshot = %v", err)
	}
	seedContextWithFile(t, seeder, "TACK_CORE.md", "gen three")
	gen3, err := seeder.BuildPromptSnapshot()
	if err != nil {
		t.Fatalf("BuildPromptSnapshot = %v", err)
	}

	// Two readers on gen1 and one on gen2. gen3 is the current
	// committed revision and is therefore protected on every prune.
	seeder.AcquireReader("ws", "run-a", gen1)
	seeder.AcquireReader("ws", "run-b", gen1)
	seeder.AcquireReader("ws", "run-c", gen2)
	defer seeder.ReleaseReader("ws", "run-a")
	defer seeder.ReleaseReader("ws", "run-b")
	defer seeder.ReleaseReader("ws", "run-c")

	seeder.PrunePromptSnapshots(gen3)
	if _, err := os.Stat(gen1); err != nil {
		t.Fatalf("gen1 was pruned while readers still hold it: %v", err)
	}
	if _, err := os.Stat(gen2); err != nil {
		t.Fatalf("gen2 was pruned while a reader still holds it: %v", err)
	}
	if _, err := os.Stat(gen3); err != nil {
		t.Fatalf("current generation was pruned: %v", err)
	}

	// Release gen2's reader. The safety floor (gen2 is the
	// previousSnapshot relative to gen3) keeps it alive for one
	// more prune cycle.
	seeder.ReleaseReader("ws", "run-c")
	seeder.PrunePromptSnapshots(gen3)
	if _, err := os.Stat(gen1); err != nil {
		t.Fatalf("gen1 was pruned early while readers still hold it: %v", err)
	}
	if _, err := os.Stat(gen2); err != nil {
		t.Fatalf("gen2 was pruned too early: %v", err)
	}
	if _, err := os.Stat(gen3); err != nil {
		t.Fatalf("current generation was pruned: %v", err)
	}

	// Release gen1's readers. gen1 is the only generation not on
	// the safety floor and not held by any reader, so it is
	// prunable now.
	seeder.ReleaseReader("ws", "run-a")
	seeder.ReleaseReader("ws", "run-b")
	seeder.PrunePromptSnapshots(gen3)
	if _, err := os.Stat(gen1); !os.IsNotExist(err) {
		t.Fatalf("gen1 should have been released: stat err = %v", err)
	}
	if _, err := os.Stat(gen2); err != nil {
		t.Fatalf("gen2 must survive as the previousSnapshot safety floor: %v", err)
	}
	if _, err := os.Stat(gen3); err != nil {
		t.Fatalf("current generation was pruned: %v", err)
	}

	// Age the safety floor out: commit a fresh generation so the
	// previousSnapshot shifts from gen2 to gen3, and prune again.
	seedContextWithFile(t, seeder, "TACK_CORE.md", "gen four")
	gen4, err := seeder.BuildPromptSnapshot()
	if err != nil {
		t.Fatalf("BuildPromptSnapshot = %v", err)
	}
	seeder.PrunePromptSnapshots(gen4)
	if _, err := os.Stat(gen2); !os.IsNotExist(err) {
		t.Fatalf("gen2 should be prunable once the safety floor shifts: stat err = %v", err)
	}
	if _, err := os.Stat(gen3); err != nil {
		t.Fatalf("gen3 must survive as the previousSnapshot safety floor: %v", err)
	}
	if _, err := os.Stat(gen4); err != nil {
		t.Fatalf("current generation was pruned: %v", err)
	}

	// The reader-set rule must appear in the public contract for
	// future maintainers.
	if !strings.Contains(extractPruneDocComment(), "reader set") {
		t.Fatalf("PrunePromptSnapshots doc comment must mention the reader-set rule; got %q", extractPruneDocComment())
	}
}

// TestAcquireReleaseReaderAreSafe guards the public reader API:
// acquiring twice on the same key overwrites (latest wins);
// releasing an unknown key is a no-op; double-release is safe.
func TestAcquireReleaseReaderAreSafe(t *testing.T) {
	seeder := New(t.TempDir(), nil)
	seeder.AcquireReader("ws", "run", "gen-a")
	seeder.AcquireReader("ws", "run", "gen-b")
	got := seeder.RegisteredReaders()[readerKey("ws", "run")]
	if got != "gen-b" {
		t.Fatalf("acquire overwrite = %q, want gen-b", got)
	}
	seeder.ReleaseReader("ws", "run")
	seeder.ReleaseReader("ws", "run") // double release: no-op
	if _, ok := seeder.RegisteredReaders()[readerKey("ws", "run")]; ok {
		t.Fatalf("reader key still present after release")
	}
	seeder.ReleaseReader("unknown", "key") // unknown key: no-op
}

// extractPruneDocComment reads the source file snapshot.go (which
// is in the same package directory) and returns the doc comment
// immediately above the PrunePromptSnapshots method. The test uses
// it to verify the public contract mentions the reader-set rule.
func extractPruneDocComment() string {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return ""
	}
	pkgDir := filepath.Dir(thisFile)
	data, err := os.ReadFile(filepath.Join(pkgDir, "snapshot.go"))
	if err != nil {
		return ""
	}
	const marker = "func (s *Seeder) PrunePromptSnapshots(keep string)"
	idx := strings.Index(string(data), marker)
	if idx < 0 {
		return ""
	}
	prefix := string(data)[:idx]
	lines := strings.Split(prefix, "\n")
	collected := make([]string, 0, 8)
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" && len(collected) > 0 {
			break
		}
		if strings.HasPrefix(line, "//") || line == "" {
			collected = append([]string{lines[i]}, collected...)
			continue
		}
		break
	}
	return strings.Join(collected, "\n")
}
