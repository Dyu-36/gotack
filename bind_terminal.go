package main

import (
	"errors"

	"github.com/Dyu-36/gotack/internal/terminal"
)

var (
	errTerminalUnavailable = errors.New("terminal service unavailable")
	errInvalidSize         = errors.New("invalid terminal size")
)

func (a *App) termService() (*terminal.Service, error) {
	c := a.getConn()
	if c == nil || c.term == nil {
		return nil, errTerminalUnavailable
	}
	return c.term, nil
}

func (a *App) OpenTerminal(cwd string) (string, error) {
	term, err := a.termService()
	if err != nil {
		return "", err
	}
	return term.Open(cwd)
}

func (a *App) WriteTerminal(id, data string) error {
	term, err := a.termService()
	if err != nil {
		return err
	}
	return term.Write(id, data)
}

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

func (a *App) CloseTerminal(id string) error {
	term, err := a.termService()
	if err != nil {
		return err
	}
	return term.Close(id)
}
