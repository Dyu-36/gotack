package recall

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestLockProbeRetriesUntilReleased(t *testing.T) {
	dataDir := t.TempDir()
	lockPath := filepath.Join(dataDir, dataDirLockFile)

	release, err := holdDataDirLock(lockPath)
	if err != nil {
		t.Fatalf("holder acquire: %v", err)
	}
	go func() {
		time.Sleep(80 * time.Millisecond)
		release()
	}()

	start := time.Now()
	probe, err := probeDataDirLock(context.Background(), dataDir, 100, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if probe.Contended {
		t.Fatalf("probe stayed contended after the holder released: %+v", probe)
	}
	if probe.Attempts < 2 {
		t.Fatalf("probe should have retried while the lock was held: %+v", probe)
	}
	if elapsed := time.Since(start); elapsed < 60*time.Millisecond {
		t.Fatalf("probe returned before the holder could release: %v", elapsed)
	}
}

func TestLockProbeBoundsItsWait(t *testing.T) {
	dataDir := t.TempDir()
	lockPath := filepath.Join(dataDir, dataDirLockFile)

	release, err := holdDataDirLock(lockPath)
	if err != nil {
		t.Fatalf("holder acquire: %v", err)
	}
	defer release()

	start := time.Now()
	probe, err := probeDataDirLock(context.Background(), dataDir, 3, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("probe must not fail on contention: %v", err)
	}
	if !probe.Contended {
		t.Fatalf("probe must report contention: %+v", probe)
	}
	if probe.Attempts != 3 {
		t.Fatalf("attempts = %d, want the full budget of 3", probe.Attempts)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("probe blocked too long on a held lock: %v", elapsed)
	}
}

func TestLockProbeFreeWithoutLockFile(t *testing.T) {
	probe, err := probeDataDirLock(context.Background(), t.TempDir(), 0, 0)
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if probe.Contended || probe.Attempts != 1 {
		t.Fatalf("missing lock file must read free on one attempt: %+v", probe)
	}
}

func TestSearchProceedsWhileEngineHoldsLock(t *testing.T) {
	dataDir := standardFixture(t, t.TempDir())
	release, err := holdDataDirLock(filepath.Join(dataDir, dataDirLockFile))
	if err != nil {
		t.Fatalf("holder acquire: %v", err)
	}
	defer release()

	store := newTestStore(t, dataDir)
	results, err := store.Search(context.Background(), "kubernetes", 10)
	if err != nil {
		t.Fatalf("search must proceed read-only while crush.lock is held: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("hits under held lock = %+v", results)
	}
}

func TestProbeContextCancelStopsRetries(t *testing.T) {
	dataDir := t.TempDir()
	release, err := holdDataDirLock(filepath.Join(dataDir, dataDirLockFile))
	if err != nil {
		t.Fatalf("holder acquire: %v", err)
	}
	defer release()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if _, err := probeDataDirLock(ctx, dataDir, 1000, 10*time.Millisecond); err == nil {
		t.Fatal("probe must surface context cancellation")
	}
}
