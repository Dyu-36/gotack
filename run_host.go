package main

import "github.com/Dyu-36/gotack/internal/uievents"

func (a *App) runDone(done uievents.SessionDonePayload) {
	if a.zalo != nil {
		a.zalo.Done(done.SessionID, done.Text)
	}
}
