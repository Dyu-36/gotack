package contextseed

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/Dyu-36/gotack/internal/assistant"
	"github.com/Dyu-36/gotack/internal/memory"
)

const (
	promptSnapshotRoot     = "context-prompt"
	snapshotPrefix         = "snapshot-"
	snapshotLayoutVersion  = 2
	identityKeyFileName    = ".identity-key"
	snapshotIdentityDomain = "gotack.context-snapshot-identity.v1"
	maxCoreBytes           = 4096
	maxPersistentBytes     = 24 * 1024
)

var beforeValidateStagedSnapshot func(staging string)
var beforeCollectSnapshot func(source string)
var beforeReadSnapshotFile func(path, rel string) error

type PromptStats struct {
	LayoutVersion int   `json:"layout_version"`
	Files         int   `json:"files"`
	PayloadBytes  int   `json:"payload_bytes"`
	MaxBytes      int   `json:"max_bytes"`
	ProfileChars  int   `json:"profile_chars"`
	MemoryChars   int   `json:"memory_chars"`
	Omitted       int   `json:"omitted_entries"`
	BuildMicros   int64 `json:"build_micros"`
	Reused        bool  `json:"reused"`
}

func (s *Seeder) PromptStats() PromptStats {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stats
}

func (s *Seeder) PromptContextRoot() string {
	return filepath.Join(s.dataDir, promptSnapshotRoot)
}

func (s *Seeder) identityKeyPath() string {
	return filepath.Join(s.PromptContextRoot(), identityKeyFileName)
}

type snapshotEntry struct {
	rel   string
	bytes []byte
}

type snapshotManifest struct {
	mode    string
	entries []snapshotEntry
	stats   PromptStats
}

func (m *snapshotManifest) encode() []byte {
	var out strings.Builder
	fmt.Fprintf(&out, "version=%d\nmode=%s\nfiles=%d\n", snapshotLayoutVersion, m.mode, len(m.entries))
	for _, entry := range m.entries {
		digest := sha256.Sum256(entry.bytes)
		fmt.Fprintf(&out, "file=%d:%s\nsha256=%s\n", len(entry.rel), entry.rel, hex.EncodeToString(digest[:]))
	}
	return []byte(out.String())
}

// BuildPromptSnapshot publishes exactly embedded core + bounded profile + bounded
// memory. Archive size and unrelated files cannot increase the prompt surface.
// Identity, staged validation, atomic publication and reader leases are retained.
func (s *Seeder) BuildPromptSnapshot() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	started := time.Now()
	source := s.ContextDir()
	if info, err := os.Stat(source); err != nil || !info.IsDir() {
		return "", fmt.Errorf("assistant context directory is unavailable: %s", source)
	}
	root := s.PromptContextRoot()
	if err := os.MkdirAll(root, 0o700); err != nil {
		return "", err
	}
	key, err := loadOrCreateSnapshotIdentityKey(s.identityKeyPath())
	if err != nil {
		return "", fmt.Errorf("snapshot identity key: %w", err)
	}
	if beforeCollectSnapshot != nil {
		beforeCollectSnapshot(source)
	}
	manifest, err := s.collectSnapshot(source)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(snapshotIdentityDomain))
	mac.Write([]byte{0})
	mac.Write(manifest.encode())
	final := filepath.Join(root, snapshotPrefix+base64.RawURLEncoding.EncodeToString(mac.Sum(nil)))
	if _, err := os.Lstat(final); err == nil {
		if err := validateStagedSnapshot(final, manifest); err != nil {
			return "", fmt.Errorf("validate committed prompt snapshot: %w", err)
		}
		s.recordSnapshot(final, manifest, started, true)
		return final, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	staging, err := os.MkdirTemp(root, ".staging-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(staging)
	for _, entry := range manifest.entries {
		destination := filepath.Join(staging, filepath.FromSlash(entry.rel))
		if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
			return "", err
		}
		if err := os.WriteFile(destination, entry.bytes, 0o600); err != nil {
			return "", err
		}
	}
	if beforeValidateStagedSnapshot != nil {
		beforeValidateStagedSnapshot(staging)
	}
	if err := validateStagedSnapshot(staging, manifest); err != nil {
		return "", fmt.Errorf("validate staged prompt snapshot: %w", err)
	}
	if err := os.Rename(staging, final); err != nil {
		// Another process may have published the same content while we staged.
		if validateErr := validateStagedSnapshot(final, manifest); validateErr != nil {
			return "", fmt.Errorf("commit prompt snapshot: %w", err)
		}
	}
	s.recordSnapshot(final, manifest, started, false)
	return final, nil
}

