package main

import "github.com/Dyu-36/gotack/internal/changes"

const maxDiffBytes = 256 * 1024

func (a *App) ChangedFiles(sessionID string) ([]changes.FileStatus, error) {
	svc, err := a.services()
	if err != nil {
		return nil, err
	}
	return svc.diffs.ChangedFiles(a.ctx, sessionID)
}

func (a *App) FileDiff(sessionID, path string) (string, error) {
	svc, err := a.services()
	if err != nil {
		return "", err
	}
	return svc.diffs.Diff(a.ctx, sessionID, path, maxDiffBytes)
}
