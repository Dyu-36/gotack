// Package officecli copies the bundled officecli binary and the matching
// Crush skill files into the user's Gotack data directory and prepends that
// directory to PATH. Crush inherits the updated PATH via the per-workspace
// `env` config so the agent can invoke `officecli` directly.
package officecli

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Seeder manages the per-user officecli installation.
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

// BinDir is the destination directory for the officecli binary. It is also
// prepended to PATH so Crush can resolve `officecli` without a full path.
func (s *Seeder) BinDir() string {
	return filepath.Join(s.dataDir, "bin")
}

// SkillsDir is the destination directory for Crush skill files. It is
// registered under `options.skills_paths` so the agent loads them on startup.
func (s *Seeder) SkillsDir() string {
	return filepath.Join(s.dataDir, "skills")
}

// OfficeCLIPath returns the absolute path to the bundled officecli binary or
// the empty string when it is not available.
func (s *Seeder) OfficeCLIPath() string {
	name := "officecli"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	candidate := filepath.Join(s.BinDir(), name)
	if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
		return candidate
	}
	return ""
}

// Seed copies the bundled binary and skill tree into the data directory.
// It is safe to call repeatedly: each file is only rewritten when the source
// is missing or differs in size.
func (s *Seeder) Seed(sourceDir string) error {
	if sourceDir == "" {
		return nil
	}
	if err := os.MkdirAll(s.BinDir(), 0o755); err != nil {
		return fmt.Errorf("create bin dir: %w", err)
	}
	if err := copyDirIfChanged(sourceDir, s.BinDir()); err != nil {
		return fmt.Errorf("copy bin tree: %w", err)
	}
	if err := os.MkdirAll(s.SkillsDir(), 0o755); err != nil {
		return fmt.Errorf("create skills dir: %w", err)
	}
	skillsSource := filepath.Join(sourceDir, "skills")
	if info, err := os.Stat(skillsSource); err == nil && info.IsDir() {
		if err := copyDirIfChanged(skillsSource, s.SkillsDir()); err != nil {
			return fmt.Errorf("copy skills tree: %w", err)
		}
	}
	return nil
}

// InstallPath prepends the bin directory to the current process PATH so any
// child process (including the Crush engine) can resolve `officecli`.
func (s *Seeder) InstallPath() {
	bin := s.BinDir()
	current := os.Getenv("PATH")
	entries := filepath.SplitList(current)
	for _, entry := range entries {
		if strings.EqualFold(entry, bin) {
			return
		}
	}
	prepended := append([]string{bin}, entries...)
	if joined := strings.Join(prepended, string(os.PathListSeparator)); joined != "" {
		_ = os.Setenv("PATH", joined)
	}
}

// CrushEnv returns the environment overrides Crush must apply to every
// workspace so the agent shell sees the bundled officecli on PATH.
func (s *Seeder) CrushEnv() map[string]string {
	path := os.Getenv("PATH")
	if path == "" {
		path = s.BinDir()
	}
	return map[string]string{"PATH": path}
}

// SkillsPathArg returns the path value Crush expects inside options.skills_paths.
func (s *Seeder) SkillsPathArg() string {
	return s.SkillsDir()
}

// EnsureOfficeCLIOnPath locates the officecli binary on disk, falling back to
// any PATH-resident install. The returned path is absolute.
func EnsureOfficeCLIOnPath() string {
	name := "officecli"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if found, err := exec.LookPath(name); err == nil {
		if abs, err := filepath.Abs(found); err == nil {
			return abs
		}
		return found
	}
	if local := os.Getenv("LOCALAPPDATA"); local != "" && runtime.GOOS == "windows" {
		candidate := filepath.Join(local, "OfficeCLI", name)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

// report holds the persisted bundle version used to skip unchanged files.
type report struct {
	Files map[string]int64 `json:"files"`
}

func copyDirIfChanged(source, destination string) error {
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
		previous, present := state.Files[rel]
		if present && previous == info.Size() {
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