func (s *Seeder) collectSnapshot(source string) (*snapshotManifest, error) {
	if len(assistant.CorePrompt) == 0 || len(assistant.CorePrompt) > maxCoreBytes {
		return nil, errors.New("embedded assistant core is empty or exceeds its byte budget")
	}
	manifest := &snapshotManifest{
		mode:    "assistant",
		entries: []snapshotEntry{{rel: canonicalSnapshotRel(managedCoreName), bytes: []byte(assistant.CorePrompt)}},
		stats:   PromptStats{LayoutVersion: snapshotLayoutVersion, MaxBytes: maxPersistentBytes},
	}
	for _, target := range []memory.Target{memory.TargetUser, memory.TargetMemory} {
		name := memory.FileNameFor(target)
		path := filepath.Join(source, name)
		rel := "memory/" + name
		data, err := memory.ReadPromptFile(path)
		if errors.Is(err, memory.ErrPromptFileTooLarge) {
			s.log.Warn("assistant: oversized memory file excluded; original preserved", "file", name)
			manifest.stats.Omitted++
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read assistant context %s: %w", name, err)
		}
		if beforeReadSnapshotFile != nil {
			if err := beforeReadSnapshotFile(path, rel); err != nil {
				return nil, err
			}
		}
		text, omitted, err := memory.BoundedPrompt(target, data)
		if err != nil {
			s.log.Warn("assistant: invalid memory file excluded; original preserved", "file", name, "err", err)
			manifest.stats.Omitted++
			continue
		}
		manifest.stats.Omitted += omitted
		if target == memory.TargetUser {
			manifest.stats.ProfileChars = memory.PromptBodyChars(text)
		} else {
			manifest.stats.MemoryChars = memory.PromptBodyChars(text)
		}
		if text != "" {
			manifest.entries = append(manifest.entries, snapshotEntry{rel: canonicalSnapshotRel(rel), bytes: []byte(text)})
		}
	}
	sort.Slice(manifest.entries, func(i, j int) bool { return manifest.entries[i].rel < manifest.entries[j].rel })
	for _, entry := range manifest.entries {
		manifest.stats.PayloadBytes += len(entry.bytes)
	}
	manifest.stats.Files = len(manifest.entries)
	if manifest.stats.PayloadBytes > maxPersistentBytes {
		return nil, errors.New("assistant context exceeds its total byte budget")
	}
	return manifest, nil
}

func (s *Seeder) recordSnapshot(path string, manifest *snapshotManifest, started time.Time, reused bool) {
	s.retainSnapshot(path)
	s.stats = manifest.stats
	s.stats.BuildMicros = time.Since(started).Microseconds()
	s.stats.Reused = reused
	// Only counts/timings; never record memory contents or their hashes in logs.
	s.log.Debug("assistant context budget", "bytes", s.stats.PayloadBytes,
		"profile_chars", s.stats.ProfileChars, "memory_chars", s.stats.MemoryChars,
		"omitted", s.stats.Omitted, "build_us", s.stats.BuildMicros, "reused", reused)
}

func canonicalSnapshotRel(rel string) string {
	if runtime.GOOS == "windows" {
		return strings.ToLower(rel)
	}
	return rel
}

