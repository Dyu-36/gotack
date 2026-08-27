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

// unixBackend wraps the master end of a pty(7) pair so it satisfies
// ptyBackend. The child process is reaped by the reap goroutine and its
// exit code is delivered through Wait via the done channel.
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

// Close terminates the child if it is still alive and closes the master fd.
// It is safe to call multiple times; the first call wins, later calls return
// nil. We do not return the error from the actual close because the child
// may have already exited on its own.
func (b *unixBackend) Close() error {
	if b.cmd != nil && b.cmd.Process != nil {
		// Best-effort SIGTERM; the reaper will reap. If the child is already
		// gone the signal is harmlessly dropped by the kernel.
		_ = b.cmd.Process.Signal(syscall.SIGTERM)
	}
	if b.master != nil {
		err := b.master.Close()
		b.master = nil
		return err
	}
	return nil
}

// Wait blocks until the child exits and returns its exit code. A successful
// normal exit is 0; an exit by signal is reported as -1 because the Go
// high-level API does not surface the signal number.
func (b *unixBackend) Wait() (int32, error) {
	<-b.done
	return b.code, b.err
}

// init wires the real pty(7) backend. Tests override openBackend directly.
func init() {
	openBackend = openUnixBackend
}

// openUnixBackend validates cwd, picks $SHELL (or /bin/bash) and starts it
// attached to a fresh pty. The initial window is 80x24 which the UI can
// resize as soon as the first TerminalData chunk arrives.
func openUnixBackend(cwd string) (ptyBackend, shellSpec, error) {
	cleaned, err := validateCwd(cwd)
	if err != nil {
		return nil, shellSpec{}, err
	}

	shell, args := pickShell()
	cmd := exec.Command(shell, args...)
	cmd.Dir = cleaned
	// Inherit the parent environment but force TERM so shells that gate
	// colour output on it do not see "dumb" or nothing.
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

// reap waits for the child to exit and stashes the result on the backend so
// the public Wait call can return immediately.
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

// pickShell returns the shell binary and its default args. The $SHELL
// environment variable is preferred because that is the user-chosen default;
// on a fresh container or minimal CI image that may be empty or point to
// something that does not exist, so we fall back to /bin/bash which is
// effectively universal on the Unix-likes we care about (linux, darwin,
// *bsd). The last-ditch fallback is /bin/sh, which POSIX mandates.
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

// withTERM returns env with TERM forced to xterm-256color. Shells that
// detect no TERM fall back to monochrome output.
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

// validateCwd enforces the rules from the bridge spec: the path must be an
// existing, accessible directory. Symlinks are followed and the result is
// cleaned so the child process receives a stable path.
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
