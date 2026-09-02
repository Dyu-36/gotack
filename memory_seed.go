package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/Dyu-36/gotack/internal/crushapi"
)

const memoryMCPName = "gotack-memory"

var resolveMemoryCommand = resolveMemoryCommandFromDisk

func memoryBinaryName() string {
	if runtime.GOOS == "windows" {
		return "memory.exe"
	}
	return "memory"
}

func resolveMemoryCommandFromDisk() string {
	name := memoryBinaryName()
	if executable, err := os.Executable(); err == nil {
		root := filepath.Dir(executable)
		for _, candidate := range []string{
			filepath.Join(root, "resources", name),
			filepath.Join(root, name),
		} {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate
			}
		}
	}
	if found, err := exec.LookPath(name); err == nil {
		return found
	}
	return ""
}

func memoryEntry(command string) map[string]any {
	return map[string]any{"command": command, "type": "stdio", "timeout": 30}
}

func (a *App) registerMemoryTools(workspaceID string) {
	svc, err := a.services()
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Second)
	defer cancel()

	command := resolveMemoryCommand()
	if command == "" {
		if err := svc.api.RemoveConfigField(ctx, workspaceID, crushapi.ConfigScopeWorkspace, "mcp_servers."+memoryMCPName); err != nil && a.log != nil {
			a.log.Warn("memory registration removal failed", "err", err)
		}
		return
	}
	if err := svc.api.SetConfigField(ctx, workspaceID, crushapi.ConfigScopeWorkspace, "mcp_servers."+memoryMCPName, memoryEntry(command)); err != nil && a.log != nil {
		a.log.Warn("memory registration failed", "err", err)
	}
}
