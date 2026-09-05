package contextseed

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// contentIdentityPattern matches a content-addressed snapshot directory
// name: the literal prefix followed by a base64url-encoded HMAC-SHA256
// digest (43 characters). A timestamp-based name can never match because
// decimal digits are not valid base64url alphabet at this length.
var contentIdentityPattern = regexp.MustCompile(`^snapshot-[A-Za-z0-9_-]{43}$`)

func seedContextWithFile(t *testing.T, seeder *Seeder, rel, content string) {
	t.Helper()
	path := filepath.Join(seeder.ContextDir(), filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSnapshotIdentityIsContentAddressedAndReused(t *testing.T) {
	seeder := New(t.TempDir(), nil)
	seedContextWithFile(t, seeder, "TACK_CORE.md", "core policy")

	first, err := seeder.BuildPromptSnapshot()
	if err != nil {
		t.Fatalf("BuildPromptSnapshot = %v", err)
	}
	if !contentIdentityPattern.MatchString(filepath.Base(first)) {
		t.Fatalf("snapshot name %q is not content-addressed", filepath.Base(first))
	}

	second, err := seeder.BuildPromptSnapshot()
	if err != nil {
		t.Fatalf("second BuildPromptSnapshot = %v", err)
	}
	if second != first {
		t.Fatalf("identical content produced a different snapshot: %q vs %q", second, first)
	}
	entries, err := os.ReadDir(seeder.PromptContextRoot())
	if err != nil {
		t.Fatal(err)
	}
	dirs := 0
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), snapshotPrefix) {
			dirs++
		}
	}
	if dirs != 1 {
		t.Fatalf("snapshot root holds %d snapshot dirs, want 1 (immutable reuse)", dirs)
	}
}

func TestSnapshotIdentitySurvivesRestart(t *testing.T) {
	dataDir := t.TempDir()
	first := New(dataDir, nil)
	seedContextWithFile(t, first, "TACK_CORE.md", "core policy")
	before, err := first.BuildPromptSnapshot()
	if err != nil {
		t.Fatalf("BuildPromptSnapshot = %v", err)
	}

	// A restarted process creates a fresh Seeder; the identity key is
	// reloaded from disk, so the same content must produce the same
	// physical snapshot directory.
	second := New(dataDir, nil)
	after, err := second.BuildPromptSnapshot()
	if err != nil {
		t.Fatalf("restarted BuildPromptSnapshot = %v", err)
	}
	if after != before {
		t.Fatalf("restart changed snapshot identity: %q vs %q", before, after)
	}
}

func TestSnapshotSameSizeEditChangesIdentityOnce(t *testing.T) {
	seeder := New(t.TempDir(), nil)
	seedContextWithFile(t, seeder, "TACK_CORE.md", "ABCD")

	before, err := seeder.BuildPromptSnapshot()
	if err != nil {
		t.Fatalf("BuildPromptSnapshot = %v", err)
	}
	// Same byte count, different content: the identity must change.
	seedContextWithFile(t, seeder, "TACK_CORE.md", "WXYZ")
	after, err := seeder.BuildPromptSnapshot()
	if err != nil {
		t.Fatalf("same-size edit BuildPromptSnapshot = %v", err)
	}
	if after == before {
		t.Fatal("same-size content edit did not change the snapshot identity")
	}
	again, err := seeder.BuildPromptSnapshot()
	if err != nil {
		t.Fatalf("rebuild BuildPromptSnapshot = %v", err)
	}
	if again != after {
		t.Fatalf("unchanged content drifted again: %q vs %q", again, after)
	}
	if _, err := os.Stat(before); err != nil {
		t.Fatalf("previous committed revision was removed by refresh: %v", err)
	}
}

