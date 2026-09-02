package contextseed

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

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

func (s *Seeder) ContextDir() string {
	return filepath.Join(s.dataDir, "context")
}

func (s *Seeder) Seed(sourceDir string) error {
	if sourceDir == "" {
		return nil
	}
	if err := os.MkdirAll(s.ContextDir(), 0o755); err != nil {
		return fmt.Errorf("create context dir: %w", err)
	}
	options := bundleseed.Options{
		ExistingFiles: bundleseed.UserEditableFiles,
		OnPreserve:    s.logPreserved,
	}
	if err := bundleseed.CopyIfChanged(sourceDir, s.ContextDir(), options); err != nil {
		return fmt.Errorf("copy context tree: %w", err)
	}
	return nil
}

func (s *Seeder) logPreserved(path string, reason bundleseed.PreserveReason) {
	message := "contextseed: preserving user-modified file"
	if reason == bundleseed.UntrackedFile {
		message = "contextseed: preserving user file never written by the seeder"
	}
	s.log.Info(message, "file", path)
}
