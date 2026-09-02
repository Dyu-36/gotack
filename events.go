package main

import "github.com/wailsapp/wails/v2/pkg/runtime"

func (a *App) emit(name string, data any) {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, name, data)
}
