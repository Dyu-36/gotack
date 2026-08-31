// Package contextseed copies the bundled context files (the Tack persona and
// later agent memory files) into the user's Gotack data directory. Crush
// ingests the directory through options.global_context_paths and re-reads
// every file on each prompt construction, so seeding once at startup is
// enough for the engine to pick up updates on the next turn.
package contextseed

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// Seeder manages the per-user context directory.
type Seeder struct {
	dataDir string
	log     *slog.Logger
}

// New returns a Seeder rooted at the Gotack data directory.
func New(dataDir string, log *slog.Logger) *Seeder {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Seeder{dataDir: dataDir, log: log}
}

// ContextDir is the destination directory for the seeded context files. It is
// registered under options.global_context_paths so the engine injects the
// files into the system prompt.
func (s *Seeder) ContextDir() string {
	return filepath.Join(s.dataDir, "context")
}

// ContextPathArg returns the path value Crush expects inside
// options.global_context_paths.
func (s *Seeder) ContextPathArg() string {
	return s.ContextDir()
}

// Seed copies the bundled context tree into the data directory. It is safe to
// call repeatedly: unchanged files are skipped, updated bundled files are
// propagated, and files the user has modified after seeding are preserved.
func (s *Seeder) Seed(sourceDir string) error {
	if sourceDir == "" {
		return nil
	}
	if err := os.MkdirAll(s.ContextDir(), 0o755); err != nil {
		return fmt.Errorf("create context dir: %w", err)
	}
	if err := s.copyDirIfChanged(sourceDir, s.ContextDir()); err != nil {
		return fmt.Errorf("copy context tree: %w", err)
	}
	return nil
}

// report holds the persisted bundle version used to skip unchanged files.
type report struct {
	Files map[string]int64 `json:"files"`
}

// copyDirIfChanged mirrors officecli's size-keyed seeding with one extra
// rule: a file present on disk that the seeder did not write, or no longer
// matches what it last wrote, is treated as user-owned and left untouched.
// The persona is advisory text; silently overwriting user edits there would
// destroy exactly the customization the context seam exists to carry.
func (s *Seeder) copyDirIfChanged(source, destination string) error {
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
		seederSize, present := state.Files[rel]
		targetInfo, targetErr := os.Stat(target)
		if targetErr != nil {
			// Missing destination: first seed of this file, or the user
			// deleted it. Either way the bundled copy is authoritative.
			if err := copyFile(path, target, info.Mode()); err != nil {
				return err
			}
			state.Files[rel] = info.Size()
			updated = true
			return nil
		}
		if !present {
			// On disk but never recorded: the file predates the seeder, so
			// it belongs to the user.
			s.log.Info("contextseed: preserving user file never written by the seeder", "file", rel)
			return nil
		}
		if seederSize != targetInfo.Size() {
			// Recorded but no longer matching what was written: user edit.
			s.log.Info("contextseed: preserving user-modified file", "file", rel)
			return nil
		}
		if seederSize == info.Size() {
			return nil
		}
		// Bundled source changed and the destination is still the seeder's
		// own copy, so propagating cannot lose user work.
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
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}

func loadReport(destination string) (report, error) {
	report := report{Files: map[string]int64{}}
	data, err := os.ReadFile(filepath.Join(destination, ".seed-report.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return report, nil
		}
		return report, err
	}
	if len(data) == 0 {
		return report, nil
	}
	_ = json.Unmarshal(data, &report)
	if report.Files == nil {
		report.Files = map[string]int64{}
	}
	return report, nil
}

func saveReport(destination string, value report) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(destination, ".seed-report.json"), data, 0o644)
}
