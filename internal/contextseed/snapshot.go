package contextseed

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Dyu-36/gotack/internal/memory"
)

const (
	promptSnapshotRoot = "context-prompt"
	snapshotPrefix     = "snapshot-"
)

func (s *Seeder) PromptContextRoot() string {
	return filepath.Join(s.dataDir, promptSnapshotRoot)
}

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
	err = filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
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
		if rel == ".seed-report.json" || rel == stockManifestName {
			return nil
		}
		if (status.Mode == MigrationPending || status.Mode == MigrationLegacy || status.Mode == MigrationRolledBack) && (rel == managedCoreName || rel == userContextName) {
			return nil
		}
		if status.Mode == MigrationCommitted && rel == legacyContextName {
			return nil
		}
		if isMemoryPath(rel) {
			if entry.IsDir() {
				return nil
			}
			return s.snapshotMemoryFile(path, rel, entry, staging)
		}
		if entry.IsDir() {
			return os.MkdirAll(filepath.Join(staging, rel), 0o755)
		}
		return copyPromptFile(path, filepath.Join(staging, rel), entry)
	})
	if err != nil {
		return "", fmt.Errorf("build prompt snapshot: %w", err)
	}

	final := filepath.Join(root, snapshotPrefix+fmt.Sprintf("%d", time.Now().UnixNano()))
	if err := os.Rename(staging, final); err != nil {
		return "", fmt.Errorf("commit prompt snapshot: %w", err)
	}
	removeStaging = false
	return final, nil
}

func isMemoryPath(rel string) bool {
	rel = filepath.Clean(rel)
	memoryPrefix := "memory" + string(filepath.Separator)
	return rel == "memory" || strings.HasPrefix(rel, memoryPrefix)
}

func (s *Seeder) snapshotMemoryFile(source, rel string, entry os.DirEntry, staging string) error {
	base := filepath.Base(rel)
	var target memory.Target
	switch base {
	case memory.MemoryFileName:
		target = memory.TargetMemory
	case memory.UserFileName:
		target = memory.TargetUser
	default:

		return nil
	}
	data, err := os.ReadFile(source)
	if err != nil {
		s.log.Warn("contextseed: skipping unreadable memory file", "file", rel, "err", err)
		return nil
	}
	content, err := memory.SanitizeFileForPrompt(target, data)
	if err != nil {
		s.log.Warn("contextseed: skipping invalid memory file", "file", rel, "err", err)
		return nil
	}
	if content == "" {
		return nil
	}
	return writePromptFile(filepath.Join(staging, rel), []byte(content), entry)
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

func copyPromptFile(source, destination string, entry os.DirEntry) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return nil
	}
	return writePromptFile(destination, data, entry)
}

func writePromptFile(destination string, data []byte, entry os.DirEntry) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	mode := os.FileMode(0o644)
	if info, err := entry.Info(); err == nil && info.Mode().Perm() != 0 {
		mode = info.Mode().Perm()
	}
	return os.WriteFile(destination, data, mode)
}

func (s *Seeder) PrunePromptSnapshots(keep string) {
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
		_ = os.RemoveAll(path)
	}
}
