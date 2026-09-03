package officecli

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/Dyu-36/gotack/internal/bundleseed"
)

type Seeder struct {
	dataDir string
	log     *slog.Logger
}

func New(dataDir string, log *slog.Logger) *Seeder {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Seeder{dataDir: dataDir, log: log}
}

func (s *Seeder) BinDir() string {
	return filepath.Join(s.dataDir, "bin")
}

func (s *Seeder) SkillsDir() string {
	return filepath.Join(s.dataDir, "skills")
}

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

func (s *Seeder) CrushEnv() map[string]string {
	path := os.Getenv("PATH")
	if path == "" {
		path = s.BinDir()
	}
	return map[string]string{"PATH": path}
}

func (s *Seeder) SkillsPathArg() string {
	return s.SkillsDir()
}
