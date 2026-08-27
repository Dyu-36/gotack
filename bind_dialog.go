package main

import "github.com/wailsapp/wails/v2/pkg/runtime"

// SelectWorkspace opens the native directory picker and returns the selected
// project root. An empty string means the user cancelled the dialog.
func (a *App) SelectWorkspace() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Chon thu muc workspace",
	})
}
