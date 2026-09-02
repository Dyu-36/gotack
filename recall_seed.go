package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/crushapi"
)

const recallMCPName = "gotack-recall"

var resolveRecallCommand = resolveRecallCommandFromDisk

func recallBinaryName() string {
	if runtime.GOOS == "windows" {
		return "recall.exe"
	}
	return "recall"
}

func resolveRecallCommandFromDisk() string {
	name := recallBinaryName()
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

func recallEntry(command, dataDir, indexDir string) map[string]any {
	return map[string]any{
		"command": command,
		"args":    []string{"--data-dir", dataDir, "--index-dir", indexDir},
		"type":    "stdio",
		"timeout": 30,
	}
}

func (a *App) registerRecallTools(workspaceID string) {
	svc, err := a.services()
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Second)
	defer cancel()

	command := resolveRecallCommand()
	desc, current := svc.ws.Current()
	if command == "" || !current || desc.WorkspaceID != workspaceID || desc.DataDir == "" {
		if err := svc.api.RemoveConfigField(ctx, workspaceID, crushapi.ConfigScopeWorkspace, "mcp_servers."+recallMCPName); err != nil && a.log != nil {
			a.log.Warn("recall registration removal failed", "err", err)
		}
		return
	}
	indexDir := filepath.Join(appconfig.Dir(), "recall", workspaceID)
	entry := recallEntry(command, desc.DataDir, indexDir)
	if err := svc.api.SetConfigField(ctx, workspaceID, crushapi.ConfigScopeWorkspace, "mcp_servers."+recallMCPName, entry); err != nil && a.log != nil {
		a.log.Warn("recall registration failed", "err", err)
	}
}
