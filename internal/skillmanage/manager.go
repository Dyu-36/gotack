package skillmanage

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const (
	MaxOperations          = 20
	MaxNameLength          = 64
	MaxDescriptionChars    = 60
	MaxSkillContent        = 100_000
	MaxSupportFileBytes    = 1 << 20
	maxReadSessions        = 64
	maxReadMarksPerSession = 64

	actionCreate     = "create"
	actionPatch      = "patch"
	actionDelete     = "delete"
	actionWriteFile  = "write_file"
	actionRemoveFile = "remove_file"
)

type Operation struct {
	Action       string  `json:"action"`
	Name         string  `json:"name"`
	Content      string  `json:"content,omitempty"`
	Category     string  `json:"category,omitempty"`
	OldString    string  `json:"old_string,omitempty"`
	NewString    *string `json:"new_string,omitempty"`
	FilePath     string  `json:"file_path,omitempty"`
	FileContent  *string `json:"file_content,omitempty"`
	AbsorbedInto string  `json:"absorbed_into,omitempty"`
}

type AppliedOperation struct {
	Action string `json:"action"`
	Name   string `json:"name"`
	Path   string `json:"path,omitempty"`
}

type Result struct {
	Success                bool               `json:"success"`
	OperationsApplied      int                `json:"operations_applied,omitempty"`
	Results                []AppliedOperation `json:"results,omitempty"`
	Error                  string             `json:"error,omitempty"`
	FailedIndex            *int               `json:"failed_index,omitempty"`
	CompletedBeforeFailure int                `json:"completed_before_failure,omitempty"`
}

type Manager struct {
	root string
	mu   sync.Mutex

	readMarks        map[string]map[string]string
	readMarkOrder    map[string][]string
	readSessionOrder []string
}

type RequestMeta struct {
	SessionID        string
	BackgroundReview bool
}

type applyState struct {
	meta           RequestMeta
	owned          map[string]bool
	ownershipDirty bool
	readMarks      map[string]string
	known          map[string]bool
}

