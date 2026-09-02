// Package contextseed copies the bundled context files (the Tack persona and
// later agent memory files) into the user's Gotack data directory. Crush reads
// an immutable prompt projection produced by BuildPromptSnapshot; writable
// memory files never enter the engine's recursive context walk directly.
package contextseed

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/Dyu-36/gotack/internal/bundleseed"
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

// ContextDir is the writable destination directory for seeded context files.
// It is deliberately not registered under options.global_context_paths.
func (s *Seeder) ContextDir() string {
	return filepath.Join(s.dataDir, "context")
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
