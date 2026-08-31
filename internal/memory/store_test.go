package memory

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fixedClock returns a now-func that yields the given times in order, so
// provenance timestamps in tests are deterministic.
func fixedClock(t *testing.T, times ...time.Time) func() time.Time {
	t.Helper()
	index := 0
	return func() time.Time {
		if index >= len(times) {
			t.Fatalf("fixed clock exhausted after %d reads", len(times))
		}
		value := times[index]
		index++
		return value
	}
}

func newTestStore(t *testing.T, session string) *Store {
	t.Helper()
	return NewStore(t.TempDir(), session)
}

func mustAdd(t *testing.T, store *Store, target Target, section, content string) Result {
	t.Helper()
	result, err := store.Add(context.Background(), target, section, content)
	if err != nil {
		t.Fatalf("add(%s/%s): %v", target, section, err)
	}
	return result
}

func TestAddAndViewRoundTrip(t *testing.T) {
	store := newTestStore(t, "sess-1")
	store.now = fixedClock(t, time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC))

	added := mustAdd(t, store, TargetMemory, "Project facts", "The build uses pnpm.")
	if added.File != MemoryFileName || added.Target != "memory" {
		t.Fatalf("result file/target = %+v", added)
	}
	if added.Cap != MemoryCap {
		t.Fatalf("cap = %d, want %d", added.Cap, MemoryCap)
	}

	view, err := store.View(context.Background(), TargetMemory)
	if err != nil {
		t.Fatalf("view: %v", err)
	}
	if !strings.Contains(view.Content, SectionMarker+" Project facts") {
		t.Fatalf("view lacks section: %q", view.Content)
	}
	if !strings.Contains(view.Content, provenanceStamp("sess-1", "2026-09-01T10:00:00Z")) {
		t.Fatalf("view lacks provenance stamp: %q", view.Content)
	}
	if view.Size != len(view.Content) || view.Remaining != MemoryCap-view.Size {
		t.Fatalf("budget arithmetic wrong: %+v", view)
	}
	onDisk, err := os.ReadFile(store.Path(TargetMemory))
	if err != nil {
		t.Fatalf("read persisted file: %v", err)
	}
	if string(onDisk) != view.Content {
		t.Fatalf("disk and view diverged:\n%s\n---\n%s", onDisk, view.Content)
	}
}

// TestCrossSessionPersistence is the executable stand-in for the Phase 2
// outcome proof: a fact stated by one server instance (session A) is
// visible to a fresh instance (session B) over the same directory, with
// both writers' provenance preserved.
func TestCrossSessionPersistence(t *testing.T) {
	dir := t.TempDir()
	sessionA := NewStore(dir, "sess-A")
	mustAdd(t, sessionA, TargetMemory, "Facts", "The deploy key lives in the vault.")

	sessionB := NewStore(dir, "sess-B")
	view, err := sessionB.View(context.Background(), TargetMemory)
	if err != nil {
		t.Fatalf("session B view: %v", err)
	}
	if !strings.Contains(view.Content, "The deploy key lives in the vault.") {
		t.Fatalf("session A fact not visible to session B:\n%s", view.Content)
	}
	if !strings.Contains(view.Content, "session=sess-A") {
		t.Fatalf("session A provenance lost:\n%s", view.Content)
	}

	mustAdd(t, sessionB, TargetMemory, "Facts", "Rotated quarterly.")
	final, err := sessionA.View(context.Background(), TargetMemory)
	if err != nil {
		t.Fatalf("session A re-view: %v", err)
	}
	if !strings.Contains(final.Content, "session=sess-A") || !strings.Contains(final.Content, "session=sess-B") {
		t.Fatalf("both writers must stay traceable:\n%s", final.Content)
	}
}

