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
//
// The lifecycle status is owned by App, not here. This file used to keep its
// own status/lastErr pair behind three getters, but nothing outside the package
// ever read them: App already holds a.status and a.lastError and is what emits
// engine:status to the UI. A second copy of the same state machine could only
// drift, so it is gone and the Status type below is purely shared vocabulary.

// Status reports the engine lifecycle. The string values are part of the
// public contract: the UI binds to them through internal/uievents
// EngineStatus events.
type Status string

const (
	StatusStopped  Status = "stopped"
	StatusStarting Status = "starting"
	StatusRunning  Status = "running"
	StatusError    Status = "error"
)

// Supervisor owns the optional crush child process started by this host. All
// exported methods are safe for concurrent use; mu guards cmd, owned and the
// endpoint the child was launched on or adopted at.
type Supervisor struct {
	log    *slog.Logger
	binary string

	mu       sync.Mutex
	cmd      *exec.Cmd
	owned    bool
	endpoint crushapi.Endpoint
}

// NewSupervisor returns a Supervisor that will launch `binary` (or "crush"
// when binary is empty, resolved against PATH) as `binary server`. The logger
// is used for lifecycle events; passing nil is tolerated and discards output.
func NewSupervisor(log *slog.Logger, binary string) *Supervisor {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	if binary == "" {
		binary = "crush"
	}
	return &Supervisor{
		log:    log,
		binary: binary,
	}
}

// Owned reports whether the supervisor has launched a child process itself.
// Adopted servers (found via Locate) are not owned and must not be killed.
func (s *Supervisor) Owned() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.owned
}

// Start launches `<binary> server` as a child process and returns the endpoint
// it expects to listen on. The child is recorded as owned; Stop will terminate
// the process tree. Already-started instances return an error together with
// the endpoint already in use.
//
// Start takes no context on purpose. The child's lifetime belongs to Stop, not
// to whichever attach attempt happened to launch it. Binding it to a
// cancellable attach scope through exec.CommandContext would let a Reconnect
// kill a healthy engine mid-run, which is exactly what the two-process model
// exists to prevent.
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
	// Spec: capture nothing -- the child writes its own log file.
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

// Stop terminates the owned child process tree, if any. It is a no-op for
// adopted servers and safe to call multiple times.
func (s *Supervisor) Stop() error {
	s.mu.Lock()
	cmd := s.cmd
	if cmd == nil || cmd.Process == nil {
		s.mu.Unlock()
		return nil
	}
	// Detach the cmd under the same lock that read it, so a concurrent Stop or
	// Start cannot act on the same process.
	s.cmd = nil
	s.mu.Unlock()

	if err := killTree(cmd); err != nil {
		// Fall back to a plain kill: best-effort cleanup.
		_ = cmd.Process.Kill()
		return fmt.Errorf("engine: stop: %w", err)
	}
	// Reap synchronously so the process does not linger as a zombie.
	_, _ = cmd.Process.Wait()
	return nil
}
