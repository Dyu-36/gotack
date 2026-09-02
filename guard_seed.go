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

const (
	guardHookName = "gotack-guard"

	guardHookKey = "hooks.PreToolUse"

	guardHookEvent = "PreToolUse"

	guardHookTimeout = 10
)

func guardBinaryName() string {
	if runtime.GOOS == "windows" {
		return "guard.exe"
	}
	return "guard"
}

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

func (a *App) registerGuardHook(workspaceID string) {
	svc, err := a.services()
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Second)
	defer cancel()

	cfg, err := svc.api.GetWorkspaceConfig(ctx, workspaceID)
	if err != nil {

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
		Matcher: "",
		Command: command,
		Timeout: guardHookTimeout,
	})
	if err := svc.api.SetConfigField(ctx, workspaceID, crushapi.ConfigScopeWorkspace, guardHookKey, merged); err != nil && a.log != nil {
		a.log.Warn("guard hook registration failed", "err", err)
	}
}