func TestViewEmptyStore(t *testing.T) {
	store := newTestStore(t, "sess-1")
	for _, target := range []Target{TargetMemory, TargetUser} {
		view, err := store.View(context.Background(), target)
		if err != nil {
			t.Fatalf("view %s: %v", target, err)
		}
		if view.Content != "" || view.Size != 0 || view.Evicted != 0 {
			t.Fatalf("empty view not neutral: %+v", view)
		}
		if view.Remaining != view.Cap {
			t.Fatalf("empty view remaining = %d, want cap %d", view.Remaining, view.Cap)
		}
	}
}

func TestUserTargetUsesOwnFileAndCap(t *testing.T) {
	store := newTestStore(t, "sess-1")
	added := mustAdd(t, store, TargetUser, "Preferences", "Concise answers.")
	if added.File != UserFileName || added.Cap != UserCap {
		t.Fatalf("user result = %+v", added)
	}
	if _, err := os.Stat(store.Path(TargetMemory)); !os.IsNotExist(err) {
		t.Fatal("user write must not create MEMORY.md")
	}
}

// TestCapEnforcementEvictsOldest pins the documented policy: when a write
// would exceed the cap, the oldest entries are evicted until it fits, the
// newest entry always survives, and surviving entries keep provenance.
func TestCapEnforcementEvictsOldest(t *testing.T) {
	store := newTestStore(t, "sess-x")
	base := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	store.now = fixedClock(t, base, base.Add(time.Hour), base.Add(2*time.Hour))

	mustAdd(t, store, TargetMemory, "Facts", strings.Repeat("a", 1400))         // session sess-x @ 10:00
	mustAdd(t, store, TargetMemory, "Facts", strings.Repeat("b", 600))          // @ 11:00
	third := mustAdd(t, store, TargetMemory, "Facts", strings.Repeat("c", 400)) // @ 12:00

	if third.Size > MemoryCap {
		t.Fatalf("post-eviction size %d exceeds cap %d", third.Size, MemoryCap)
	}
	if third.Evicted != 1 {
		t.Fatalf("evicted = %d, want 1 (the oldest entry)", third.Evicted)
	}
	view, _ := store.View(context.Background(), TargetMemory)
	if strings.Contains(view.Content, strings.Repeat("a", 100)) {
		t.Fatalf("oldest entry survived eviction:\n%s", view.Content)
	}
	if !strings.Contains(view.Content, strings.Repeat("b", 600)) || !strings.Contains(view.Content, strings.Repeat("c", 400)) {
		t.Fatalf("newer entries lost:\n%s", view.Content)
	}
	// Provenance is preserved for every surviving entry.
	parsed := parseFile(view.Content)
	stamps := 0
	for _, section := range parsed.Sections {
		for _, entry := range section.Entries {
			if !entry.stamped() {
				t.Fatalf("surviving entry lost provenance: %+v", entry)
			}
			stamps++
		}
	}
	if stamps != 2 {
		t.Fatalf("stamps = %d, want 2 surviving entries", stamps)
	}
}

// TestEvictionOrdersByTimestamp pins that eviction follows provenance time,
// not file position: an entry stamped earlier is picked first even when it
// sits later in the file, and the keep entry is never picked.
func TestEvictionOrdersByTimestamp(t *testing.T) {
	newer := "2026-09-01T11:00:00Z"
	older := "2026-09-01T10:00:00Z"
	file := File{Sections: []Section{
		{Heading: "A", Entries: []Entry{
			{Session: "s", At: newer, Lines: []string{"written first, stamped later"}},
		}},
		{Heading: "B", Entries: []Entry{
			{Session: "s", At: older, Lines: []string{"written last, stamped earlier"}},
			{Session: "s", At: newer, Lines: []string{"the keep entry"}},
		}},
	}}

	// Keep is section 1 entry 1: the oldest stamped entry (section 1,
	// entry 0) must be chosen, not the file-first section 0 entry.
	si, ei, found := oldestEntry(&file, 1, 1)
	if !found || si != 1 || ei != 0 {
		t.Fatalf("oldestEntry = (%d,%d,%v), want (1,0,true)", si, ei, found)
	}

	// Equal stamps fall back to file position: drop the older stamp.
	file.Sections[1].Entries[0].At = newer
	si, ei, found = oldestEntry(&file, 1, 1)
	if !found || si != 0 || ei != 0 {
		t.Fatalf("tie-break by position = (%d,%d,%v), want (0,0,true)", si, ei, found)
	}

	// Only the keep entry left: no victim.
	file.Sections = file.Sections[1:]
	file.Sections[0].Entries = file.Sections[0].Entries[1:]
	if _, _, found := oldestEntry(&file, 0, 0); found {
		t.Fatal("keep-only file must have no eviction victim")
	}
}

