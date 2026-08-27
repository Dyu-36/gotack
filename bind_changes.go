package main

import "github.com/Dyu-36/gotack/internal/changes"

// bind_changes.go -- role: Wails-bound API for changed files and diffs.
//
// Full editor features stay out of scope, see docs/roadmap.md.
//
// There is deliberately no bind-layer DTO here. changes.FileStatus is already
// exactly the wire shape the UI declares in frontend/src/platform/desktop.ts
// (path, size, updated_at), so a local struct would be an identity type plus a
// copy loop. Contrast bind_session.go, where SessionInfo earns its keep by
// dropping engine fields and flattening Parts.

// maxDiffBytes caps diff payloads sent to the UI.
const maxDiffBytes = 256 * 1024

// ChangedFiles lists files the agent touched in this session, latest version
// per path. Live change notifications arrive as changes:updated events; this
// call is the on-demand fetch for the viewer.
func (a *App) ChangedFiles(sessionID string) ([]changes.FileStatus, error) {
	svc, err := a.services()
	if err != nil {
		return nil, err
	}
	return svc.diffs.ChangedFiles(a.ctx, sessionID)
}

// FileDiff returns a lightweight unified diff between the previous and the
// latest stored version of the file in this session.
func (a *App) FileDiff(sessionID, path string) (string, error) {
	svc, err := a.services()
	if err != nil {
		return "", err
	}
	return svc.diffs.Diff(a.ctx, sessionID, path, maxDiffBytes)
}