func TestSnapshotIdentityIgnoresWindowsPathCasing(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("NTFS case-insensitive aliasing contract is Windows-only")
	}
	dataDir := t.TempDir()
	seeder := New(dataDir, nil)
	seedContextWithFile(t, seeder, "Notes/Guide.md", "guide body")
	before, err := seeder.BuildPromptSnapshot()
	if err != nil {
		t.Fatalf("BuildPromptSnapshot = %v", err)
	}

	// Case-only rename of a directory must not change the identity: the
	// filesystem is case-insensitive under contract v1.
	source := seeder.ContextDir()
	if err := os.Rename(filepath.Join(source, "Notes"), filepath.Join(source, "notes")); err != nil {
		t.Fatalf("case-only rename failed: %v", err)
	}
	after, err := seeder.BuildPromptSnapshot()
	if err != nil {
		t.Fatalf("post-rename BuildPromptSnapshot = %v", err)
	}
	if after != before {
		t.Fatalf("case-only directory rename changed snapshot identity: %q vs %q", before, after)
	}

	// A differently-cased data-dir alias must resolve to the same
	// identity name (the snapshot root itself moves with the data dir,
	// but the engine folds rendered path case on Windows, so the
	// invariant that matters is the directory identity).
	alias := New(strings.ToUpper(dataDir), nil)
	fromAlias, err := alias.BuildPromptSnapshot()
	if err != nil {
		t.Fatalf("alias BuildPromptSnapshot = %v", err)
	}
	if filepath.Base(fromAlias) != filepath.Base(before) {
		t.Fatalf("data-dir casing alias changed snapshot identity: %q vs %q", fromAlias, before)
	}
}

func TestFailedSnapshotRefreshKeepsCommittedRevision(t *testing.T) {
	seeder := New(t.TempDir(), nil)
	seedContextWithFile(t, seeder, "TACK_CORE.md", "committed policy")
	committed, err := seeder.BuildPromptSnapshot()
	if err != nil {
		t.Fatalf("BuildPromptSnapshot = %v", err)
	}

	// A failing refresh (source removed) must leave the previously
	// committed revision fully intact and reusable.
	if err := os.RemoveAll(seeder.ContextDir()); err != nil {
		t.Fatal(err)
	}
	if _, err := seeder.BuildPromptSnapshot(); err == nil {
		t.Fatal("BuildPromptSnapshot succeeded after the source directory was removed")
	}
	got, err := os.ReadFile(filepath.Join(committed, "TACK_CORE.md"))
	if err != nil {
		t.Fatalf("committed revision unavailable after failed refresh: %v", err)
	}
	if string(got) != "committed policy" {
		t.Fatalf("committed revision content changed: %q", got)
	}
}

func TestSnapshotIdentityKeyFailureIsFailClosed(t *testing.T) {
	seeder := New(t.TempDir(), nil)
	seedContextWithFile(t, seeder, "TACK_CORE.md", "policy")
	committed, err := seeder.BuildPromptSnapshot()
	if err != nil {
		t.Fatalf("BuildPromptSnapshot = %v", err)
	}

	// Corrupt the identity key: the refresh must fail closed instead of
	// falling back to an unkeyed hash, and the committed revision must
	// stay untouched.
	keyPath := seeder.identityKeyPath()
	if err := os.WriteFile(keyPath, []byte("not-hex"), 0o600); err != nil {
		t.Fatal(err)
	}
	seedContextWithFile(t, seeder, "TACK_CORE.md", "changed policy")
	if _, err := seeder.BuildPromptSnapshot(); err == nil {
		t.Fatal("BuildPromptSnapshot fell back to an unkeyed identity after key corruption")
	}
	if got := string(mustRead(t, filepath.Join(committed, "TACK_CORE.md"))); got != "policy" {
		t.Fatalf("committed revision content changed: %q", got)
	}
}

func TestSnapshotRetentionKeepsPreviousGeneration(t *testing.T) {
	seeder := New(t.TempDir(), nil)
	seedContextWithFile(t, seeder, "TACK_CORE.md", "one")
	first, err := seeder.BuildPromptSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	seedContextWithFile(t, seeder, "TACK_CORE.md", "two")
	second, err := seeder.BuildPromptSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	// The generation a concurrent run may still reference survives one
	// prune cycle.
	seeder.PrunePromptSnapshots(second)
	if _, err := os.Stat(first); err != nil {
		t.Fatalf("previous generation pruned too early: %v", err)
	}
	if _, err := os.Stat(second); err != nil {
		t.Fatalf("current generation pruned: %v", err)
	}

	seedContextWithFile(t, seeder, "TACK_CORE.md", "three")
	third, err := seeder.BuildPromptSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	seeder.PrunePromptSnapshots(third)
	if _, err := os.Stat(first); !os.IsNotExist(err) {
		t.Fatalf("oldest generation was not bounded: %v", err)
	}
	if _, err := os.Stat(second); err != nil {
		t.Fatalf("previous generation pruned early: %v", err)
	}
	if _, err := os.Stat(third); err != nil {
		t.Fatalf("current generation pruned: %v", err)
	}
}
