package main

import (
	"path/filepath"

	"github.com/Dyu-36/gotack/internal/appconfig"
	workspaceconfig "github.com/Dyu-36/gotack/internal/workspaceconfig"
)

var resolveRecallCommand = resolveRecallCommandFromDisk

func recallBinaryName() string {
	return workspaceconfig.BinaryName("recall")
}

func resolveRecallCommandFromDisk() string {
	return workspaceconfig.ResolveBinary(recallBinaryName())
}

func recallEntry(command, dataDir, indexDir string) map[string]any {
	return workspaceconfig.RecallEntry(command, dataDir, indexDir)
}

func (a *App) registerRecallTools(workspaceID string) {
	svc, err := a.services()
	if err != nil {
		return
	}
	desc, _ := svc.ws.Current()
	if err := workspaceconfig.RegisterRecall(a.ctx, svc.api, workspaceID, desc, resolveRecallCommand(), filepath.Join(appconfig.Dir(), "recall")); err != nil && a.log != nil {
		a.log.Warn("recall registration failed", "err", err)
	}
}
