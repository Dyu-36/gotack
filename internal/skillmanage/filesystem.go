package skillmanage

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	ownershipFileName       = ".ownership.json"
	legacyOwnershipFileName = ".gotack-agent-skills.json"
	ownershipVersion        = 1
	archiveDirName          = ".archive"
)

type skillSnapshot struct {
	Name         string
	OriginalPath string
	BackupPath   string
	Existed      bool
}

type batchSnapshot struct {
	root   string
	skills []skillSnapshot
}

type ownershipManifest struct {
	Version    int      `json:"version"`
	AgentOwned []string `json:"agent_owned"`
}

func (m *Manager) ensureWithinRoot(path string) error {
	relative, err := filepath.Rel(m.root, filepath.Clean(path))
	if err != nil {
		return fmt.Errorf("resolve path below skill root: %w", err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return fmt.Errorf("path escapes the skill root: %s", path)
	}
	return nil
}

func (m *Manager) secureMkdirAll(path string) error {
	path = filepath.Clean(path)
	if err := m.ensureWithinRoot(path); err != nil {
		return err
	}
	relative, err := filepath.Rel(m.root, path)
	if err != nil {
		return err
	}
	current := m.root
	if relative == "." {
		return nil
	}
	for _, part := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		switch {
		case errors.Is(statErr, fs.ErrNotExist):
			if err := os.Mkdir(current, 0o755); err != nil && !errors.Is(err, fs.ErrExist) {
				return fmt.Errorf("create directory %s: %w", current, err)
			}
			info, statErr = os.Lstat(current)
			if statErr != nil {
				return fmt.Errorf("inspect created directory %s: %w", current, statErr)
			}
		case statErr != nil:
			return fmt.Errorf("inspect directory %s: %w", current, statErr)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("refusing redirected or non-directory path: %s", current)
		}
	}
	return nil
}

func (m *Manager) ensureNoSymlinkParents(path string) error {
	path = filepath.Clean(path)
	if err := m.ensureWithinRoot(path); err != nil {
		return err
	}
	relative, err := filepath.Rel(m.root, path)
	if err != nil {
		return err
	}
	current := m.root
	for _, part := range strings.Split(relative, string(filepath.Separator)) {
		if part == "." || part == "" {
			continue
		}
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		if errors.Is(statErr, fs.ErrNotExist) {
			return nil
		}
		if statErr != nil {
			return fmt.Errorf("inspect path %s: %w", current, statErr)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing symlink or junction path: %s", current)
		}
	}
	return nil
}

func (m *Manager) isSkillDir(path string) (bool, error) {
	if err := m.ensureNoSymlinkParents(path); err != nil {
		return false, err
	}
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect skill directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return false, fmt.Errorf("refusing symlink skill directory: %s", path)
	}
	if !info.IsDir() {
		return false, nil
	}
	skillFile := filepath.Join(path, "SKILL.md")
	fileInfo, err := os.Lstat(skillFile)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect SKILL.md: %w", err)
	}
	if fileInfo.Mode()&os.ModeSymlink != 0 || !fileInfo.Mode().IsRegular() {
		return false, fmt.Errorf("refusing non-regular SKILL.md: %s", skillFile)
	}
	return true, nil
}

func (m *Manager) resolveSupportTarget(skillDir, rawPath string) (string, error) {
	clean, err := normalizeSupportPath(rawPath)
	if err != nil {
		return "", err
	}
	target := filepath.Join(skillDir, filepath.FromSlash(clean))
	if err := m.ensureWithinRoot(target); err != nil {
		return "", err
	}
	relative, err := filepath.Rel(skillDir, target)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", errors.New("file_path escapes the skill directory")
	}
	if err := m.ensureNoSymlinkParents(target); err != nil {
		return "", err
	}
	return target, nil
}

