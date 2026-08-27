//go:build windows

package terminal

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/UserExistsError/conpty"
)

// windowsBackend wraps a conpty.ConPty so it satisfies ptyBackend.
type windowsBackend struct {
	cpty *conpty.ConPty
}

func (b *windowsBackend) Read(p []byte) (int, error)  { return b.cpty.Read(p) }
func (b *windowsBackend) Write(p []byte) (int, error) { return b.cpty.Write(p) }

func (b *windowsBackend) Resize(cols, rows uint16) error {
	return b.cpty.Resize(int(cols), int(rows))
}

func (b *windowsBackend) Close() error { return b.cpty.Close() }

// Wait blocks on the underlying conpty handle and returns the exit code as a
// signed int32. STILL_ACTIVE (259) is mapped to -1 so the UI can tell that
// the wait was interrupted rather than the process actually exiting with
// 259. In practice this branch only fires if the conpty handle is closed
// before the child exits.
func (b *windowsBackend) Wait() (int32, error) {
	const stillActive = 259
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	code, err := b.cpty.Wait(ctx)
	if err != nil {
		return -1, err
	}
	if code == stillActive {
		return -1, errors.New("terminal: conpty still active at wait")
	}
	return int32(code), nil
}

// init wires the real ConPTY backend. Tests override openBackend directly.
func init() {
	openBackend = openWindowsBackend
}

// openWindowsBackend validates cwd and starts powershell -NoLogo inside a
// ConPTY. The default 80x24 window is used; the UI can resize as soon as the
// first TerminalData chunk arrives.
func openWindowsBackend(cwd string) (ptyBackend, shellSpec, error) {
	cleaned, err := validateCwd(cwd)
	if err != nil {
		return nil, shellSpec{}, err
	}

	cpty, err := conpty.Start(
		"powershell -NoLogo",
		conpty.ConPtyWorkDir(cleaned),
	)
	if err != nil {
		return nil, shellSpec{}, fmt.Errorf("terminal: start powershell: %w", err)
	}

	return &windowsBackend{cpty: cpty}, shellSpec{
		commandLine: "powershell -NoLogo",
		workDir:     cleaned,
	}, nil
}

// validateCwd enforces the rules from the bridge spec: the path must be an
// existing, accessible directory. Symlinks are followed and the result is
// cleaned so the ConPTY receives a stable path the shell can actually chdir
// to.
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
		// Surface a precise message: file/dir missing vs permission denied.
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
