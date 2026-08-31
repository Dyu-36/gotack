package main

// memory_seed.go -- role: register (or remove) the gotack memory MCP server
// in the Crush workspace configuration.
//
// The memory binary is the only sanctioned writer of the memory files
// (decision 0003), so its lifecycle mirrors the bundled office and guard
// binaries: resolve the executable, and when it is absent remove the config
// key instead of leaving a dangling mcp_servers entry that errors on every
// tool call.

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

// resolveMemoryCommand is a variable rather than a plain function so the
// registration tests can exercise both the binary-present and the
// binary-absent paths without fabricating executables on disk.
var resolveMemoryCommand = resolveMemoryCommandFromDisk

// memoryBinaryName is the platform-correct memory executable name.
func memoryBinaryName() string {
	if runtime.GOOS == "windows" {
		return "memory.exe"
	}
	return "memory"
}

// resolveMemoryCommandFromDisk prefers the bundled memory binary shipped
// next to the desktop executable, falling back to a system PATH install. An
// empty result tells the host to remove the registration so the agent is
// never shown a half-configured tool (hard rule 8).
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

// memoryEntry is the exact config value registered under
// mcp_servers.gotack-memory; the shape matches gotack-office so the engine
// launches it the same way.
func memoryEntry(command string) map[string]any {
	return map[string]any{"command": command, "type": "stdio", "timeout": 30}
}

// registerMemoryTools wires the memory MCP server into the active workspace
// configuration at workspace activation scope. When the binary cannot be
// resolved the key is removed, so a missing binary never leaves a dangling
// mcp_servers.gotack-memory entry behind.
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
