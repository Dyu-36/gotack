package contextseed

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/Dyu-36/gotack/internal/memory"
)

const (
	promptSnapshotRoot = "context-prompt"
	snapshotPrefix     = "snapshot-"

	// snapshotLayoutVersion is baked into the snapshot identity. Bump it
	// whenever the inclusion, sanitization or rendering policy changes so
	// identities rotate instead of silently reusing stale snapshots.
	snapshotLayoutVersion = 1

	// identityKeyFileName holds the 32-byte HMAC key that derives the
	// content-addressed snapshot identity. It lives beside the snapshot
	// directories (never inside one) and survives pruning.
	identityKeyFileName = ".identity-key"

	// snapshotIdentityDomain separates this HMAC use from every other
	// keyed digest in the product.
	snapshotIdentityDomain = "gotack.context-snapshot-identity.v1"
)

func (s *Seeder) PromptContextRoot() string {
	return filepath.Join(s.dataDir, promptSnapshotRoot)
}

func (s *Seeder) identityKeyPath() string {
	return filepath.Join(s.PromptContextRoot(), identityKeyFileName)
}

// snapshotEntry is one immutable file revision collected from the source
// tree. Bytes are read exactly once and reused for the manifest, the
// staging copy and the publish validation.
type snapshotEntry struct {
	rel   string // slash-separated, case-folded on Windows
	bytes []byte
}

// snapshotManifest is the canonical identity input: layout version,
// migration mode and the ordered (path, content digest) file list. Two
// collections with equal manifest bytes are the same logical snapshot
// regardless of timestamps, mtimes, sizes or staging paths.
type snapshotManifest struct {
	mode    string
	entries []snapshotEntry
}

func (m *snapshotManifest) encode() []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "version=%d\n", snapshotLayoutVersion)
	fmt.Fprintf(&b, "mode=%s\n", m.mode)
	fmt.Fprintf(&b, "files=%d\n", len(m.entries))
	for _, entry := range m.entries {
		digest := sha256.Sum256(entry.bytes)
		fmt.Fprintf(&b, "file=%d:%s\nsha256=%s\n", len(entry.rel), entry.rel, hex.EncodeToString(digest[:]))
	}
	return []byte(b.String())
}

// BuildPromptSnapshot publishes the prompt context as a content-addressed
// immutable revision (ImplementPlan 0.3 / PR2):
//
//  1. Collect every included file's bytes exactly once.
//  2. Derive the identity from an install-key HMAC of the canonical
//     manifest (layout version, migration mode, ordered source-relative
//     paths, per-file content digests). Timestamps, mtimes, sizes and
//     staging paths never enter the identity, so identical content
//     reuses the identical committed directory across refreshes and
//     restarts, and a same-size content edit rotates the identity.
//  3. Validate the staged revision, then atomically rename it into
//     place. A failed refresh removes only its staging directory and
//     leaves the previously committed revision untouched.
//
// If the identity key cannot be loaded or created the refresh fails
// closed; there is no fallback to an unkeyed digest.
func (s *Seeder) BuildPromptSnapshot() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	source := s.ContextDir()
	if info, err := os.Stat(source); err != nil || !info.IsDir() {
		if err == nil {
			err = fmt.Errorf("not a directory")
		}
		return "", fmt.Errorf("context source: %w", err)
	}
	root := s.PromptContextRoot()
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", fmt.Errorf("create prompt snapshot root: %w", err)
	}
	key, err := loadOrCreateSnapshotIdentityKey(s.identityKeyPath())
	if err != nil {
		return "", fmt.Errorf("snapshot identity key: %w", err)
	}

	status, err := s.loadMigrationStatus()
	if err != nil {
		return "", err
	}
	if status.Mode == MigrationStaged {
		status, err = s.recoverStage(status)
		if err != nil {
			return "", fmt.Errorf("recover context migration: %w", err)
		}
	}

	manifest, err := s.collectSnapshot(source, status)
	if err != nil {
		return "", fmt.Errorf("build prompt snapshot: %w", err)
	}

	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(snapshotIdentityDomain))
	mac.Write([]byte{0})
	mac.Write(manifest.encode())
	final := filepath.Join(root, snapshotPrefix+base64.RawURLEncoding.EncodeToString(mac.Sum(nil)))

	if info, err := os.Stat(final); err == nil && info.IsDir() {
		// Identical logical content: reuse the committed immutable
		// revision instead of publishing a duplicate.
		s.retainSnapshot(final)
		return final, nil
	}

	staging, err := os.MkdirTemp(root, ".staging-")
	if err != nil {
		return "", fmt.Errorf("create prompt snapshot: %w", err)
	}
	removeStaging := true
	defer func() {
		if removeStaging {
			_ = os.RemoveAll(staging)
		}
	}()

	for _, entry := range manifest.entries {
		destination := filepath.Join(staging, filepath.FromSlash(entry.rel))
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return "", fmt.Errorf("stage prompt snapshot: %w", err)
		}
		if err := os.WriteFile(destination, entry.bytes, 0o644); err != nil {
			return "", fmt.Errorf("stage prompt snapshot: %w", err)
		}
	}
	if err := validateStagedSnapshot(staging, manifest); err != nil {
		return "", fmt.Errorf("validate prompt snapshot: %w", err)
	}

	if err := os.Rename(staging, final); err != nil {
		// A concurrent publisher may have committed the same identity
		// first; adopt it only when it is a real directory.
		if info, statErr := os.Stat(final); statErr == nil && info.IsDir() {
			s.retainSnapshot(final)
			return final, nil
		}
		return "", fmt.Errorf("commit prompt snapshot: %w", err)
	}
	removeStaging = false
	s.retainSnapshot(final)
	return final, nil
}