// Walking the generated, bounded output here is integrity validation, not source
// discovery. No new file is admitted to a prompt by this walk.
func validateStagedSnapshot(staging string, manifest *snapshotManifest) error {
	info, err := os.Lstat(staging)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("snapshot root must be a real directory")
	}
	expected := make(map[string][]byte, len(manifest.entries))
	for _, entry := range manifest.entries {
		expected[entry.rel] = entry.bytes
	}
	seen := make(map[string]bool, len(expected))
	err = filepath.WalkDir(staging, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("snapshot contains a link")
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return errors.New("snapshot contains a non-regular file")
		}
		rel, err := filepath.Rel(staging, path)
		if err != nil {
			return err
		}
		rel = canonicalSnapshotRel(filepath.ToSlash(rel))
		want, ok := expected[rel]
		if !ok || seen[rel] {
			return fmt.Errorf("snapshot contains unexpected/duplicate file %q", rel)
		}
		data, err := memory.ReadPromptFile(path)
		if err != nil {
			return err
		}
		if !bytesEqual(data, want) {
			return fmt.Errorf("snapshot file %q differs from committed input", rel)
		}
		seen[rel] = true
		return nil
	})
	if err != nil {
		return err
	}
	if len(seen) != len(expected) {
		return errors.New("snapshot is missing an expected file")
	}
	return nil
}

func bytesEqual(left, right []byte) bool { return bytes.Equal(left, right) }

func (s *Seeder) retainSnapshot(final string) {
	if s.currentSnapshot != final {
		s.previousSnapshot = s.currentSnapshot
		s.currentSnapshot = final
	}
}

func (s *Seeder) SnapshotOwner() string {
	if s == nil {
		return "none"
	}
	if info, err := os.Stat(s.ContextDir()); err != nil || !info.IsDir() {
		return "none"
	}
	return "managed"
}

func (s *Seeder) PrunePromptSnapshots(keep string) { _ = s.PrunePromptSnapshotsChecked(keep) }

// Keep reader/lease retention intact: a short prompt is not a reason to delete a
// generation still being consumed by a foreground or remote run.
func (s *Seeder) PrunePromptSnapshotsChecked(keep string) error {
	return s.withSnapshotRegistryLock(func() error {
		s.mu.Lock()
		defer s.mu.Unlock()
		entries, err := os.ReadDir(s.PromptContextRoot())
		if err != nil {
			return err
		}
		protected := make(map[string]bool, len(s.snapshotReaders)+3)
		for _, path := range []string{keep, s.currentSnapshot, s.previousSnapshot} {
			if path != "" {
				protected[filepath.Clean(path)] = true
			}
		}
		for _, path := range s.snapshotReaders {
			if path != "" {
				protected[filepath.Clean(path)] = true
			}
		}
		for _, entry := range entries {
			if !entry.IsDir() || !strings.HasPrefix(entry.Name(), snapshotPrefix) {
				continue
			}
			path := filepath.Join(s.PromptContextRoot(), entry.Name())
			if protected[filepath.Clean(path)] {
				continue
			}
			leasePath := filepath.Join(s.snapshotLeaseDir(), entry.Name()+".lock")
			file, err := os.OpenFile(leasePath, os.O_CREATE|os.O_RDWR, 0o600)
			if err != nil {
				return err
			}
			locked, lockErr := tryLockSnapshotFileExclusive(file)
			if lockErr != nil || !locked {
				_ = file.Close()
				if lockErr != nil {
					return lockErr
				}
				continue
			}
			removeErr := os.RemoveAll(path)
			unlockErr := unlockSnapshotFile(file)
			closeErr := file.Close()
			if err := errors.Join(removeErr, unlockErr, closeErr); err != nil {
				return err
			}
			if err := os.Remove(leasePath); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
		return nil
	})
}
