package contextseed

import (
	"crypto/hmac"
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

// beforeValidateStagedSnapshot is a test-only injection point invoked
// from BuildPromptSnapshot after the staged tree is fully written
// and before validateStagedSnapshot runs. Production callers leave
// it nil. Tests register a hook to mutate the staged bytes (extra
// file, modified byte, dropped file) so the validate path can be
// exercised without race-prone polling. The variable is unexported on
// purpose; only tests in this package can set it.
var beforeValidateStagedSnapshot func(staging string)

// beforeCollectSnapshot is a test-only injection point invoked
// just before collectSnapshot walks the source tree. Tests use it
// to drop a source file (or otherwise invalidate one) so the read
// path actually fails. Production callers leave it nil. The
// variable is unexported on purpose; only tests in this package
// can set it.
var beforeCollectSnapshot func(source string)
var beforeReadSnapshotFile func(path, rel string) error

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
// immutable revision (PR2 content-addressed prompt contract):
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
	// beforeCollectSnapshot is a test-only injection point invoked
	// just before collectSnapshot walks the source tree. Production
	// callers leave it nil; tests register a hook to drop a source
	// file (or otherwise invalidate one) so the read path actually
	// fails. The hook is the only way to deterministically exercise
	// the per-file fail-closed branch without racing the walk.
	if beforeCollectSnapshot != nil {
		beforeCollectSnapshot(source)
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
	// beforeValidateStagedSnapshot is a test-only injection point
	// between staging the manifest and validating it. Production
	// callers leave it nil; tests register a hook to mutate the
	// staged bytes (extra file, modified byte, dropped file) so the
	// validate path can be exercised without race-prone polling.
	if beforeValidateStagedSnapshot != nil {
		beforeValidateStagedSnapshot(staging)
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
//
// Regular context files (non-memory) fail closed on read errors: a
// missing or unreadable source file aborts the snapshot so a partial
// manifest is never published. Memory files retain the documented
// per-file exclusion policy because their sanitization rules are an
// intentional part of the prompt contract, not a transient read
// failure. Either failure mode leaves the previously committed
// revision untouched because the staging directory is only renamed
// after the manifest validates.
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
		if entry.Type()&os.ModeSymlink != 0 {
			// filepath.WalkDir does not follow symlinks, but on
			// Windows a directory symlink reports IsDir() == false
			// via os.DirEntry. Resolve the link: if the target is a
			// directory, skip (do not enter); otherwise the
			// upcoming os.ReadFile will follow the link and pick up
			// the target's content.
			info, statErr := os.Stat(path)
			if statErr != nil || info.IsDir() {
				return nil
			}
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
				// Context files fail closed: a transient read error
				// must never produce a partial manifest that the
				// publisher then commits.
				return fmt.Errorf("read context file %s: %w", rel, err)
			}
		}
		if beforeReadSnapshotFile != nil {
			if abort := beforeReadSnapshotFile(path, rel); abort != nil {
				return abort
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

// validateStagedSnapshot re-reads the staged revision and confirms
// its file set matches the collected manifest byte-for-byte before
// the revision is allowed to publish. The check is defense-in-depth
// on top of the HMAC identity: every staged file's SHA-256 must
// equal the digest captured during collectSnapshot. A mismatch is a
// hard error and the previously committed revision stays intact (the
// staging directory is removed by BuildPromptSnapshot's deferred
// cleanup).

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
		return fmt.Errorf("staged file count %d does not match manifest %d (extra=%d missing=%d)", len(staged), len(manifest.entries), len(staged)-len(manifest.entries), len(manifest.entries)-len(staged))
	}
	for _, entry := range manifest.entries {
		data, ok := staged[entry.rel]
		if !ok {
			return fmt.Errorf("staged revision missing file %q", entry.rel)
		}
		if !bytesEqual(data, entry.bytes) {
			return fmt.Errorf("staged file %q content does not match manifest", entry.rel)
		}
		stagedDigest := sha256.Sum256(data)
		manifestDigest := sha256.Sum256(entry.bytes)
		if stagedDigest != manifestDigest {
			return fmt.Errorf("staged file %q digest %x does not match manifest digest %x", entry.rel, stagedDigest, manifestDigest)
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

// retainSnapshot records the committed revision. The retention rule
// has moved off an arbitrary cap and onto the actual reader set (see
// PrunePromptSnapshots). retainSnapshot keeps a single
// previously-committed generation reference so a stale `keep`
// argument cannot accidentally evict the most recent committed
// revision; the prune step is what enforces the reader-set bound.
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

// PrunePromptSnapshots removes superseded snapshot directories.
// Retention follows the actual reader set: every generation still
// referenced by a live (workspace, run) reader survives, plus the
// `keep` argument (the freshly returned path from
// BuildPromptSnapshot) which is treated as the currently committed
// revision. There is no arbitrary two-generation cap on retention;
// the previous committed revision is retained as a safety floor so
// a reader that has not yet registered through AcquireReader still
// has its generation alive for one prune cycle. The cardinality is
// bounded by the number of distinct reader holders at any moment;
// releasing a reader through ReleaseReader makes that generation
// prunable on the next call.
func (s *Seeder) PrunePromptSnapshots(keep string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.PromptContextRoot())
	if err != nil {
		return
	}
	protected := make(map[string]struct{}, len(s.snapshotReaders)+3)
	if keep != "" {
		protected[filepath.Clean(keep)] = struct{}{}
	}
	if s.currentSnapshot != "" {
		protected[filepath.Clean(s.currentSnapshot)] = struct{}{}
	}
	if s.previousSnapshot != "" {
		protected[filepath.Clean(s.previousSnapshot)] = struct{}{}
	}
	for _, gen := range s.snapshotReaders {
		if gen == "" {
			continue
		}
		protected[filepath.Clean(gen)] = struct{}{}
	}
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), snapshotPrefix) {
			continue
		}
		path := filepath.Join(s.PromptContextRoot(), entry.Name())
		if _, ok := protected[filepath.Clean(path)]; ok {
			continue
		}
		_ = os.RemoveAll(path)
	}
}