func (m *Manager) readRegularFile(path string) ([]byte, error) {
	if err := m.ensureNoSymlinkParents(path); err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("refusing non-regular file: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return data, nil
}

func (m *Manager) atomicWrite(path string, data []byte) error {
	if err := m.ensureNoSymlinkParents(path); err != nil {
		return err
	}
	if err := m.secureMkdirAll(filepath.Dir(path)); err != nil {
		return err
	}
	mode := fs.FileMode(0o644)
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("refusing non-regular write target: %s", path)
		}
		mode = info.Mode().Perm()
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("inspect write target: %w", err)
	}

	temporary, err := os.CreateTemp(filepath.Dir(path), ".skill-write-*")
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("set temporary file mode: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write temporary file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync temporary file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary file: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}

func (m *Manager) removeSkillTree(path string) error {
	path = filepath.Clean(path)
	if path == m.root {
		return errors.New("refusing to remove the skills root")
	}
	if err := m.ensureWithinRoot(path); err != nil {
		return err
	}
	var entries []string
	err := filepath.WalkDir(path, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing symlink or junction in skill tree: %s", current)
		}
		if !entry.IsDir() && !entry.Type().IsRegular() {
			return fmt.Errorf("refusing special file in skill tree: %s", current)
		}
		entries = append(entries, current)
		return nil
	})
	if err != nil {
		return fmt.Errorf("inspect skill tree: %w", err)
	}
	for index := len(entries) - 1; index >= 0; index-- {
		if err := os.Remove(entries[index]); err != nil {
			return fmt.Errorf("remove %s: %w", entries[index], err)
		}
	}
	return nil
}

func (m *Manager) archiveSkillTree(skillDir string) (string, error) {
	archiveRoot := filepath.Join(m.root, archiveDirName)
	if err := m.secureMkdirAll(archiveRoot); err != nil {
		return "", fmt.Errorf("create skill archive: %w", err)
	}

	destination := filepath.Join(archiveRoot, filepath.Base(skillDir))
	if _, err := os.Lstat(destination); err == nil {
		destination = filepath.Join(
			archiveRoot,
			fmt.Sprintf("%s-%s", filepath.Base(skillDir), time.Now().UTC().Format("20060102150405")),
		)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("inspect skill archive target: %w", err)
	}
	if _, err := os.Lstat(destination); err == nil {
		return "", fmt.Errorf("skill archive target already exists: %s", destination)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("inspect skill archive target: %w", err)
	}
	if err := m.ensureNoSymlinkParents(destination); err != nil {
		return "", err
	}
	if err := os.Rename(skillDir, destination); err != nil {
		return "", fmt.Errorf("archive skill: %w", err)
	}

	m.removeEmptyCategory(filepath.Dir(skillDir))
	return m.relative(destination), nil
}

func (m *Manager) removeEmptyCategory(path string) {
	path = filepath.Clean(path)
	if filepath.Dir(path) != m.root || !validCategoryName(filepath.Base(path)) {
		return
	}
	entries, err := os.ReadDir(path)
	if err == nil && len(entries) == 0 {
		_ = os.Remove(path)
	}
}

func (m *Manager) removeEmptySupportDirs(path, skillDir string) {
	for filepath.Clean(path) != filepath.Clean(skillDir) {
		entries, err := os.ReadDir(path)
		if err != nil || len(entries) != 0 {
			return
		}
		if err := os.Remove(path); err != nil {
			return
		}
		path = filepath.Dir(path)
	}
}

func (m *Manager) snapshot(operations []Operation) (*batchSnapshot, error) {
	root, err := os.MkdirTemp("", "gotack-skill-batch-*")
	if err != nil {
		return nil, fmt.Errorf("create batch snapshot: %w", err)
	}
	snapshot := &batchSnapshot{root: root}
	seen := make(map[string]bool)
	for _, operation := range operations {
		if seen[operation.Name] {
			continue
		}
		seen[operation.Name] = true
		path, err := m.findSkill(operation.Name)
		if err != nil {
			snapshot.cleanup()
			return nil, err
		}
		item := skillSnapshot{Name: operation.Name, OriginalPath: path, Existed: path != ""}
		if item.Existed {
			item.BackupPath = filepath.Join(root, fmt.Sprintf("skill-%d", len(snapshot.skills)))
			if err := copyDirectory(path, item.BackupPath); err != nil {
				snapshot.cleanup()
				return nil, fmt.Errorf("snapshot skill %q: %w", operation.Name, err)
			}
		}
		snapshot.skills = append(snapshot.skills, item)
	}
	return snapshot, nil
}

