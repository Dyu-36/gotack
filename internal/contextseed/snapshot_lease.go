package contextseed

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const snapshotLeaseDirName = ".leases"
const snapshotRegistryLockName = ".registry.lock"

type SnapshotLease struct {
	generation string
	path       string
	file       *os.File
	once       sync.Once
	err        error
}

func (l *SnapshotLease) Generation() string {
	if l == nil {
		return ""
	}
	return l.generation
}

func (l *SnapshotLease) Release() error {
	if l == nil {
		return nil
	}
	l.once.Do(func() {
		if l.file == nil {
			return
		}
		if err := unlockSnapshotFile(l.file); err != nil {
			l.err = err
		}
		if err := l.file.Close(); err != nil && l.err == nil {
			l.err = err
		}
		l.file = nil
	})
	return l.err
}

func (s *Seeder) snapshotLeaseDir() string {
	return filepath.Join(s.PromptContextRoot(), snapshotLeaseDirName)
}

func (s *Seeder) withSnapshotRegistryLock(fn func() error) error {
	if err := os.MkdirAll(s.snapshotLeaseDir(), 0o700); err != nil {
		return fmt.Errorf("create snapshot lease metadata: %w", err)
	}
	path := filepath.Join(s.snapshotLeaseDir(), snapshotRegistryLockName)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("open snapshot registry lock: %w", err)
	}
	defer f.Close()
	if err := lockSnapshotFileExclusive(f); err != nil {
		return fmt.Errorf("lock snapshot registry: %w", err)
	}
	defer func() { _ = unlockSnapshotFile(f) }()
	return fn()
}

func (s *Seeder) acquireGenerationLeaseLocked(generation string) (*SnapshotLease, error) {
	root, err := filepath.Abs(s.PromptContextRoot())
	if err != nil {
		return nil, err
	}
	generation, err = filepath.Abs(generation)
	if err != nil {
		return nil, err
	}
	relative, err := filepath.Rel(root, generation)
	if err != nil || filepath.Dir(relative) != "." || !strings.HasPrefix(filepath.Base(generation), snapshotPrefix) {
		return nil, fmt.Errorf("snapshot lease generation is outside prompt snapshot root")
	}
	info, err := os.Lstat(generation)
	if err != nil {
		return nil, fmt.Errorf("inspect snapshot lease generation: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("snapshot lease generation must be a real directory")
	}
	if err := os.MkdirAll(s.snapshotLeaseDir(), 0o700); err != nil {
		return nil, fmt.Errorf("create snapshot lease metadata: %w", err)
	}
	path := filepath.Join(s.snapshotLeaseDir(), filepath.Base(generation)+".lock")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open snapshot generation lease: %w", err)
	}
	if err := lockSnapshotFileShared(f); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("lock snapshot generation lease: %w", err)
	}
	return &SnapshotLease{generation: generation, path: path, file: f}, nil
}

// BuildPromptSnapshotLease selects/publishes a generation and acquires its
// cross-process lease while holding the same registry exclusion used by prune.
// This removes the select-then-prune gap.
func (s *Seeder) BuildPromptSnapshotLease() (string, *SnapshotLease, error) {
	var generation string
	var lease *SnapshotLease
	err := s.withSnapshotRegistryLock(func() error {
		var err error
		generation, err = s.BuildPromptSnapshot()
		if err != nil {
			return err
		}
		lease, err = s.acquireGenerationLeaseLocked(generation)
		return err
	})
	if err != nil {
		return "", nil, err
	}
	return generation, lease, nil
}

func (s *Seeder) AcquireSnapshotLease(generation string) (*SnapshotLease, error) {
	var lease *SnapshotLease
	err := s.withSnapshotRegistryLock(func() error {
		var err error
		lease, err = s.acquireGenerationLeaseLocked(generation)
		return err
	})
	return lease, err
}
