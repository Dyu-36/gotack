package main

import workspaceconfig "github.com/Dyu-36/gotack/internal/workspaceconfig"

var resolveMemoryCommand = resolveMemoryCommandFromDisk

func memoryBinaryName() string {
	return workspaceconfig.BinaryName("memory")
}

func resolveMemoryCommandFromDisk() string {
	return workspaceconfig.ResolveBinary(memoryBinaryName())
}

func memoryEntry(command string) map[string]any {
	return workspaceconfig.MemoryEntry(command)
}

func (a *App) registerMemoryTools(workspaceID string) {
	svc, err := a.services()
	if err != nil {
		return
	}
	if err := workspaceconfig.RegisterMemory(a.ctx, svc.api, workspaceID, resolveMemoryCommand()); err != nil && a.log != nil {
		a.log.Warn("memory registration failed", "err", err)
	}
}