func (snapshot *batchSnapshot) cleanup() {
	if snapshot != nil && snapshot.root != "" {
		_ = os.RemoveAll(snapshot.root)
	}
}

func (m *Manager) rollback(snapshot *batchSnapshot) error {
	var failures []string
	for _, item := range snapshot.skills {
		current, err := m.findSkill(item.Name)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", item.Name, err))
			continue
		}
		if current != "" {
			if err := m.removeSkillTree(current); err != nil {
				failures = append(failures, fmt.Sprintf("%s: %v", item.Name, err))
				continue
			}
			m.removeEmptyCategory(filepath.Dir(current))
		}
		if item.Existed {
			if err := m.secureMkdirAll(filepath.Dir(item.OriginalPath)); err != nil {
				failures = append(failures, fmt.Sprintf("%s: %v", item.Name, err))
				continue
			}
			if err := copyDirectory(item.BackupPath, item.OriginalPath); err != nil {
				failures = append(failures, fmt.Sprintf("%s: %v", item.Name, err))
			}
		}
	}
	if len(failures) != 0 {
		return errors.New(strings.Join(failures, "; "))
	}
	return nil
}

func copyDirectory(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing symlink in skill tree: %s", path)
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("refusing special file in skill tree: %s", path)
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
		if err != nil {
			_ = input.Close()
			return err
		}
		_, copyErr := io.Copy(output, input)
		inputCloseErr := input.Close()
		closeErr := output.Close()
		if copyErr != nil {
			return copyErr
		}
		if inputCloseErr != nil {
			return inputCloseErr
		}
		return closeErr
	})
}

func (m *Manager) loadOwnership() (map[string]bool, error) {
	data, err := m.readOwnershipManifest()
	if err != nil {
		return nil, err
	}
	if data == nil {
		return make(map[string]bool), nil
	}
	var manifest ownershipManifest
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return nil, fmt.Errorf("decode skill ownership: %w", err)
	}
	if manifest.Version != ownershipVersion {
		return nil, fmt.Errorf("unsupported skill ownership version %d", manifest.Version)
	}
	owned := make(map[string]bool, len(manifest.AgentOwned))
	for _, name := range manifest.AgentOwned {
		if err := validateName(name); err != nil {
			return nil, fmt.Errorf("invalid skill ownership entry: %w", err)
		}
		owned[name] = true
	}
	return owned, nil
}

func (m *Manager) readOwnershipManifest() ([]byte, error) {
	for _, fileName := range []string{ownershipFileName, legacyOwnershipFileName} {
		path := filepath.Join(m.root, fileName)
		data, err := m.readRegularFile(path)
		if errors.Is(rootCause(err), fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read skill ownership %s: %w", fileName, err)
		}
		return data, nil
	}
	return nil, nil
}

func (m *Manager) saveOwnership(owned map[string]bool) error {
	data, err := json.MarshalIndent(ownershipManifest{
		Version:    ownershipVersion,
		AgentOwned: sortedOwned(owned),
	}, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return m.atomicWrite(filepath.Join(m.root, ownershipFileName), data)
}

func rootCause(err error) error {
	for err != nil {
		next := errors.Unwrap(err)
		if next == nil {
			return err
		}
		err = next
	}
	return nil
}

func (m *Manager) requireReadMark(state *applyState, target string) error {
	if !state.meta.BackgroundReview {
		return nil
	}
	if _, err := os.Lstat(target); errors.Is(err, fs.ErrNotExist) {
		return nil
	} else if err != nil {
		return fmt.Errorf("inspect mutation target: %w", err)
	}
	key := readPathKey(target)
	if state.known[key] {
		return nil
	}
	expected, ok := state.readMarks[key]
	if !ok {
		return errors.New("background review must load the exact current file with Crush view via skill_view before mutating it")
	}
	data, err := m.readRegularFile(target)
	if err != nil {
		return err
	}
	if viewDigest(string(data)) != expected {
		return errors.New("background review Crush view via skill_view is stale; call skill_view for the exact current file again")
	}
	return nil
}
