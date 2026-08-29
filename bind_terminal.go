package main

import (
	"errors"

	"github.com/Dyu-36/gotack/internal/terminal"
)

// bind_terminal.go -- role: Wails-bound API for the optional terminal.
//
// Nothing here may run at startup: the terminal is lazy by design.
// Output flows to the UI as terminal:data / terminal:exit events.

var (
	errTerminalUnavailable = errors.New("terminal service unavailable")
	errInvalidSize         = errors.New("invalid terminal size")
)

// termService returns the lazily wired terminal service. The terminal pointer
// is built once in startup and stored in conn; all four bound calls below
// share the same nil check.
func (a *App) termService() (*terminal.Service, error) {
	c := a.getConn()
	if c == nil || c.term == nil {
		return nil, errTerminalUnavailable
	}
	return c.term, nil
}

// OpenTerminal creates a PTY session rooted at cwd and returns its ID.
func (a *App) OpenTerminal(cwd string) (string, error) {
	term, err := a.termService()
	if err != nil {
		return "", err
	}
	return term.Open(cwd)
}

// WriteTerminal forwards user keystrokes to the PTY.
func (a *App) WriteTerminal(id, data string) error {
	term, err := a.termService()
	if err != nil {
		return err
	}
	return term.Write(id, data)
}

// ResizeTerminal propagates the viewport size. The bounds check runs first:
// it depends only on the arguments, so an invalid size is rejected the same
// way whether or not the terminal happens to be wired.
func (a *App) ResizeTerminal(id string, cols, rows int) error {
	if cols <= 0 || rows <= 0 || cols > 1000 || rows > 1000 {
		return errInvalidSize
	}
	term, err := a.termService()
	if err != nil {
		return err
	}
	return term.Resize(id, uint16(cols), uint16(rows))
}

// CloseTerminal releases the PTY.
func (a *App) CloseTerminal(id string) error {
	term, err := a.termService()
	if err != nil {
		return err
	}
	return term.Close(id)
}
