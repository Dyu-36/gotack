package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const importReportName = ".legacy-import-v1.json"

type ImportReport struct {
	Version     int      `json:"version"`
	Imported    []string `json:"imported,omitempty"`
	NeedsReview []string `json:"needs_review,omitempty"`
	BackupDir   string   `json:"backup_dir,omitempty"`
}

// EnsureAssistant is a one-time, additive import. Original files are never
// removed, existing assistant files are never overwritten, and retry is safe.
// Custom root USER/TACK instructions are archived for explicit review, not
// promoted to system policy or blindly merged with the personal profile.
func EnsureAssistant(dataDir string) error {
	dir := Directory(dataDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	release, err := acquireFileLock(ctx, filepath.Join(dir, ".legacy-import.lock"))
	if err != nil {
		return err
	}
	defer release()
	reportPath := filepath.Join(dir, importReportName)
	if _, err := os.Stat(reportPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	report := ImportReport{Version: 1, BackupDir: filepath.Join(dataDir, "assistant-import-backups")}
	old := filepath.Join(dataDir, "context")
	for _, item := range []struct {
		source string
		backup string
		target Target
	}{
		{filepath.Join(old, "memory", "USER.md"), "memory-USER.md", TargetUser},
		{filepath.Join(old, "memory", MemoryFileName), "memory-MEMORY.md", TargetMemory},
		{filepath.Join(old, "USER.md"), "context-USER.md", ""},
		{filepath.Join(old, "TACK.md"), "context-TACK.md", ""},
	} {
		info, err := os.Lstat(item.source)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			report.NeedsReview = append(report.NeedsReview, item.source)
			continue
		}
		if err := copyLegacyBackup(item.source, filepath.Join(report.BackupDir, item.backup)); err != nil {
			return fmt.Errorf("backup legacy personal context: %w", err)
		}
		if item.target == "" {
			if info.Size() > 0 {
				report.NeedsReview = append(report.NeedsReview, item.source)
			}
			continue
		}
		destination := filepath.Join(dir, FileNameFor(item.target))
		// Use the normal target lock, so a concurrent MCP writer cannot race import.
		unlock, err := acquireFileLock(ctx, destination+".lock")
		if err != nil {
			return err
		}
		err = importPersonalFile(item.source, destination, item.target, &report)
		unlock()
		if err != nil {
			return err
		}
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(reportPath, append(data, '\n'))
}

func importPersonalFile(source, destination string, target Target, report *ImportReport) error {
	if _, err := os.Lstat(destination); err == nil {
		// A new-format profile has priority; the original remains available.
		report.NeedsReview = append(report.NeedsReview, source)
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	data, err := ReadPromptFile(source)
	if err != nil {
		report.NeedsReview = append(report.NeedsReview, source)
		return nil
	}
	text, omitted, err := BoundedPrompt(target, data)
	if err != nil {
		report.NeedsReview = append(report.NeedsReview, source)
		return nil
	}
	if omitted > 0 || text != Render(target, serializeEntries(parseFile(string(data)).Entries)) {
		report.NeedsReview = append(report.NeedsReview, source)
	}
	if err := writeFileAtomic(destination, []byte(text)); err != nil {
		return err
	}
	report.Imported = append(report.Imported, destination)
	return nil
}

func copyLegacyBackup(source, destination string) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return err
	}
	// Keep the first backup on retries. The original also remains untouched.
	if _, err := os.Lstat(destination); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	temp, err := os.CreateTemp(filepath.Dir(destination), ".import-*")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	_, err = io.Copy(temp, input)
	if err == nil {
		err = temp.Sync()
	}
	if closeErr := temp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(temp.Name(), destination)
}

func ReadImportReport(dataDir string) (ImportReport, error) {
	data, err := ReadPromptFile(filepath.Join(Directory(dataDir), importReportName))
	if err != nil {
		return ImportReport{}, err
	}
	if len(data) == 0 {
		return ImportReport{}, nil
	}
	var report ImportReport
	err = json.Unmarshal(data, &report)
	return report, err
}
