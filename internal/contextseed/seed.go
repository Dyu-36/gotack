package contextseed

import (
	"log/slog"
	"sync"

	"github.com/Dyu-36/gotack/internal/memory"
)

const managedCoreName = "TACK_CORE.md"

type Seeder struct {
	dataDir string
	log     *slog.Logger
	mu      sync.Mutex

	snapshotReaders  map[string]string
	currentSnapshot  string
	previousSnapshot string
	stats            PromptStats
}

func New(dataDir string, log *slog.Logger) *Seeder {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Seeder{dataDir: dataDir, log: log}
}

func (s *Seeder) ContextDir() string { return memory.Directory(s.dataDir) }

// Seed keeps the host integration stable while removing external prompt bundles.
// The old sourceDir argument is deliberately ignored; product policy is embedded.
func (s *Seeder) Seed(_ string) error { return memory.EnsureAssistant(s.dataDir) }

// Each (workspace, run) owns one reader registration, not a reference count.
func (s *Seeder) AcquireReader(workspace, run, generation string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.snapshotReaders == nil {
		s.snapshotReaders = make(map[string]string)
	}
	s.snapshotReaders[readerKey(workspace, run)] = generation
	return generation
}

func (s *Seeder) ReleaseReader(workspace, run string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.snapshotReaders, readerKey(workspace, run))
}

func (s *Seeder) RegisteredReaders() map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]string, len(s.snapshotReaders))
	for key, generation := range s.snapshotReaders {
		out[key] = generation
	}
	return out
}

func readerKey(workspace, run string) string { return workspace + "\x00" + run }
