package main

import "github.com/Dyu-36/gotack/internal/uievents"

func (a *App) runDone(done uievents.SessionDonePayload) {
	if a.zalo != nil {
		a.zalo.Done(done.SessionID, done.Text)
	}
	scheduled := false
	if a.scheduler != nil {
		scheduled = a.scheduler.RecordOutcome(done.SessionID, done.Error, done.Cancelled)
	}
	if a.reflection == nil {
		return
	}
	if scheduled {
		a.reflection.Forget(done.SessionID)
		return
	}
	review, cleanupID := a.reflection.RunDone(done.SessionID, done.Text, done.Error, done.Cancelled)
	if cleanupID != "" {
		a.cleanupReflection(cleanupID)
	}
	if review.Any() {
		a.triggerReflection(done.SessionID, review)
	}
}
