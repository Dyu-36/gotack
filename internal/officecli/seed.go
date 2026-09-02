// Package officecli copies the bundled officecli binary and the matching
// Crush skill files into the user's Gotack data directory and prepends that
// directory to PATH. Crush inherits the updated PATH via the per-workspace
// `env` config so the agent can invoke `officecli` directly.
package officecli

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Dyu-36/gotack/internal/bundleseed"
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

// Seed copies the bundled binary and skill tree into the data directory.
// It is safe to call repeatedly: files are only rewritten when they are absent
// from the destination or their recorded bundled size changes.
func (s *Seeder) Seed(sourceDir string) error {
	if sourceDir == "" {
		return nil
	}
	if err := os.MkdirAll(s.BinDir(), 0o755); err != nil {
		return fmt.Errorf("create bin dir: %w", err)
	}
	options := bundleseed.Options{ExistingFiles: bundleseed.ManagedFiles}
	if err := bundleseed.CopyIfChanged(sourceDir, s.BinDir(), options); err != nil {
		return fmt.Errorf("copy bin tree: %w", err)
	}
	if err := os.MkdirAll(s.SkillsDir(), 0o755); err != nil {
		return fmt.Errorf("create skills dir: %w", err)
	}
	skillsSource := filepath.Join(sourceDir, "skills")
	if info, err := os.Stat(skillsSource); err == nil && info.IsDir() {
		if err := bundleseed.CopyIfChanged(skillsSource, s.SkillsDir(), options); err != nil {
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
