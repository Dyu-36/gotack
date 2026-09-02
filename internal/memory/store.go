package memory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// Target selects one of Hermes' two bounded memory stores.
type Target string

const (
	TargetMemory Target = "memory"
	TargetUser   Target = "user"
)

const (
	MemoryFileName = "MEMORY.md"
	UserFileName   = "USER.md"
	MemoryCap      = 2200
	UserCap        = 1375
)

const (
	actionAdd     = "add"
	actionReplace = "replace"
	actionRemove  = "remove"
)

// Operation is one item in an atomic memory update.
type Operation struct {
	Action  string `json:"action"`
	Content string `json:"content,omitempty"`
	NewText string `json:"new_text,omitempty"`
	OldText string `json:"old_text,omitempty"`
}

// Result intentionally omits memory content. Successful calls only confirm
// completion and current budget, matching Hermes' anti-thrashing response.
type Result struct {
	Success    bool   `json:"success"`
	Done       bool   `json:"done"`
	Target     string `json:"target"`
	Usage      string `json:"usage"`
	EntryCount int    `json:"entry_count"`
	Message    string `json:"message,omitempty"`
	Note       string `json:"note"`
}

// Store owns the two memory files under dir.
type Store struct {
	dir     string
	persist func(path string, data []byte) error
}

func NewStore(dir string) *Store {
	return &Store{dir: dir, persist: writeFileAtomic}
}

func FileNameFor(target Target) string {
	if target == TargetUser {
		return UserFileName
	}
	return MemoryFileName
}

func CapFor(target Target) int {
	if target == TargetUser {
		return UserCap
	}
	return MemoryCap
}

func (s *Store) Path(target Target) string {
	return filepath.Join(s.dir, FileNameFor(target))
}

func (s *Store) Add(ctx context.Context, target Target, content string) (Result, error) {
	return s.Apply(ctx, target, []Operation{{Action: actionAdd, Content: content}})
}

func (s *Store) Replace(ctx context.Context, target Target, oldText, content string) (Result, error) {
	return s.Apply(ctx, target, []Operation{{Action: actionReplace, OldText: oldText, Content: content}})
}

func (s *Store) Remove(ctx context.Context, target Target, oldText string) (Result, error) {
	return s.Apply(ctx, target, []Operation{{Action: actionRemove, OldText: oldText}})
}

// Apply evaluates operations against a copy and persists only the valid final
// state. Intermediate overflow is allowed so one batch can consolidate and add.
func (s *Store) Apply(ctx context.Context, target Target, operations []Operation) (Result, error) {
	if target != TargetMemory && target != TargetUser {
		return Result{}, ErrUnknownTarget
	}
	if len(operations) == 0 {
		return Result{}, ErrEmptyBatch
	}
	// Scan every proposed entry before touching the filesystem. One blocked
	// operation rejects the entire batch.
	for index, operation := range operations {
		action := strings.TrimSpace(operation.Action)
		if action != actionAdd && action != actionReplace {
			continue
		}
		content := operation.Content
		if content == "" {
			content = operation.NewText
		}
		if strings.TrimSpace(content) != "" {
			if err := Scan(strings.TrimSpace(content)); err != nil {
				return Result{}, fmt.Errorf("operation %d: %w", index+1, err)
			}
		}
	}

	release, err := acquireFileLock(ctx, s.Path(target)+".lock")
	if err != nil {
		return Result{}, fmt.Errorf("memory: lock %s: %w", FileNameFor(target), err)
	}
	defer release()

	current, err := s.load(target)
	if err != nil {
		return Result{}, err
	}
	working := append([]string(nil), current...)
	duplicateOnly := len(operations) == 1 && strings.TrimSpace(operations[0].Action) == actionAdd

	for index, operation := range operations {
		operationChanged, err := applyOperation(&working, operation)
		if err != nil {
			return Result{}, withState(target, fmt.Errorf("operation %d: %w", index+1, err), current)
		}
		if operationChanged {
			duplicateOnly = false
		}
	}
	if duplicateOnly && equalEntries(current, working) {
		return successResult(target, working, "Entry already exists (no duplicate added)."), nil
	}

	body := serializeEntries(working)
	size := utf8.RuneCountInString(body)
	capacity := CapFor(target)
	if size > capacity {
		return Result{}, &OverCapError{
			Target:  target,
			Used:    utf8.RuneCountInString(serializeEntries(current)),
			Cap:     capacity,
			Wanted:  size,
			Entries: append([]string(nil), current...),
		}
	}

	if !equalEntries(current, working) {
		if err := s.persist(s.Path(target), []byte(Render(target, body))); err != nil {
			return Result{}, fmt.Errorf("memory: persist %s: %w", FileNameFor(target), err)
		}
	}

	message := fmt.Sprintf("Applied %d operation(s).", len(operations))
	if len(operations) == 1 {
		switch operations[0].Action {
		case actionAdd:
			message = "Entry added."
		case actionReplace:
			message = "Entry replaced."
		case actionRemove:
			message = "Entry removed."
		}
	}
	return successResult(target, working, message), nil
}

