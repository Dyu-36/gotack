package memory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Target selects which memory file an operation addresses.
type Target string

const (
	// TargetMemory addresses MEMORY.md, agent-curated durable facts.
	TargetMemory Target = "memory"
	// TargetUser addresses USER.md, user preferences and stable profile.
	TargetUser Target = "user"
)

const (
	// MemoryFileName and UserFileName are fixed so the guard's context-dir
	// write denial and the contract document can name them exactly.
	MemoryFileName = "MEMORY.md"
	UserFileName   = "USER.md"

	// MemoryCap and UserCap bound the prompt cost of the two files
	// (Hermes uses ~2200 and ~1375 characters). Caps are measured in UTF-8
	// bytes: that is the quantity that actually costs prompt space, and it
	// is deterministic across platforms.
	MemoryCap = 2200
	UserCap   = 1375
)

// FileNameFor maps a target to its file name.
func FileNameFor(target Target) string {
	if target == TargetUser {
		return UserFileName
	}
	return MemoryFileName
}

// CapFor maps a target to its size cap in bytes.
func CapFor(target Target) int {
	if target == TargetUser {
		return UserCap
	}
	return MemoryCap
}

// Store reads and writes the two memory files under one directory. It is
// the only sanctioned writer of those files (decision 0003); every mutation
// is capped, stamped with provenance, and persisted atomically.
type Store struct {
	dir     string
	session string
	now     func() time.Time
	// persist is the atomic-write seam; tests swap it to prove that a failed
	// write never touches the existing file.
	persist func(path string, data []byte) error
}

// NewStore returns a Store rooted at dir. session names the writer recorded
// in provenance stamps; it comes from the host environment and may be
// "unknown" when the engine exports no session id to MCP servers.
func NewStore(dir, session string) *Store {
	return &Store{
		dir:     dir,
		session: session,
		now:     func() time.Time { return time.Now() },
		persist: writeFileAtomic,
	}
}

// Dir is the directory holding the memory files.
func (s *Store) Dir() string { return s.dir }

// Path is the absolute path of one target's file. The file name comes from
// the fixed FileNameFor mapping, never from tool input: a validated target
// enum selects one of two constants, so no request-derived segment can ever
// reach the filesystem join.
func (s *Store) Path(target Target) string {
	return filepath.Join(s.dir, FileNameFor(target))
}

// Result is the tool-visible outcome of one operation. Size, cap and
// remaining are always returned so the model can self-manage its budget
// without an extra view call (plan Phase 2.2).
type Result struct {
	Target    string `json:"target"`
	File      string `json:"file"`
	Size      int    `json:"size"`
	Cap       int    `json:"cap"`
	Remaining int    `json:"remaining"`
	Evicted   int    `json:"evicted"`
	// Content is populated by view only.
	Content string `json:"content,omitempty"`
}

// View returns the current content and budget of one target. A file that
// was never written is reported as empty rather than an error.
func (s *Store) View(ctx context.Context, target Target) (Result, error) {
	file, err := s.load(target)
	if err != nil {
		return Result{}, err
	}
	content := file.serialize()
	return s.result(target, content, 0), nil
}

// Add appends content to a section, creating the section when absent, then
// enforces the cap by evicting the oldest entries.
func (s *Store) Add(ctx context.Context, target Target, section, content string) (Result, error) {
	file, err := s.load(target)
	if err != nil {
		return Result{}, err
	}
	entry := s.newEntry(content)
	index := file.findSection(section)
	if index < 0 {
		file.Sections = append(file.Sections, Section{Heading: section, Entries: []Entry{entry}})
		index = len(file.Sections) - 1
	} else {
		file.Sections[index].Entries = append(file.Sections[index].Entries, entry)
	}
	return s.commit(target, &file, index, len(file.Sections[index].Entries)-1)
}

// Replace swaps one existing section for the new content. A missing section
// is an explicit error, not a silent no-op (decision 0003, item: validation).
func (s *Store) Replace(ctx context.Context, target Target, section, content string) (Result, error) {
	file, err := s.load(target)
	if err != nil {
		return Result{}, err
	}
	index := file.findSection(section)
	if index < 0 {
		return Result{}, s.notFound(target, section)
	}
	file.Sections[index].Entries = []Entry{s.newEntry(content)}
	return s.commit(target, &file, index, 0)
}

// Remove drops one existing section. A missing section is an explicit error.
func (s *Store) Remove(ctx context.Context, target Target, section string) (Result, error) {
	file, err := s.load(target)
	if err != nil {
		return Result{}, err
	}
	index := file.findSection(section)
	if index < 0 {
		return Result{}, s.notFound(target, section)
	}
	file.Sections = append(file.Sections[:index], file.Sections[index+1:]...)
	return s.commit(target, &file, -1, -1)
}

