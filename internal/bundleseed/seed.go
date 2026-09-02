// Package bundleseed installs bundled resource trees using a size-keyed report.
package bundleseed

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const reportFileName = ".seed-report.json"

// ExistingFilePolicy defines who owns files already present at the destination.
type ExistingFilePolicy uint8

const (
	// ManagedFiles lets the bundle restore files that differ from its report.
	ManagedFiles ExistingFilePolicy = iota
	// UserEditableFiles preserves untracked files and tracked files modified by a user.
	UserEditableFiles
)

// PreserveReason explains why a user-editable destination was not replaced.
type PreserveReason uint8

const (
	UntrackedFile PreserveReason = iota
	ModifiedFile
)

// Options selects destination ownership and optional preservation reporting.
type Options struct {
	ExistingFiles ExistingFilePolicy
	OnPreserve    func(path string, reason PreserveReason)
}

type report struct {
	Files map[string]int64 `json:"files"`
}

// CopyIfChanged copies source into destination and atomically updates its report.
// A malformed existing report stops the operation before destination files are
// inspected or changed.
func CopyIfChanged(source, destination string, options Options) error {
	if options.ExistingFiles != ManagedFiles && options.ExistingFiles != UserEditableFiles {
		return fmt.Errorf("bundleseed: unsupported existing-file policy %d", options.ExistingFiles)
	}
	state, err := loadReport(destination)
	if err != nil {
		return err
	}
	updated := false
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
		info, err := entry.Info()
		if err != nil {
			return err
		}
		previous, tracked := state.Files[rel]
		targetInfo, targetErr := os.Stat(target)
		if targetErr != nil && !os.IsNotExist(targetErr) {
			return fmt.Errorf("bundleseed: stat %s: %w", target, targetErr)
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
			}
		}
		if targetErr == nil && tracked && previous == info.Size() && previous == targetInfo.Size() {
			return nil
		}
		if err := copyFile(path, target, info.Mode()); err != nil {
			return err
		}
		state.Files[rel] = info.Size()
		updated = true
		return nil
	})
	if err != nil {
		return err
	}
	if updated {
		return saveReport(destination, state)
	}
	return nil
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
	state := report{Files: map[string]int64{}}
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
