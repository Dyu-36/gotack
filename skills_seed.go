package main

// skills_seed.go -- role: register the per-user skill mutation MCP server
// against the same directory Crush indexes. Project skills remain readable
// through Crush but are never passed to this writer.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/Dyu-36/gotack/internal/crushapi"
)

const skillsMCPName = "gotack-skills"

var resolveSkillsCommand = resolveSkillsCommandFromDisk

func skillsBinaryName() string {
	if runtime.GOOS == "windows" {
		return "skills.exe"
	}
	return "skills"
}

func resolveSkillsCommandFromDisk() string {
	name := skillsBinaryName()
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

func skillsEntry(command, root string) map[string]any {
	return map[string]any{
		"command": command,
		"args":    []string{"--root", root},
		"type":    "stdio",
		"timeout": 30,
	}
}

func (a *App) registerSkillsTools(workspaceID string) {
	svc, err := a.services()
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Second)
	defer cancel()

	command := resolveSkillsCommand()
	if command == "" {
		if err := svc.api.RemoveConfigField(ctx, workspaceID, crushapi.ConfigScopeWorkspace, "mcp_servers."+skillsMCPName); err != nil && a.log != nil {
			a.log.Warn("skills registration removal failed", "err", err)
		}
		return
	}
	if err := svc.api.SetConfigField(ctx, workspaceID, crushapi.ConfigScopeWorkspace, "mcp_servers."+skillsMCPName, skillsEntry(command, userSkillsDir())); err != nil && a.log != nil {
		a.log.Warn("skills registration failed", "err", err)
	}
}
