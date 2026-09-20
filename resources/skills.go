package resources

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

//go:embed skills/timetable/SKILL.md skills/timetable/assets/*
var bundledSkills embed.FS

var installMu sync.Mutex

func InstallSkills(configDir string) (string, error) {
	if !filepath.IsAbs(configDir) {
		return "", fmt.Errorf("bundled skills require an absolute configuration directory")
	}
	installMu.Lock()
	defer installMu.Unlock()

	files := make(map[string][]byte)
	digest := sha256.New()
	err := fs.WalkDir(bundledSkills, "skills", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		data, err := bundledSkills.ReadFile(path)
		if err != nil {
			return err
		}
		name := path[len("skills/"):]
		files[name] = data
		sum := sha256.Sum256(data)
		fmt.Fprintf(digest, "%s\x00%x\n", name, sum)
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("read bundled skills: %w", err)
	}
	base := filepath.Join(configDir, "bundled-skills")
	root := filepath.Join(base, hex.EncodeToString(digest.Sum(nil)))
	if installedSkillsMatch(root, files) {
		return root, nil
	}
	if err := os.MkdirAll(base, 0o700); err != nil {
		return "", fmt.Errorf("create bundled skills directory: %w", err)
	}
	staging, err := os.MkdirTemp(base, ".install-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(staging)
	for name, data := range files {
		path := filepath.Join(staging, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return "", err
		}
	}
	old := staging + ".previous"
	hadPrevious := false
	if _, err := os.Lstat(root); err == nil {
		if err := os.Rename(root, old); err != nil {
			return "", fmt.Errorf("replace damaged bundled skills: %w", err)
		}
		hadPrevious = true
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if err := os.Rename(staging, root); err != nil {
		if hadPrevious {
			if restoreErr := os.Rename(old, root); restoreErr != nil {
				return "", fmt.Errorf("install bundled skills: %w; previous resources retained at %s: %v", err, old, restoreErr)
			}
		}
		return "", fmt.Errorf("install bundled skills: %w", err)
	}
	if hadPrevious {
		_ = os.RemoveAll(old)
	}
	return root, nil
}

func installedSkillsMatch(root string, files map[string][]byte) bool {
	seen := 0
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("non-regular bundled resource: %s", path)
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		expected, ok := files[filepath.ToSlash(rel)]
		if !ok {
			return fmt.Errorf("unexpected bundled resource: %s", rel)
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		actual, readErr := io.ReadAll(io.LimitReader(file, int64(len(expected))+1))
		closeErr := file.Close()
		if readErr != nil {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}
		if !bytes.Equal(actual, expected) {
			return fmt.Errorf("damaged bundled resource: %s", rel)
		}
		seen++
		return nil
	})
	return err == nil && seen == len(files)
}
