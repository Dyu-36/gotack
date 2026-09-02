package main

import (
	"github.com/Dyu-36/gotack/internal/userstrings"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) BackendReady() bool { return true }

func (a *App) SelectWorkspace() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: userstrings.PickWorkspaceTitle,
	})
}