// TestCapExceededRejectsSingleOversizedEntry pins the other half of the cap
// policy: an entry that alone exceeds the cap is rejected with an error
// naming the cap, and nothing is persisted.
func TestCapExceededRejectsSingleOversizedEntry(t *testing.T) {
	store := newTestStore(t, "sess-x")

	_, err := store.Add(context.Background(), TargetMemory, "Facts", strings.Repeat("z", MemoryCap+1))
	if !errors.Is(err, ErrCapExceeded) {
		t.Fatalf("err = %v, want ErrCapExceeded", err)
	}
	if !strings.Contains(err.Error(), "2200") {
		t.Fatalf("error must name the cap: %v", err)
	}
	if _, statErr := os.Stat(store.Path(TargetMemory)); !os.IsNotExist(statErr) {
		t.Fatal("rejected write must not create the file")
	}

	// Same rule against a pre-existing file: the old content stays intact.
	mustAdd(t, store, TargetMemory, "Facts", "original")
	before, _ := os.ReadFile(store.Path(TargetMemory))
	_, err = store.Add(context.Background(), TargetMemory, "Facts", strings.Repeat("z", MemoryCap+1))
	if !errors.Is(err, ErrCapExceeded) {
		t.Fatalf("second oversized add err = %v", err)
	}
	after, _ := os.ReadFile(store.Path(TargetMemory))
	if string(before) != string(after) {
		t.Fatal("rejected write modified the file")
	}
}

// TestEvictionKeepsUnstampedContentLast pins the order between stamped and
// hand-edited (unstamped) entries: unstamped entries predate recorded
// history and are evicted first.
func TestEvictionKeepsUnstampedContentLast(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir, "sess-x")
	store.now = fixedClock(t, time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC))

	// Hand-edited file: one unstamped entry, then one stamped entry.
	raw := "§ Facts\nhand edited line\n" +
		provenanceStamp("sess-a", "2026-09-01T09:00:00Z") + "\n" + strings.Repeat("s", 1900) + "\n"
	if err := os.WriteFile(filepath.Join(dir, MemoryFileName), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}

	result := mustAdd(t, store, TargetMemory, "Facts", strings.Repeat("w", 200))
	if result.Evicted == 0 {
		t.Fatal("expected eviction over the cap")
	}
	view, _ := store.View(context.Background(), TargetMemory)
	if strings.Contains(view.Content, "hand edited line") {
		t.Fatalf("unstamped entry should be evicted first:\n%s", view.Content)
	}
	if !strings.Contains(view.Content, strings.Repeat("w", 200)) {
		t.Fatalf("new entry lost:\n%s", view.Content)
	}
}

func TestReplaceSwapsSection(t *testing.T) {
	store := newTestStore(t, "sess-x")
	mustAdd(t, store, TargetMemory, "Facts", "old first")
	mustAdd(t, store, TargetMemory, "Other", "keep me")

	result, err := store.Replace(context.Background(), TargetMemory, "Facts", "new content")
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	view, _ := store.View(context.Background(), TargetMemory)
	if strings.Contains(view.Content, "old first") || !strings.Contains(view.Content, "new content") {
		t.Fatalf("replace did not swap the section:\n%s", view.Content)
	}
	if !strings.Contains(view.Content, "keep me") {
		t.Fatalf("replace touched another section:\n%s", view.Content)
	}
	if result.Size != len(view.Content) {
		t.Fatalf("result size %d != view size %d", result.Size, len(view.Content))
	}
}

