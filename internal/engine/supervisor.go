package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/engineapi"
	"github.com/Dyu-36/gotack/internal/runmetrics"
)

type Supervisor struct {
	log    *slog.Logger
	binary string

	lifecycle sync.Mutex
	mu        sync.Mutex
	cmd       *exec.Cmd
	owned     bool
	endpoint  engineapi.Endpoint
	done      chan struct{}
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

func defaultBinary() string {
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}

	primary := "tack-engine" + ext

	if executable, err := os.Executable(); err == nil {
		root := filepath.Dir(executable)
		for _, candidate := range []string{
			filepath.Join(root, "resources", primary),
			filepath.Join(root, primary),
		} {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate
			}
		}
	}

	if found, err := exec.LookPath(primary); err == nil {
		return found
	}
	if primary != "tack-engine" {
		if found, err := exec.LookPath("tack-engine"); err == nil {
			return found
		}
	}

	return primary
}

func (s *Supervisor) Locate(ctx context.Context) (engineapi.Endpoint, bool) {
	ep := appconfig.PipeEndpoint()
	if err := engineapi.Probe(ctx, ep); err != nil {
		s.log.Debug("engine: probe failed", "endpoint", ep, "err", err)
		return engineapi.Endpoint{}, false
	}

	s.mu.Lock()
	s.endpoint = ep
	s.mu.Unlock()
	return ep, true
}

func (s *Supervisor) Owned() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.owned
}

func (s *Supervisor) Start() (engineapi.Endpoint, error) {
	s.lifecycle.Lock()
	defer s.lifecycle.Unlock()

	s.mu.Lock()
	if s.cmd != nil && s.cmd.Process != nil {
		running := s.endpoint
		s.mu.Unlock()
		return running, nil
	}
	bin := s.binary
	s.mu.Unlock()

	ep := appconfig.PipeEndpoint()
	cmd := exec.Command(bin, "server", "--host", ep.Network+"://"+ep.Address)
	if keyPath, keyErr := runmetrics.EnsureKey(appconfig.Dir()); keyErr != nil {
		s.log.Warn("engine: cannot prepare telemetry key", "err", keyErr)
	} else {
		cmd.Env = append(os.Environ(), "TACK_RUN_METRICS_KEY_FILE="+keyPath)
	}
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
		return engineapi.Endpoint{}, fmt.Errorf("engine: start %s: %w", bin, err)
	}

	done := make(chan struct{})
	s.mu.Lock()
	s.cmd = cmd
	s.owned = true
	s.endpoint = ep
	s.done = done
	s.mu.Unlock()

	s.log.Info("engine: started", "binary", bin, "pid", cmd.Process.Pid, "endpoint", ep)
	go s.wait(cmd, logFile, done)
	return ep, nil
}

func (s *Supervisor) wait(cmd *exec.Cmd, logFile *os.File, done chan struct{}) {
	err := cmd.Wait()
	if logFile != nil {
		_ = logFile.Close()
	}
	s.mu.Lock()
	if s.cmd == cmd {
		s.cmd = nil
		s.owned = false
		s.done = nil
	}
	s.mu.Unlock()
	s.log.Debug("engine: exited", "pid", cmd.Process.Pid, "err", err)
	close(done)
}

func (s *Supervisor) Stop() error {
	s.lifecycle.Lock()
	defer s.lifecycle.Unlock()

	s.mu.Lock()
	cmd := s.cmd
	if cmd == nil || cmd.Process == nil || !s.owned {
		s.mu.Unlock()
		return nil
	}
	done := s.done
	s.mu.Unlock()

	if err := killTree(cmd); err != nil && !errors.Is(err, os.ErrProcessDone) {
		if killErr := cmd.Process.Kill(); killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
			return fmt.Errorf("engine: stop: %w", errors.Join(err, killErr))
		}
	}
	<-done
	return nil
}
