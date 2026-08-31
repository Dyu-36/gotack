package main

// guard_seed.go -- role: register (or remove) the Gotack approval hook in the
// Crush workspace configuration.
//
// Crush supports one hook event, PreToolUse. The guard binary is the only
// thing standing between the agent and unrecoverable commands, so its lifecycle
// mirrors the bundled office binary: resolve the executable, and when it is
// absent remove the config key instead of leaving a dangling hook that errors
// on every tool call.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/Dyu-36/gotack/internal/crushapi"
)

const (
	// guardHookName identifies the gotack entry inside hooks.PreToolUse so a
	// re-register replaces it and a removal never touches user-defined hooks.
	guardHookName = "gotack-guard"
	// guardHookKey is the Crush config path for the PreToolUse hook list.
	guardHookKey = "hooks.PreToolUse"
	// guardHookEvent is the only hook event Crush fires.
	guardHookEvent = "PreToolUse"
	// guardHookTimeout bounds a single hook run. The guard does no I/O beyond
	// stdin/stdout, so ten seconds is generous; a slow hook must not stall the
	// agent indefinitely (risk R5).
	guardHookTimeout = 10
)

// guardBinaryName is the platform-correct guard executable name.
func guardBinaryName() string {
	if runtime.GOOS == "windows" {
		return "guard.exe"
	}
	return "guard"
}

// resolveGuardCommand prefers the bundled guard binary shipped next to the
// desktop executable, falling back to a system PATH install. An empty result
// tells the host to remove the hook registration so the workspace is not left
// pointing at a binary that does not exist.
func resolveGuardCommand() string {
	name := guardBinaryName()
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

// filterGuardHook returns hooks with any gotack-guard entry removed, preserving
// user-defined hooks and their order.
func filterGuardHook(hooks []crushapi.HookEntry) []crushapi.HookEntry {
	out := make([]crushapi.HookEntry, 0, len(hooks))
	for _, h := range hooks {
		if h.Name == guardHookName {
			continue
		}
		out = append(out, h)
	}
	return out
}

// registerGuardHook wires the approval hook into the workspace config. It reads
// the current hooks.PreToolUse list and merges, so a user-defined hook on the
// same event is never clobbered. When the guard binary cannot be resolved the
// gotack entry is removed — leaving it would make every tool call error.
func (a *App) registerGuardHook(workspaceID string) {
	svc, err := a.services()
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Second)
	defer cancel()

	cfg, err := svc.api.GetWorkspaceConfig(ctx, workspaceID)
	if err != nil {
		// Without the current list we cannot merge safely; skip rather than
		// risk overwriting a user hook.
		if a.log != nil {
			a.log.Warn("guard hook: could not read current hooks, skipping registration", "err", err)
		}
		return
	}
	existing := filterGuardHook(cfg.Hooks[guardHookEvent])

	command := resolveGuardCommand()
	if command == "" {
		if len(existing) == 0 {
			if err := svc.api.RemoveConfigField(ctx, workspaceID, crushapi.ConfigScopeWorkspace, guardHookKey); err != nil && a.log != nil {
				a.log.Warn("guard hook removal failed", "err", err)
			}
		} else if err := svc.api.SetConfigField(ctx, workspaceID, crushapi.ConfigScopeWorkspace, guardHookKey, existing); err != nil && a.log != nil {
			a.log.Warn("guard hook removal (rewrite) failed", "err", err)
		}
		return
	}

	merged := append(existing, crushapi.HookEntry{
		Name:    guardHookName,
		Matcher: "", // match every tool
		Command: command,
		Timeout: guardHookTimeout,
	})
	if err := svc.api.SetConfigField(ctx, workspaceID, crushapi.ConfigScopeWorkspace, guardHookKey, merged); err != nil && a.log != nil {
		a.log.Warn("guard hook registration failed", "err", err)
	}
}