func New(root string) (*Manager, error) {
	if strings.TrimSpace(root) == "" {
		return nil, errors.New("skill root is required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve skill root: %w", err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, fmt.Errorf("create skill root: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return nil, fmt.Errorf("resolve skill root links: %w", err)
	}
	return &Manager{
		root:             filepath.Clean(resolved),
		readMarks:        make(map[string]map[string]string),
		readMarkOrder:    make(map[string][]string),
		readSessionOrder: make([]string, 0, maxReadSessions),
	}, nil
}

func (m *Manager) Root() string {
	return m.root
}

func (m *Manager) Apply(ctx context.Context, operations []Operation) Result {
	return m.ApplyWithMeta(ctx, operations, RequestMeta{})
}

func (m *Manager) ApplyWithMeta(ctx context.Context, operations []Operation, meta RequestMeta) Result {
	m.mu.Lock()
	defer m.mu.Unlock()
	if meta.BackgroundReview {
		if strings.TrimSpace(meta.SessionID) == "" {
			return failed(errors.New("background review session is required"))
		}
	}
	if err := validateBatch(operations); err != nil {
		return failed(err)
	}
	if err := ctx.Err(); err != nil {
		return failed(err)
	}
	owned, err := m.loadOwnership()
	if err != nil {
		return failed(err)
	}
	marks := make(map[string]string)
	if meta.BackgroundReview {
		marks = m.takeReadMarks(meta.SessionID)
	}
	snapshot, err := m.snapshot(operations)
	if err != nil {
		return failed(err)
	}
	defer snapshot.cleanup()

	state := &applyState{
		meta: meta, owned: owned, readMarks: marks, known: make(map[string]bool),
	}
	results := make([]AppliedOperation, 0, len(operations))
	for index, operation := range operations {
		if err := ctx.Err(); err != nil {
			return m.abort(snapshot, index, err)
		}
		applied, err := m.applyOne(operation, state)
		if err != nil {
			return m.abort(snapshot, index, err)
		}
		results = append(results, applied)
	}
	if state.ownershipDirty {
		if err := m.saveOwnership(state.owned); err != nil {
			return m.abort(snapshot, len(operations), fmt.Errorf("save skill ownership: %w", err))
		}
	}
	return Result{
		Success:           true,
		OperationsApplied: len(results),
		Results:           results,
	}
}

func (m *Manager) abort(snapshot *batchSnapshot, index int, operationErr error) Result {
	rollbackErr := m.rollback(snapshot)
	return operationFailure(index, operationErr, rollbackErr)
}

func failed(err error) Result {
	return Result{Success: false, Error: err.Error()}
}

func operationFailure(index int, operationErr, rollbackErr error) Result {
	message := operationErr.Error()
	if rollbackErr != nil {
		message += "; rollback failed: " + rollbackErr.Error()
	} else {
		message += "; all touched skills were rolled back"
	}
	return Result{
		Success:                false,
		Error:                  message,
		FailedIndex:            &index,
		CompletedBeforeFailure: index,
	}
}

func validateBatch(operations []Operation) error {
	if len(operations) == 0 {
		return errors.New("operations must be a non-empty array")
	}
	if len(operations) > MaxOperations {
		return fmt.Errorf("operations is capped at %d per call", MaxOperations)
	}
	if len(operations) > 1 {
		for _, operation := range operations {
			if operation.Action == actionDelete {
				return errors.New("delete must be the sole operation in its call")
			}
		}
	}

	seenName := make(map[string]bool)
	touched := make(map[string]bool)
	for index, operation := range operations {
		if err := validateOperation(operation); err != nil {
			return fmt.Errorf("operations[%d]: %w", index, err)
		}
		if operation.Action == actionCreate && seenName[operation.Name] {
			return fmt.Errorf("operations[%d]: create for %q must precede that skill's other operations", index, operation.Name)
		}
		seenName[operation.Name] = true

		target := operationTarget(operation)
		key := operation.Name + "\x00" + target
		destructive := operation.Action == actionCreate || operation.Action == actionWriteFile || operation.Action == actionRemoveFile
		if destructive && touched[key] {
			return fmt.Errorf("operations[%d]: %s on %q would discard an earlier operation on the same file", index, operation.Action, target)
		}
		touched[key] = true
	}
	return nil
}

func operationTarget(operation Operation) string {
	switch operation.Action {
	case actionCreate:
		return "SKILL.md"
	case actionPatch:
		if operation.FilePath == "" {
			return "SKILL.md"
		}
		clean, _ := normalizeSupportPath(operation.FilePath)
		return clean
	default:
		clean, _ := normalizeSupportPath(operation.FilePath)
		return clean
	}
}

func (m *Manager) applyOne(operation Operation, state *applyState) (AppliedOperation, error) {
	if state.meta.BackgroundReview && operation.Action != actionCreate && !state.owned[operation.Name] {
		return AppliedOperation{}, fmt.Errorf("refusing background review %s for %q: the skill is not agent-owned", operation.Action, operation.Name)
	}

	var (
		path string
		err  error
	)
	switch operation.Action {
	case actionCreate:
		path, err = m.create(operation)
		if err == nil && state.meta.BackgroundReview {
			state.owned[operation.Name] = true
			state.ownershipDirty = true
			state.markKnown(filepath.Join(m.root, filepath.FromSlash(path), "SKILL.md"))
		}
	case actionPatch:
		path, err = m.patch(operation, state)
	case actionDelete:
		path, err = m.delete(operation.Name, operation, state)

		if err == nil && !state.meta.BackgroundReview && state.owned[operation.Name] {
			delete(state.owned, operation.Name)
			state.ownershipDirty = true
		}
	case actionWriteFile:
		path, err = m.writeFile(operation, state)
	case actionRemoveFile:
		path, err = m.removeFile(operation, state)
	}
	if err != nil {
		return AppliedOperation{}, err
	}
	return AppliedOperation{Action: operation.Action, Name: operation.Name, Path: path}, nil
}

func (m *Manager) create(operation Operation) (string, error) {
	if existing, err := m.findSkill(operation.Name); err != nil {
		return "", err
	} else if existing != "" {
		return "", fmt.Errorf("skill %q already exists", operation.Name)
	}

	parent := m.root
	if operation.Category != "" {
		parent = filepath.Join(m.root, operation.Category)
		if categoryIsSkill, err := m.isSkillDir(parent); err != nil {
			return "", err
		} else if categoryIsSkill {
			return "", fmt.Errorf("category %q collides with an existing skill", operation.Category)
		}
	}
	if err := m.secureMkdirAll(parent); err != nil {
		return "", err
	}
	target := filepath.Join(parent, operation.Name)
	if err := m.ensureWithinRoot(target); err != nil {
		return "", err
	}
	if _, err := os.Lstat(target); !errors.Is(err, fs.ErrNotExist) {
		if err == nil {
			return "", fmt.Errorf("skill target already exists: %s", target)
		}
		return "", fmt.Errorf("inspect skill target: %w", err)
	}

	temporary, err := os.MkdirTemp(parent, ".skill-create-*")
	if err != nil {
		return "", fmt.Errorf("create temporary skill directory: %w", err)
	}
	defer os.RemoveAll(temporary)
	if err := os.WriteFile(filepath.Join(temporary, "SKILL.md"), []byte(operation.Content), 0o644); err != nil {
		return "", fmt.Errorf("write temporary SKILL.md: %w", err)
	}
	if err := os.Rename(temporary, target); err != nil {
		return "", fmt.Errorf("publish skill %q: %w", operation.Name, err)
	}
	return m.relative(target), nil
}

func (m *Manager) patch(operation Operation, state *applyState) (string, error) {
	skillDir, err := m.requireSkill(operation.Name)
	if err != nil {
		return "", err
	}
	target := filepath.Join(skillDir, "SKILL.md")
	if operation.FilePath != "" {
		target, err = m.resolveSupportTarget(skillDir, operation.FilePath)
		if err != nil {
			return "", err
		}
	}
	if err := m.requireReadMark(state, target); err != nil {
		return "", err
	}
	data, err := m.readRegularFile(target)
	if err != nil {
		return "", err
	}
	content := string(data)
	matches := strings.Count(content, operation.OldString)
	if matches == 0 {
		return "", errors.New("old_string was not found; load the exact current file with Crush view and copy its text")
	}
	if matches > 1 {
		return "", fmt.Errorf("old_string matches %d times; provide a unique excerpt", matches)
	}
	updated := strings.Replace(content, operation.OldString, *operation.NewString, 1)
	if operation.FilePath == "" {
		if err := validateSkillContent(operation.Name, updated, false); err != nil {
			return "", fmt.Errorf("patch would make SKILL.md invalid: %w", err)
		}
	} else if err := validateSupportContent(updated, operation.FilePath); err != nil {
		return "", err
	}
	if err := m.atomicWrite(target, []byte(updated)); err != nil {
		return "", err
	}
	state.markKnown(target)
	return m.relative(target), nil
}

func (m *Manager) delete(name string, operation Operation, state *applyState) (string, error) {
	skillDir, err := m.requireSkill(name)
	if err != nil {
		return "", err
	}
	if err := m.requireReadMark(state, filepath.Join(skillDir, "SKILL.md")); err != nil {
		return "", err
	}
	if state.meta.BackgroundReview {
		absorbedInto := strings.TrimSpace(operation.AbsorbedInto)
		if absorbedInto == "" {
			return "", errors.New("background review delete requires absorbed_into: name the existing agent-owned umbrella that received this skill")
		}
		if absorbedInto == name {
			return "", errors.New("absorbed_into cannot equal the skill being deleted")
		}
		if !state.owned[absorbedInto] {
			return "", fmt.Errorf("refusing background review delete of %q: absorbed_into %q is not agent-owned", name, absorbedInto)
		}
		if _, err := m.requireSkill(absorbedInto); err != nil {
			return "", fmt.Errorf("absorbed_into %q must name an existing agent-owned umbrella: %w", absorbedInto, err)
		}
		return m.archiveSkillTree(skillDir)
	}
	relative := m.relative(skillDir)
	if err := m.removeSkillTree(skillDir); err != nil {
		return "", err
	}
	m.removeEmptyCategory(filepath.Dir(skillDir))
	return relative, nil
}

func (m *Manager) writeFile(operation Operation, state *applyState) (string, error) {
	skillDir, err := m.requireSkill(operation.Name)
	if err != nil {
		return "", err
	}
	target, err := m.resolveSupportTarget(skillDir, operation.FilePath)
	if err != nil {
		return "", err
	}
	if err := m.requireReadMark(state, target); err != nil {
		return "", err
	}
	if err := m.secureMkdirAll(filepath.Dir(target)); err != nil {
		return "", err
	}
	if err := m.atomicWrite(target, []byte(*operation.FileContent)); err != nil {
		return "", err
	}
	state.markKnown(target)
	return m.relative(target), nil
}

func (m *Manager) removeFile(operation Operation, state *applyState) (string, error) {
	skillDir, err := m.requireSkill(operation.Name)
	if err != nil {
		return "", err
	}
	target, err := m.resolveSupportTarget(skillDir, operation.FilePath)
	if err != nil {
		return "", err
	}
	if err := m.requireReadMark(state, target); err != nil {
		return "", err
	}
	if _, err := m.readRegularFile(target); err != nil {
		return "", err
	}
	if err := os.Remove(target); err != nil {
		return "", fmt.Errorf("remove support file: %w", err)
	}
	delete(state.known, readPathKey(target))
	m.removeEmptySupportDirs(filepath.Dir(target), skillDir)
	return m.relative(target), nil
}

func (state *applyState) markKnown(target string) {
	if state.meta.BackgroundReview {
		state.known[readPathKey(target)] = true
	}
}

func (m *Manager) requireSkill(name string) (string, error) {
	path, err := m.findSkill(name)
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", fmt.Errorf("skill %q was not found", name)
	}
	return path, nil
}

func (m *Manager) findSkill(name string) (string, error) {
	direct := filepath.Join(m.root, name)
	if ok, err := m.isSkillDir(direct); err != nil {
		return "", err
	} else if ok {
		return direct, nil
	}

	entries, err := os.ReadDir(m.root)
	if err != nil {
		return "", fmt.Errorf("list skill root: %w", err)
	}
	var found string
	for _, entry := range entries {
		if !validCategoryName(entry.Name()) || entry.Name() == name {
			continue
		}
		category := filepath.Join(m.root, entry.Name())
		if ok, err := m.isSkillDir(category); err != nil {
			return "", err
		} else if ok {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return "", fmt.Errorf("inspect category %q: %w", entry.Name(), err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			continue
		}
		candidate := filepath.Join(category, name)
		if ok, err := m.isSkillDir(candidate); err != nil {
			return "", err
		} else if ok {
			if found != "" {
				return "", fmt.Errorf("skill name %q is ambiguous", name)
			}
			found = candidate
		}
	}
	return found, nil
}

func (m *Manager) relative(path string) string {
	relative, err := filepath.Rel(m.root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(relative)
}

func sortedOwned(owned map[string]bool) []string {
	names := make([]string, 0, len(owned))
	for name, yes := range owned {
		if yes {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}