// commit enforces the cap keeping the entry at keepSection/keepEntry alive
// (-1/-1 when the operation added no entry), then persists atomically.
func (s *Store) commit(target Target, file *File, keepSection, keepEntry int) (Result, error) {
	evicted, ok := enforceCap(file, CapFor(target), keepSection, keepEntry)
	if !ok {
		size := len(file.serialize())
		return Result{}, fmt.Errorf(
			"memory: this write needs %d bytes but %s caps at %d even after evicting every other entry; shorten or consolidate the content: %w",
			size, FileNameFor(target), CapFor(target), ErrCapExceeded)
	}
	content := file.serialize()
	if err := s.persist(s.Path(target), []byte(content)); err != nil {
		return Result{}, fmt.Errorf("memory: persist %s: %w", FileNameFor(target), err)
	}
	return s.result(target, content, evicted), nil
}

func (s *Store) result(target Target, content string, evicted int) Result {
	cap := CapFor(target)
	size := len(content)
	return Result{
		Target:    string(target),
		File:      FileNameFor(target),
		Size:      size,
		Cap:       cap,
		Remaining: cap - size,
		Evicted:   evicted,
		Content:   content,
	}
}

func (s *Store) notFound(target Target, section string) error {
	return fmt.Errorf("memory: section %q not found in %s (use action \"view\" to list sections): %w",
		section, FileNameFor(target), ErrSectionNotFound)
}

// newEntry stamps the entry with the writing session and time so every
// stored item is traceable (decision 0003).
func (s *Store) newEntry(content string) Entry {
	return Entry{
		Session: s.session,
		At:      s.now().UTC().Format(time.RFC3339),
		Lines:   strings.Split(strings.TrimRight(content, "\n"), "\n"),
	}
}

// load reads and parses one target; a missing file starts empty so the
// first write needs no setup.
func (s *Store) load(target Target) (File, error) {
	data, err := os.ReadFile(s.Path(target))
	if err != nil {
		if os.IsNotExist(err) {
			return File{}, nil
		}
		return File{}, fmt.Errorf("memory: read %s: %w", FileNameFor(target), err)
	}
	return parseFile(string(data)), nil
}

// enforceCap evicts the oldest entries until the serialized file fits the
// cap. Policy (documented in docs/contracts/gotack-memory-mcp.md): the
// just-written entry (keep) always survives; among the rest, unstamped
// entries are oldest, then stamped entries in ascending timestamp order,
// ties broken by file position (top of file first). Sections left empty
// are dropped. ok is false when the file cannot fit even with only keep
// left, so the caller rejects the write instead of persisting a loss.
//
// Entries are addressed by index, never by pointer: eviction mutates the
// same slices it selects from, so a retained pointer would silently
// re-target after a removal and could mark the survivor for eviction.
func enforceCap(file *File, cap int, keepSection, keepEntry int) (evicted int, ok bool) {
	for len(file.serialize()) > cap {
		victimSection, victimEntry, found := oldestEntry(file, keepSection, keepEntry)
		if !found {
			return evicted, false
		}
		sectionVanishes := len(file.Sections[victimSection].Entries) == 1
		removeEntryAt(file, victimSection, victimEntry)
		// Keep the survivor's coordinates valid as the structure shifts.
		switch {
		case victimSection == keepSection && victimEntry < keepEntry:
			keepEntry--
		case victimSection < keepSection && sectionVanishes:
			keepSection--
		}
		evicted++
	}
	return evicted, true
}

// oldestEntry picks the eviction victim's coordinates: never the keep
// entry, unstamped before stamped, older timestamps before newer, ties by
// file position.
func oldestEntry(file *File, keepSection, keepEntry int) (sectionIdx, entryIdx int, found bool) {
	type candidate struct {
		sectionIdx int
		entryIdx   int
		stamped    bool
		at         time.Time
		position   int
	}
	var candidates []candidate
	position := 0
	for si := range file.Sections {
		section := &file.Sections[si]
		for ei := range section.Entries {
			if si == keepSection && ei == keepEntry {
				continue
			}
			entry := &section.Entries[ei]
			c := candidate{sectionIdx: si, entryIdx: ei, position: position}
			if entry.stamped() {
				if parsed, err := time.Parse(time.RFC3339, entry.At); err == nil {
					c.stamped = true
					c.at = parsed
				}
			}
			candidates = append(candidates, c)
			position++
		}
	}
	if len(candidates) == 0 {
		return 0, 0, false
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if a.stamped != b.stamped {
			return !a.stamped // unstamped entries predate recorded history
		}
		if a.stamped && !a.at.Equal(b.at) {
			return a.at.Before(b.at)
		}
		return a.position < b.position
	})
	return candidates[0].sectionIdx, candidates[0].entryIdx, true
}

// removeEntryAt deletes one entry and drops its section when it goes empty.
func removeEntryAt(file *File, sectionIdx, entryIdx int) {
	section := &file.Sections[sectionIdx]
	section.Entries = append(section.Entries[:entryIdx], section.Entries[entryIdx+1:]...)
	if len(section.Entries) == 0 {
		file.Sections = append(file.Sections[:sectionIdx], file.Sections[sectionIdx+1:]...)
	}
}

// writeFileAtomic persists data via a temp file in the same directory plus
// rename, so a crash at any point leaves either the old or the new content
// on disk, never a half-written file in the system prompt (decision 0003).
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	_, err = tmp.Write(data)
	if err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("rename temp file over target: %w", err)
	}
	return nil
}
