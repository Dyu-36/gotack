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
	"strings"
	"sync"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/engineapi"
	"github.com/Dyu-36/gotack/internal/runmetrics"
)

type Supervisor struct {
	log           *slog.Logger
	binary        string
	autoBinary    bool
	fixedEndpoint engineapi.Endpoint

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
	autoBinary := binary == ""
	if autoBinary {
		binary = defaultBinary()
	}
	return &Supervisor{log: log, binary: binary, autoBinary: autoBinary}
}

// NewSupervisorWithEndpoint creates an isolated sidecar on an explicit IPC
// endpoint, allowing contract tests to run alongside the desktop's engine.
func NewSupervisorWithEndpoint(log *slog.Logger, binary string, ep engineapi.Endpoint) *Supervisor {
	supervisor := NewSupervisor(log, binary)
	supervisor.fixedEndpoint = ep
	return supervisor
}

func (s *Supervisor) ipcEndpoint() engineapi.Endpoint {
	if s.fixedEndpoint.Network != "" && s.fixedEndpoint.Address != "" {
		return s.fixedEndpoint
	}
	return appconfig.PipeEndpoint()
}

func defaultBinary() string {
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}

	primary := "tack-engine" + ext

	for _, envKey := range []string{"GOTACK_ENGINE", "GOTACK_TEST_ENGINE"} {
		if val := strings.TrimSpace(os.Getenv(envKey)); val != "" {
			if info, err := os.Stat(val); err == nil && !info.IsDir() {
				if abs, err := filepath.Abs(val); err == nil {
					return abs
				}
				return val
			}
		}
	}

	candidates := make([]string, 0, 8)
	if executable, err := os.Executable(); err == nil {
		root := filepath.Dir(executable)
		candidates = append(candidates,
			filepath.Join(root, "resources", primary),
			filepath.Join(root, primary),
		)
	}

	candidates = append(candidates,
		filepath.Join("build", "bin", "resources", primary),
		filepath.Join("resources", primary),
		primary,
	)

	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			if abs, err := filepath.Abs(candidate); err == nil {
				return abs
			}
			return candidate
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
	ep := s.ipcEndpoint()
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
	// Build hooks or an installation repair may provide the sidecar after the
	// supervisor was created. Resolve again on every launch and reconnect.
	if s.autoBinary {
		bin = defaultBinary()
	}

	if !filepath.IsAbs(bin) && strings.ContainsAny(bin, `/\`) {
		absolute, err := filepath.Abs(bin)
		if err != nil {
			return engineapi.Endpoint{}, fmt.Errorf("engine: resolve binary path: %w", err)
		}
		bin = absolute
	}

	ep := s.ipcEndpoint()
	cmd := exec.Command(bin, "server", "--host", ep.Network+"://"+ep.Address)
	engineDir := filepath.Join(appconfig.Dir(), "engine")
	if err := os.MkdirAll(engineDir, 0o700); err != nil {
		return engineapi.Endpoint{}, fmt.Errorf("engine: prepare isolated configuration: %w", err)
	}
	cmd.Env = isolatedEngineEnvironment(engineDir)
	if executable, err := os.Executable(); err == nil {
		cmd.Env = bundledPythonEnvironment(cmd.Env, executable)
	}
	cmd.Dir = engineDir
	if keyPath, keyErr := runmetrics.EnsureKey(appconfig.Dir()); keyErr != nil {
		s.log.Warn("engine: cannot prepare telemetry key", "err", keyErr)
	} else {
		cmd.Env = append(cmd.Env, "TACK_RUN_METRICS_KEY_FILE="+keyPath)
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
		if errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
			return engineapi.Endpoint{}, fmt.Errorf("engine: executable %s is missing; "+
				"restore the bundled engine in resources beside Gotack, "+
				"or run scripts/build-engine.ps1 for development: %w", bin, err)
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

func isolatedEngineEnvironment(root string) []string {
	overrides := map[string]string{
		"TACK_GLOBAL_CONFIG":                 filepath.Join(root, "prompt-config"),
		"TACK_ENGINE_GLOBAL_CONFIG":              filepath.Join(root, "config"),
		"TACK_ENGINE_GLOBAL_DATA":                filepath.Join(root, "data"),
		"TACK_ENGINE_CACHE_DIR":                  filepath.Join(root, "cache"),
		"TACK_ENGINE_SKILLS_DIR":                 filepath.Join(appconfig.Dir(), "skills"),
		"TACK_DISABLE_PROVIDER_AUTO_UPDATE":      "1",
	}
	env := make([]string, 0, len(os.Environ())+len(overrides))
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if _, replaced := overrides[strings.ToUpper(key)]; !replaced && !strings.EqualFold(key, "TACK_RUN_METRICS_KEY_FILE") {
			env = append(env, entry)
		}
	}
	for _, key := range []string{"TACK_GLOBAL_CONFIG", "TACK_ENGINE_GLOBAL_CONFIG", "TACK_ENGINE_GLOBAL_DATA", "TACK_ENGINE_CACHE_DIR", "TACK_ENGINE_SKILLS_DIR", "TACK_DISABLE_PROVIDER_AUTO_UPDATE"} {
		env = append(env, key+"="+overrides[key])
	}
	return env
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
