package main

import (
	"github.com/Dyu-36/gotack/internal/uievents"
	"github.com/Dyu-36/gotack/internal/zalo"
)

func (a *App) runDone(done uievents.SessionDonePayload) {
	if a.zalo != nil {
		a.zalo.Done(zalo.Completion{RunID: done.RunID, SessionID: done.SessionID, Text: done.Text, Error: done.Error, Cancelled: done.Cancelled})
	}
}
