package main

import (
	"github.com/Dyu-36/gotack/internal/autostart"
	"github.com/Dyu-36/gotack/internal/userstrings"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) BackendReady() bool { return true }

// GetAutoStart reports whether Tack is registered to launch at login.
func (a *App) GetAutoStart() bool { return autostart.Enabled() }

// SetAutoStart registers or removes the launch-at-login entry, which always
// starts hidden in the tray.
func (a *App) SetAutoStart(enabled bool) error { return autostart.SetEnabled(enabled) }

func (a *App) SelectWorkspace() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: userstrings.PickWorkspaceTitle,
	})
}
