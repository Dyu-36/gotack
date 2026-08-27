package main

// bind_bridge.go -- role: Wails-bound probe and handshake methods.
//
// Every exported method on App becomes a UI API: window.go.main.App.<Method>.
// Keep arguments and results JSON-serializable; never leak context, channels
// or engine types across this boundary.

// BackendReady reports whether the host can serve UI calls yet.
func (a *App) BackendReady() bool { return true }
