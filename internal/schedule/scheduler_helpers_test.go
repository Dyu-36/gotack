package schedule

import "testing"

func (s *Scheduler) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}

func (s *Scheduler) inflightCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.inflight)
}

func TestLoadPreservesConsecutiveFailures(t *testing.T) {
	path := t.TempDir() + "/" + FileName
	file := &File{Jobs: []*Job{{
		ID:                  "job1",
		Prompt:              "work",
		Every:               "10m",
		Enabled:             true,
		ConsecutiveFailures: 2,
		DisabledReason:      "stale reason",
	}}}
	if err := SaveFile(path, file, base2026()); err != nil {
		t.Fatal(err)
	}

	s := New(path, Runtime{}, nil)
	s.now = base2026
	if err := s.load(); err != nil {
		t.Fatal(err)
	}
	job := s.file.Jobs[0]
	if job.ConsecutiveFailures != 2 {
		t.Fatalf("consecutive failures = %d, want 2", job.ConsecutiveFailures)
	}
	if job.DisabledReason != "" {
		t.Fatalf("enabled job kept stale disabled reason %q", job.DisabledReason)
	}
}
