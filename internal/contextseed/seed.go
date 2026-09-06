package contextseed

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/Dyu-36/gotack/internal/bundleseed"
)

type Seeder struct {
	dataDir   string
	log       *slog.Logger
	mu        sync.Mutex
	sourceDir string
	// snapshotReaders tracks every (workspace, run) currently holding a
	// reader on a committed snapshot generation. Each entry maps the
	// reader key to the absolute path of the generation the reader is
	// still using. Retention follows the actual reader set: prune
	// keeps any generation that still has a live reader plus the
	// freshly committed revision. There is no arbitrary
	// two-generation cap; the cardinality is bounded by the number
	// of distinct reader holders at any moment.
	snapshotReaders  map[string]string
	currentSnapshot  string
	previousSnapshot string
}

// AcquireReader registers a (workspace, run) reader against the given
// snapshot generation. The caller MUST eventually call ReleaseReader
// with the same key; the entry is removed when its refcount reaches
// zero. While any reader holds the entry, the corresponding
// generation is kept alive across PrunePromptSnapshots. The returned
// generation path is the path the caller should pass to the engine
// (and eventually release).
func (s *Seeder) AcquireReader(workspace, run, generation string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.snapshotReaders == nil {
		s.snapshotReaders = make(map[string]string)
	}
	s.snapshotReaders[readerKey(workspace, run)] = generation
	return generation
}

// ReleaseReader drops the (workspace, run) reader registration. The
// generation becomes prunable once every reader that holds it has
// released. Unknown keys are no-ops; double-release is safe.
func (s *Seeder) ReleaseReader(workspace, run string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.snapshotReaders, readerKey(workspace, run))
}

// RegisteredReaders reports the active reader registrations. Test
// helpers use this to assert the registry state; production callers
// should rely on PrunePromptSnapshots which reads the registry under
// the same mutex.
func (s *Seeder) RegisteredReaders() map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]string, len(s.snapshotReaders))
	for k, v := range s.snapshotReaders {
		out[k] = v
	}
	return out
}

func readerKey(workspace, run string) string {
	return workspace + "\x00" + run
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
	if _, coreErr := os.Stat(filepath.Join(sourceDir, managedCoreName)); coreErr == nil {
		if _, manifestErr := os.Stat(filepath.Join(sourceDir, stockManifestName)); manifestErr == nil {
			return s.seedLayered(sourceDir)
		}
	}
	// Compatibility for old/custom resource bundles that still contain only
	// legacy TACK.md. Current Gotack releases always take the layered path.
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
