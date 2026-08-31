package main

import "github.com/wailsapp/wails/v2/pkg/runtime"

// bind_host.go -- role: Wails-bound host-level methods: the readiness probe and
// native OS dialogs.
//
// Every exported method on App becomes a UI API: window.go.main.App.<Method>.
// Keep arguments and results JSON-serializable; never leak context, channels
// or engine types across this boundary.
//
// This file merges the former bind_bridge.go (10 lines) and bind_dialog.go
// (11 lines). Both carried a single method that talks to the host process
// rather than to a Crush subsystem, and bind_dialog.go was the only bind_*.go
// file missing the role header this repo requires.

// BackendReady reports whether the host can serve UI calls yet.
func (a *App) BackendReady() bool { return true }

// SelectWorkspace opens the native directory picker and returns the selected
// project root. An empty string means the user cancelled the dialog.
func (a *App) SelectWorkspace() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Chọn thư mục làm việc",
	})
}
