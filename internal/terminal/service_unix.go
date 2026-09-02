//go:build !windows

package terminal

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/creack/pty"
)

type unixBackend struct {
	master *os.File
	cmd    *exec.Cmd
	done   chan struct{}
	code   int32
	err    error
}

func (b *unixBackend) Read(p []byte) (int, error)  { return b.master.Read(p) }
func (b *unixBackend) Write(p []byte) (int, error) { return b.master.Write(p) }

func (b *unixBackend) Resize(cols, rows uint16) error {
	return pty.Setsize(b.master, &pty.Winsize{
		Rows: rows,
		Cols: cols,
	})
}

func (b *unixBackend) Close() error {
	if b.cmd != nil && b.cmd.Process != nil {

		_ = b.cmd.Process.Signal(syscall.SIGTERM)
	}
	if b.master != nil {
		err := b.master.Close()
		b.master = nil
		return err
	}
	return nil
}

func (b *unixBackend) Wait() (int32, error) {
	<-b.done
	return b.code, b.err
}

func init() {
	openBackend = openUnixBackend
}

func openUnixBackend(cwd string) (ptyBackend, shellSpec, error) {
	cleaned, err := validateCwd(cwd)
	if err != nil {
		return nil, shellSpec{}, err
	}

	shell, args := pickShell()
	cmd := exec.Command(shell, args...)
	cmd.Dir = cleaned

	cmd.Env = withTERM(os.Environ())

	ws := &pty.Winsize{Rows: 24, Cols: 80}
	master, err := pty.StartWithSize(cmd, ws)
	if err != nil {
		return nil, shellSpec{}, fmt.Errorf("terminal: start pty: %w", err)
	}

	be := &unixBackend{
		master: master,
		cmd:    cmd,
		done:   make(chan struct{}),
	}
	go be.reap()
	return be, shellSpec{
		commandLine: strings.Join(append([]string{shell}, args...), " "),
		workDir:     cleaned,
	}, nil
}

func (b *unixBackend) reap() {
	err := b.cmd.Wait()
	if err != nil {
		b.code = -1
	} else {
		b.code = 0
	}
	b.err = err
	close(b.done)
}

func pickShell() (string, []string) {
	if sh := strings.TrimSpace(os.Getenv("SHELL")); sh != "" {
		if _, err := os.Stat(sh); err == nil {
			return sh, []string{"-i"}
		}
	}
	if _, err := os.Stat("/bin/bash"); err == nil {
		return "/bin/bash", []string{"-i"}
	}
	return "/bin/sh", []string{"-i"}
}

func withTERM(env []string) []string {
	const want = "TERM=xterm-256color"
	out := make([]string, 0, len(env))
	replaced := false
	for _, kv := range env {
		if strings.HasPrefix(kv, "TERM=") {
			out = append(out, want)
			replaced = true
			continue
		}
		out = append(out, kv)
	}
	if !replaced {
		out = append(out, want)
	}
	return out
}

func validateCwd(cwd string) (string, error) {
	trimmed := strings.TrimSpace(cwd)
	if trimmed == "" {
		return "", errors.New("terminal: empty working directory")
	}
	abs, err := filepath.Abs(trimmed)
	if err != nil {
		return "", fmt.Errorf("terminal: resolve cwd: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("terminal: cwd not found: %s", abs)
		}
		return "", fmt.Errorf("terminal: cwd inaccessible: %s: %w", abs, err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("terminal: stat cwd: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("terminal: cwd is not a directory: %s", resolved)
	}
	return filepath.Clean(resolved), nil
}
