package recall

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// dataDirLockFile is the engine's lock file name, mirrored from
// third_party/crush/internal/db/datadirlock.go.
const dataDirLockFile = "crush.lock"

// errLockContended is the sentinel for a lock held by another process. It
// mirrors Crush's lock.ErrContended semantics without importing the engine.
var errLockContended = errors.New("file lock is held by another process")

// Default lock policy. The running engine holds crush.lock for its whole
// lifetime, so the retry budget is short: it exists to ride out engine
// restart and migration windows, not to wait the engine out.
const (
	defaultLockAttempts = 5
	defaultLockBackoff  = 200 * time.Millisecond
)

// LockProbe describes the outcome of observing the engine's data-dir lock.
type LockProbe struct {
	// LockPath is the probed lock file.
	LockPath string
	// Attempts is the number of non-blocking tries made.
	Attempts int
	// Contended is true when the lock was still held after the last attempt.
	Contended bool
}

// probeDataDirLock observes {dataDir}/crush.lock without ever holding it.
// Crush takes an exclusive flock on the file for the lifetime of a running
// engine (third_party/crush/internal/db/datadirlock.go), and recall must stay
// usable while the engine runs, so contention is reported, never fatal. A
// successful probe releases the lock immediately; recall only mirrors Crush's
// TryFile semantics to learn whether the engine currently owns the lock.
//
// attempts <= 0 or backoff <= 0 select the defaults. Contention after the
// budget is exhausted returns Contended=true with a nil error: the caller
// proceeds with a strictly read-only open, which is safe under WAL.
func probeDataDirLock(ctx context.Context, dataDir string, attempts int, backoff time.Duration) (LockProbe, error) {
	probe := LockProbe{LockPath: filepath.Join(dataDir, dataDirLockFile)}
	if attempts <= 0 {
		attempts = defaultLockAttempts
	}
	if backoff <= 0 {
		backoff = defaultLockBackoff
	}

	// Crush creates the lock file on startup and never unlinks it, so a
	// missing file means no engine process can currently hold the flock.
	if _, err := os.Stat(probe.LockPath); errors.Is(err, os.ErrNotExist) {
		probe.Attempts = 1
		return probe, nil
	} else if err != nil {
		return probe, fmt.Errorf("stat lock file %q: %w", probe.LockPath, err)
	}

	for attempt := 1; attempt <= attempts; attempt++ {
		probe.Attempts = attempt
		f, err := os.OpenFile(probe.LockPath, os.O_RDWR, 0)
		if errors.Is(err, os.ErrNotExist) {
			return probe, nil // released and removed between stat and open
		}
		if err != nil {
			return probe, fmt.Errorf("open lock file %q: %w", probe.LockPath, err)
		}
		release, err := tryLockFile(f)
		if err == nil {
			release()
			_ = f.Close()
			probe.Contended = false
			return probe, nil
		}
		_ = f.Close()
		if !errors.Is(err, errLockContended) {
			return probe, err
		}
		probe.Contended = true
		if attempt == attempts {
			break
		}
		select {
		case <-ctx.Done():
			return probe, fmt.Errorf("probe data-dir lock: %w", ctx.Err())
		case <-time.After(backoff):
		}
	}
	return probe, nil
}

// holdDataDirLock acquires and holds the flock on path. Test helper only:
// the recall server itself never holds the engine's lock.
func holdDataDirLock(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock file %q: %w", path, err)
	}
	release, err := tryLockFile(f)
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	return func() {
		release()
		_ = f.Close()
	}, nil
}
