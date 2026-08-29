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

// office_setup.go -- role: register the bundled Office MCP server into the
// Crush workspace config so the agent gains Word, Excel and PowerPoint tools
// without any user setup.

// officeMCPName is the Crush config key of the bundled office server.
const officeMCPName = "gotack-office"

// resolveOfficeCommand finds the office MCP binary next to the desktop
// executable or on PATH. An empty result means the integration is not
// installed, which is normal in source builds.
func resolveOfficeCommand() string {
	name := "office"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
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

// registerOfficeTools syncs the mcp_servers entry of the open workspace with
// the installed office binary. The call is best effort: a failed registration
// degrades to no office tools, so it is logged, never surfaced as an error.
func (a *App) registerOfficeTools(workspaceID string) {
	svc, err := a.services()
	if err != nil {
		return
	}
	key := "mcp_servers." + officeMCPName
	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Second)
	defer cancel()

	command := resolveOfficeCommand()
	if command == "" {
		if err := svc.api.RemoveConfigField(ctx, workspaceID, crushapi.ConfigScopeWorkspace, key); err != nil && a.log != nil {
			a.log.Debug("office tools unregister skipped", "err", err)
		}
		return
	}
	entry := map[string]any{"command": command, "type": "stdio", "timeout": 30}
	if err := svc.api.SetConfigField(ctx, workspaceID, crushapi.ConfigScopeWorkspace, key, entry); err != nil {
		if a.log != nil {
			a.log.Warn("office tools registration failed", "err", err)
		}
		return
	}
	if a.log != nil {
		a.log.Info("office tools registered", "command", command)
	}
}
