package bundleseed

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const reportFileName = ".seed-report.json"

type ExistingFilePolicy uint8

const (
	ManagedFiles ExistingFilePolicy = iota
	UserEditableFiles
)

type PreserveReason uint8

const (
	UntrackedFile PreserveReason = iota
	ModifiedFile
)

type Options struct {
	ExistingFiles ExistingFilePolicy
	OnPreserve    func(path string, reason PreserveReason)
}

type report struct {
	Files  map[string]int64  `json:"files"`
	Hashes map[string]string `json:"hashes,omitempty"`
}

func CopyIfChanged(source, destination string, options Options) error {
	if options.ExistingFiles != ManagedFiles && options.ExistingFiles != UserEditableFiles {
		return fmt.Errorf("bundleseed: unsupported existing-file policy %d", options.ExistingFiles)
	}
	state, err := loadReport(destination)
	if err != nil {
		return err
	}
	updated := false
	sourceFiles := map[string]struct{}{}
	err = filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		sourceFiles[rel] = struct{}{}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		sourceHash, err := fileHash(path)
		if err != nil {
			return fmt.Errorf("bundleseed: hash %s: %w", path, err)
		}
		previous, tracked := state.Files[rel]
		previousHash := state.Hashes[rel]
		targetInfo, targetErr := os.Stat(target)
		if targetErr != nil && !os.IsNotExist(targetErr) {
			return fmt.Errorf("bundleseed: stat %s: %w", target, targetErr)
		}

		var targetHash string
		if targetErr == nil {
			targetHash, err = fileHash(target)
			if err != nil {
				return fmt.Errorf("bundleseed: hash %s: %w", target, err)
			}
		}

		if targetErr == nil && options.ExistingFiles == UserEditableFiles {
			switch {
			case !tracked:
				if options.OnPreserve != nil {
					options.OnPreserve(rel, UntrackedFile)
				}
				return nil
			case previous != targetInfo.Size():
				if options.OnPreserve != nil {
					options.OnPreserve(rel, ModifiedFile)
				}
				return nil
			case previousHash != "" && previousHash != targetHash:
				if options.OnPreserve != nil {
					options.OnPreserve(rel, ModifiedFile)
				}
				return nil
			case previousHash == "" && targetHash != sourceHash:
				if options.OnPreserve != nil {
					options.OnPreserve(rel, ModifiedFile)
				}
				return nil
			}
		}

		if targetErr == nil && targetHash == sourceHash {
			if previous != info.Size() || previousHash != sourceHash {
				state.Files[rel] = info.Size()
				state.Hashes[rel] = sourceHash
				updated = true
			}
			return nil
		}
		if err := copyFile(path, target, info.Mode()); err != nil {
			return err
		}
		state.Files[rel] = info.Size()
		state.Hashes[rel] = sourceHash
		updated = true
		return nil
	})
	if err != nil {
		return err
	}
	for rel, previousSize := range state.Files {
		if _, exists := sourceFiles[rel]; exists {
			continue
		}
		target := filepath.Join(destination, rel)
		info, statErr := os.Stat(target)
		if statErr != nil && !os.IsNotExist(statErr) {
			return fmt.Errorf("bundleseed: stat removed file %s: %w", target, statErr)
		}
		if statErr == nil && options.ExistingFiles == UserEditableFiles {
			targetHash, hashErr := fileHash(target)
			if hashErr != nil {
				return fmt.Errorf("bundleseed: hash removed file %s: %w", target, hashErr)
			}
			previousHash := state.Hashes[rel]
			if info.Size() != previousSize || previousHash != "" && targetHash != previousHash {
				if options.OnPreserve != nil {
					options.OnPreserve(rel, ModifiedFile)
				}
				delete(state.Files, rel)
				delete(state.Hashes, rel)
				updated = true
				continue
			}
		}
		if statErr == nil {
			if err := os.Remove(target); err != nil {
				return fmt.Errorf("bundleseed: remove stale file %s: %w", target, err)
			}
			if err := removeEmptyParents(filepath.Dir(target), destination); err != nil {
				return err
			}
		}
		delete(state.Files, rel)
		delete(state.Hashes, rel)
		updated = true
	}
	if updated {
		return saveReport(destination, state)
	}
	return nil
}

func removeEmptyParents(dir, root string) error {
	root = filepath.Clean(root)
	for dir = filepath.Clean(dir); dir != root; dir = filepath.Dir(dir) {
		rel, err := filepath.Rel(root, dir)
		if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil
		}
		if err := os.Remove(dir); err != nil {
			return nil
		}
	}
	return nil
}

func fileHash(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func copyFile(source, destination string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

func loadReport(destination string) (report, error) {
	state := report{Files: map[string]int64{}, Hashes: map[string]string{}}
	path := filepath.Join(destination, reportFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return state, nil
		}
		return report{}, fmt.Errorf("bundleseed: read %s: %w", path, err)
	}
	if len(data) == 0 {
		return state, nil
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return report{}, fmt.Errorf("bundleseed: parse %s: %w", path, err)
	}
	if state.Files == nil {
		state.Files = map[string]int64{}
	}
	if state.Hashes == nil {
		state.Hashes = map[string]string{}
	}
	return state, nil
}

func saveReport(destination string, value report) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("bundleseed: encode report: %w", err)
	}
	temp, err := os.CreateTemp(destination, ".seed-report-*.tmp")
	if err != nil {
		return fmt.Errorf("bundleseed: create report temp: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(0o644); err != nil {
		_ = temp.Close()
		return fmt.Errorf("bundleseed: chmod report temp: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return fmt.Errorf("bundleseed: write report temp: %w", err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return fmt.Errorf("bundleseed: sync report temp: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("bundleseed: close report temp: %w", err)
	}
	path := filepath.Join(destination, reportFileName)
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("bundleseed: replace %s: %w", path, err)
	}
	return nil
}
