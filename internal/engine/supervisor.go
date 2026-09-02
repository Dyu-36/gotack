package engine

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/crushapi"
)

type Supervisor struct {
	log    *slog.Logger
	binary string

	mu       sync.Mutex
	cmd      *exec.Cmd
	owned    bool
	endpoint crushapi.Endpoint
	logFile  *os.File
}

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

	logPath := filepath.Join(appconfig.LogDir(), "tack-engine.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		s.log.Warn("engine: cannot open engine log, discarding output", "path", logPath, "err", err)
	} else {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
	}
	cmd.Stdin = nil

	if err := cmd.Start(); err != nil {
		if logFile != nil {
			_ = logFile.Close()
		}
		return crushapi.Endpoint{}, fmt.Errorf("engine: start %s: %w", bin, err)
	}

	s.mu.Lock()
	s.cmd = cmd
	s.owned = true
	s.endpoint = ep
	s.logFile = logFile
	s.mu.Unlock()

	s.log.Info("engine: started", "binary", bin, "pid", cmd.Process.Pid, "endpoint", ep)
	return ep, nil
}

func (s *Supervisor) Stop() error {
	s.mu.Lock()
	cmd := s.cmd
	if cmd == nil || cmd.Process == nil || !s.owned {
		s.mu.Unlock()
		return nil
	}
	logFile := s.logFile
	s.cmd = nil
	s.owned = false
	s.logFile = nil
	s.mu.Unlock()

	if logFile != nil {
		defer logFile.Close()
	}

	if err := killTree(cmd); err != nil {
		_ = cmd.Process.Kill()
		return fmt.Errorf("engine: stop: %w", err)
	}
	_, _ = cmd.Process.Wait()
	return nil
}

var _ EngineAPI = (*Supervisor)(nil)