func applyOperation(entries *[]string, operation Operation) (bool, error) {
	action := strings.TrimSpace(operation.Action)
	content := operation.Content
	if content == "" {
		content = operation.NewText
	}
	content = strings.TrimSpace(content)
	oldText := strings.TrimSpace(operation.OldText)

	switch action {
	case actionAdd:
		if content == "" {
			return false, ErrEmptyContent
		}
		for _, entry := range *entries {
			if entry == content {
				return false, nil
			}
		}
		*entries = append(*entries, content)
		return true, nil

	case actionReplace:
		if oldText == "" {
			return false, ErrMissingOldText
		}
		if content == "" {
			return false, ErrEmptyContent
		}
		index, err := locateEntry(*entries, oldText)
		if err != nil {
			return false, err
		}
		if (*entries)[index] == content {
			return false, nil
		}
		// Hermes replaces the whole matched entry, not the matching substring.
		(*entries)[index] = content
		return true, nil

	case actionRemove:
		if oldText == "" {
			return false, ErrMissingOldText
		}
		index, err := locateEntry(*entries, oldText)
		if err != nil {
			return false, err
		}
		*entries = append((*entries)[:index], (*entries)[index+1:]...)
		return true, nil

	default:
		return false, ErrUnknownAction
	}
}

func locateEntry(entries []string, oldText string) (int, error) {
	matches := make([]int, 0, 2)
	for index, entry := range entries {
		if strings.Contains(entry, oldText) {
			matches = append(matches, index)
		}
	}
	switch len(matches) {
	case 0:
		return -1, fmt.Errorf("%w: no entry contains %q", ErrTextNotFound, oldText)
	case 1:
		return matches[0], nil
	default:
		return -1, fmt.Errorf("%w: %q matched %d entries; use a more specific substring", ErrTextNotUnique, oldText, len(matches))
	}
}

func successResult(target Target, entries []string, message string) Result {
	current := utf8.RuneCountInString(serializeEntries(entries))
	capacity := CapFor(target)
	percent := 0
	if capacity > 0 {
		percent = current * 100 / capacity
		if percent > 100 {
			percent = 100
		}
	}
	return Result{
		Success:    true,
		Done:       true,
		Target:     string(target),
		Usage:      fmt.Sprintf("%d%% — %s/%s chars", percent, group(current), group(capacity)),
		EntryCount: len(entries),
		Message:    message,
		Note:       "Write saved. This update is complete — do not repeat it.",
	}
}

func withState(target Target, cause error, entries []string) error {
	return &operationError{
		cause:   cause,
		entries: append([]string(nil), entries...),
		used:    utf8.RuneCountInString(serializeEntries(entries)),
		cap:     CapFor(target),
	}
}

func equalEntries(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func (s *Store) load(target Target) ([]string, error) {
	data, err := os.ReadFile(s.Path(target))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("memory: read %s: %w", FileNameFor(target), err)
	}
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("memory: read %s: %w", FileNameFor(target), ErrInvalidUTF8)
	}
	return parseFile(string(data)).Entries, nil
}

func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	temporary, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	temporaryName := temporary.Name()
	if _, err = temporary.Write(data); err == nil {
		err = temporary.Sync()
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(temporaryName)
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := replaceFile(temporaryName, path); err != nil {
		_ = os.Remove(temporaryName)
		return fmt.Errorf("replace target: %w", err)
	}
	return nil
}
