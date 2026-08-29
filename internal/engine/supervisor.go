package engine

import (
	"fmt"
	"log/slog"
	"os/exec"
	"sync"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/crushapi"
)

// supervisor.go -- role: launch, adopt and stop the crush child process.
//
// Process model: gotack (UI + host) -> crush server as a separate process.
// Ownership is tracked so an adopted server is never terminated on UI exit.

type Status string

const (
	StatusStopped  Status = "stopped"
	StatusStarting Status = "starting"
	StatusRunning  Status = "running"
	StatusError    Status = "error"
)

type Supervisor struct {
	log    *slog.Logger
	binary string

	mu       sync.Mutex
	cmd      *exec.Cmd
	owned    bool
	endpoint crushapi.Endpoint
}

// NewSupervisor returns a Supervisor. EngineBinary is an explicit override;
// otherwise the release resolver prefers bundled resources/crush(.exe) and
// falls back to an externally installed crush on PATH.
func NewSupervisor(log *slog.Logger, binary string) *Supervisor {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	if binary == "" {
		binary = defaultBinary()
	}
	return &Supervisor{log: log, binary: binary}
}

func (s *Supervisor) Owned() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.owned
}

// Start launches `<binary> server` as a child process. The child lifetime is
// owned by this supervisor and is not tied to an attach/reconnect context.
func (s *Supervisor) Start() (crushapi.Endpoint, error) {
	s.mu.Lock()
	if s.cmd != nil && s.cmd.Process != nil {
		running := s.endpoint
		s.mu.Unlock()
		return running, fmt.Errorf("engine: already started")
	}
	bin := s.binary
	s.mu.Unlock()

	ep := appconfig.PipeEndpoint()
	cmd := exec.Command(bin, "server")
	configureProcAttr(cmd)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	if err := cmd.Start(); err != nil {
		return crushapi.Endpoint{}, fmt.Errorf("engine: start %s: %w", bin, err)
	}

	s.mu.Lock()
	s.cmd = cmd
	s.owned = true
	s.endpoint = ep
	s.mu.Unlock()

	s.log.Info("engine: started", "binary", bin, "pid", cmd.Process.Pid, "endpoint", ep)
	return ep, nil
}

// Stop terminates the owned child process tree, if any. Adopted servers never
// populate cmd/owned and therefore cannot be killed by Gotack shutdown.
func (s *Supervisor) Stop() error {
	s.mu.Lock()
	cmd := s.cmd
	if cmd == nil || cmd.Process == nil || !s.owned {
		s.mu.Unlock()
		return nil
	}
	s.cmd = nil
	s.owned = false
	s.mu.Unlock()

	if err := killTree(cmd); err != nil {
		_ = cmd.Process.Kill()
		return fmt.Errorf("engine: stop: %w", err)
	}
	_, _ = cmd.Process.Wait()
	return nil
}

// Compile-time check that *Supervisor satisfies EngineAPI. If a method
// signature changes, the build breaks here first.
var _ EngineAPI = (*Supervisor)(nil)
