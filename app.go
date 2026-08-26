package main

import "context"

type App struct {
	ctx context.Context
}

func NewApp() *App { return &App{} }

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// BackendReady is a minimal bridge probe. Crush/session/streaming APIs will be
// implemented behind this Wails boundary while the Svelte UI stays unchanged.
func (a *App) BackendReady() bool { return true }
