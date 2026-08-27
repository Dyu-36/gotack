package main

import (
	"time"

	"github.com/Dyu-36/gotack/internal/engine"
	"github.com/Dyu-36/gotack/internal/uievents"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// events.go -- role: the single place where the host emits events to the UI.
//
// Engine SSE -> internal/uievents.Forwarder -> runtime.EventsEmit(name, payload)
// The UI subscribes only through frontend/src/platform/desktop.ts.
// Event names are declared once in internal/uievents/names.go.
//
// Rule: no polling loops here. Every UI update originates from an engine event.

const permissionTTL = 5 * time.Minute

// emit is the Emitter handed to the forwarder and terminal service. It stays a
// branch-free wrapper on purpose: every session:delta and every terminal:data
// chunk passes through here, so this is the wrong place for feature logic.
// Permission pairing now lives in the forwarder, the only code that decodes the
// typed request; see uievents.PermissionSink.
func (a *App) emit(name string, data any) {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, name, data)
}

func (a *App) setStatus(s engine.Status) {
	a.mu.Lock()
	a.status = s
	info := a.engineInfoLocked()
	a.mu.Unlock()
	a.emit(uievents.EngineStatus, info)
}