func TestReplaceAndRemoveRequireExistingSection(t *testing.T) {
	store := newTestStore(t, "sess-x")
	mustAdd(t, store, TargetMemory, "Facts", "content")
	before, _ := os.ReadFile(store.Path(TargetMemory))

	if _, err := store.Replace(context.Background(), TargetMemory, "Missing", "x"); !errors.Is(err, ErrSectionNotFound) {
		t.Fatalf("replace missing err = %v, want ErrSectionNotFound", err)
	}
	if _, err := store.Remove(context.Background(), TargetMemory, "Missing"); !errors.Is(err, ErrSectionNotFound) {
		t.Fatalf("remove missing err = %v, want ErrSectionNotFound", err)
	}
	after, _ := os.ReadFile(store.Path(TargetMemory))
	if string(before) != string(after) {
		t.Fatal("failed replace/remove modified the file")
	}
}

func TestRemoveDropsSection(t *testing.T) {
	store := newTestStore(t, "sess-x")
	mustAdd(t, store, TargetMemory, "Facts", "content")
	mustAdd(t, store, TargetMemory, "Other", "keep me")

	if _, err := store.Remove(context.Background(), TargetMemory, "Facts"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	view, _ := store.View(context.Background(), TargetMemory)
	if strings.Contains(view.Content, "Facts") || !strings.Contains(view.Content, "keep me") {
		t.Fatalf("remove result wrong:\n%s", view.Content)
	}
}

// TestAtomicWriteSemantics pins the crash-safety contract: a successful
// write shows only new content with no temp residue; a failed rename keeps
// the target intact and removes the temp file; a failed persist never
// touches the existing file. Either way the reader observes the old or the
// new content, never a half-written file.
func TestAtomicWriteSemantics(t *testing.T) {
	t.Run("success replaces content and leaves no temp file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "MEMORY.md")
		if err := writeFileAtomic(path, []byte("one")); err != nil {
			t.Fatalf("first write: %v", err)
		}
		if err := writeFileAtomic(path, []byte("two")); err != nil {
			t.Fatalf("overwrite: %v", err)
		}
		data, err := os.ReadFile(path)
		if err != nil || string(data) != "two" {
			t.Fatalf("content = %q, err = %v; want \"two\"", data, err)
		}
		assertNoTempFiles(t, dir)
	})

	t.Run("rename failure keeps the target and removes the temp file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "MEMORY.md")
		// A directory at the target path makes the final rename fail after
		// the temp file was fully written: the exact window a crash matters.
		if err := os.Mkdir(path, 0o755); err != nil {
			t.Fatal(err)
		}
		err := writeFileAtomic(path, []byte("corrupt"))
		if err == nil {
			t.Fatal("expected rename over a directory to fail")
		}
		if info, statErr := os.Stat(path); statErr != nil || !info.IsDir() {
			t.Fatalf("target damaged by failed rename: %v", statErr)
		}
		assertNoTempFiles(t, dir)
	})

	t.Run("failed persist never touches the existing file", func(t *testing.T) {
		store := newTestStore(t, "sess-x")
		mustAdd(t, store, TargetMemory, "Facts", "original")
		before, _ := os.ReadFile(store.Path(TargetMemory))

		store.persist = func(string, []byte) error { return errors.New("disk on fire") }
		if _, err := store.Add(context.Background(), TargetMemory, "Facts", "second"); err == nil {
			t.Fatal("expected the persisted write to fail")
		}
		after, _ := os.ReadFile(store.Path(TargetMemory))
		if string(before) != string(after) {
			t.Fatal("failed write modified the file")
		}
		assertNoTempFiles(t, store.Dir())
	})

	t.Run("store write leaves no temp files behind", func(t *testing.T) {
		store := newTestStore(t, "sess-x")
		mustAdd(t, store, TargetMemory, "Facts", "content")
		assertNoTempFiles(t, store.Dir())
	})
}

func assertNoTempFiles(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp-") {
			t.Fatalf("temp file left behind: %s", entry.Name())
		}
	}
}
