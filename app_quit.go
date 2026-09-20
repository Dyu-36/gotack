package main

import (
	"context"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"runtime"
)

func shouldHideOnClose(platform string, quitting bool) bool {
	return platform == "windows" && !quitting
}

func (a *App) beforeClose(ctx context.Context) bool {
	if !shouldHideOnClose(runtime.GOOS, a.quitting.Load()) {
		return false
	}
	wailsruntime.WindowHide(ctx)
	return true
}

func (a *App) quit() {
	a.quitting.Store(true)
	if a.ctx != nil {
		wailsruntime.Quit(a.ctx)
	}
}