// collectSnapshot walks the source tree once and produces the ordered,
// policy-filtered file set that defines the revision. Paths are recorded
// slash-separated and case-folded on Windows (contract v1: the filesystem
// is case-insensitive, so alias casings are one logical path).
func (s *Seeder) collectSnapshot(source string, status MigrationStatus) (*snapshotManifest, error) {
	manifest := &snapshotManifest{mode: string(status.Mode)}
	err := filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if rel == ".seed-report.json" || rel == stockManifestName {
			return nil
		}
		if (status.Mode == MigrationPending || status.Mode == MigrationLegacy || status.Mode == MigrationRolledBack) && (rel == managedCoreName || rel == userContextName) {
			return nil
		}
		if status.Mode == MigrationCommitted && rel == legacyContextName {
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		var bytes []byte
		if isMemoryPath(rel) {
			bytes, err = s.sanitizedMemoryBytes(path, rel)
			if err != nil || bytes == nil {
				return err
			}
		} else {
			bytes, err = os.ReadFile(path)
			if err != nil {
				// Unreadable files never enter the snapshot; the walk
				// continues so one bad file cannot break the revision.
				return nil
			}
		}
		manifest.entries = append(manifest.entries, snapshotEntry{rel: canonicalSnapshotRel(rel), bytes: bytes})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(manifest.entries, func(i, j int) bool {
		return manifest.entries[i].rel < manifest.entries[j].rel
	})
	return manifest, nil
}

func canonicalSnapshotRel(rel string) string {
	if runtime.GOOS == "windows" {
		return strings.ToLower(rel)
	}
	return rel
}

// sanitizedMemoryBytes applies the memory prompt policy to one memory
// file. A nil result (with a nil error) means the file is excluded.
func (s *Seeder) sanitizedMemoryBytes(source, rel string) ([]byte, error) {
	base := filepath.Base(rel)
	var target memory.Target
	switch base {
	case memory.MemoryFileName:
		target = memory.TargetMemory
	case memory.UserFileName:
		target = memory.TargetUser
	default:
		return nil, nil
	}
	data, err := os.ReadFile(source)
	if err != nil {
		s.log.Warn("contextseed: skipping unreadable memory file", "file", rel, "err", err)
		return nil, nil
	}
	content, err := memory.SanitizeFileForPrompt(target, data)
	if err != nil {
		s.log.Warn("contextseed: skipping invalid memory file", "file", rel, "err", err)
		return nil, nil
	}
	if content == "" {
		return nil, nil
	}
	return []byte(content), nil
}

// validateStagedSnapshot re-reads the staged revision and confirms its
// file set matches the collected manifest byte-for-byte before the
// revision is allowed to publish.
func validateStagedSnapshot(staging string, manifest *snapshotManifest) error {
	staged := make(map[string][]byte, len(manifest.entries))
	err := filepath.WalkDir(staging, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(staging, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		staged[canonicalSnapshotRel(filepath.ToSlash(rel))] = data
		return nil
	})
	if err != nil {
		return err
	}
	if len(staged) != len(manifest.entries) {
		return fmt.Errorf("staged file count %d does not match manifest %d", len(staged), len(manifest.entries))
	}
	for _, entry := range manifest.entries {
		data, ok := staged[entry.rel]
		if !ok {
			return fmt.Errorf("staged revision missing file")
		}
		if !bytesEqual(data, entry.bytes) {
			return fmt.Errorf("staged file content does not match manifest")
		}
	}
	return nil
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// retainSnapshot records the committed revision as current and keeps the
// immediately previous one alive for one more prune cycle so a reader
// that is still finishing against the older generation is not pruned
// underneath. Retention is bounded at two generations.
func (s *Seeder) retainSnapshot(final string) {
	if s.currentSnapshot == final {
		return
	}
	s.previousSnapshot = s.currentSnapshot
	s.currentSnapshot = final
}

func isMemoryPath(rel string) bool {
	rel = filepath.Clean(rel)
	memoryPrefix := "memory" + string(filepath.Separator)
	return rel == "memory" || strings.HasPrefix(rel, memoryPrefix)
}

func (s *Seeder) SnapshotOwner() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctxDir := s.ContextDir()
	status, err := s.loadMigrationStatus()
	if err != nil {
		return "none"
	}
	if status.Mode == MigrationPending || status.Mode == MigrationLegacy || status.Mode == MigrationRolledBack {
		if _, err := os.Stat(filepath.Join(ctxDir, legacyContextName)); err == nil {
			return "legacy"
		}
	}
	if _, err := os.Stat(filepath.Join(ctxDir, managedCoreName)); err == nil {
		return "managed"
	}
	return "none"
}

// PrunePromptSnapshots removes superseded snapshot directories. The
// currently committed revision and the immediately previous one are
// always retained (bounded retention for concurrent readers); everything
// else under the snapshot root is removed.
func (s *Seeder) PrunePromptSnapshots(keep string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.PromptContextRoot())
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), snapshotPrefix) {
			continue
		}
		path := filepath.Join(s.PromptContextRoot(), entry.Name())
		if filepath.Clean(path) == filepath.Clean(keep) {
			continue
		}
		if s.previousSnapshot != "" && filepath.Clean(path) == filepath.Clean(s.previousSnapshot) {
			continue
		}
		_ = os.RemoveAll(path)
	}
}

// loadOrCreateSnapshotIdentityKey loads the 32-byte snapshot identity
// key, creating it on first use through an exclusive create plus a
// verified write. A reader of an empty or unparseable key file fails
// closed instead of guessing.
func loadOrCreateSnapshotIdentityKey(path string) ([]byte, error) {
	if data, err := os.ReadFile(path); err == nil {
		return parseSnapshotIdentityKey(data)
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate identity key")
	}
	encoded := []byte(hex.EncodeToString(key))
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		// Another writer may have created it first; adopt it only when
		// it parses as a valid key.
		if data, readErr := os.ReadFile(path); readErr == nil {
			if other, parseErr := parseSnapshotIdentityKey(data); parseErr == nil {
				return other, nil
			}
		}
		return nil, fmt.Errorf("persist identity key")
	}
	return key, nil
}

func parseSnapshotIdentityKey(data []byte) ([]byte, error) {
	key, err := hex.DecodeString(strings.TrimSpace(string(data)))
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("invalid identity key")
	}
	return key, nil
}
