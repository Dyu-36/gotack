package workspaceconfig

import (
	"context"
	"fmt"

	"github.com/Dyu-36/gotack/internal/crushapi"
)

const (
	GuardHookName    = "gotack-guard"
	GuardHookKey     = "hooks.PreToolUse"
	GuardHookEvent   = "PreToolUse"
	GuardHookTimeout = 10
)

func FilterGuardHook(hooks []crushapi.HookEntry) []crushapi.HookEntry {
	out := make([]crushapi.HookEntry, 0, len(hooks))
	for _, hook := range hooks {
		if hook.Name == GuardHookName {
			continue
		}
		out = append(out, hook)
	}
	return out
}

func RegisterGuard(base context.Context, api *crushapi.Client, workspaceID, command string) error {
	ctx, cancel := registrationContext(base)
	defer cancel()

	cfg, err := api.GetWorkspaceConfig(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("guard hook: read current hooks: %w", err)
	}
	existing := FilterGuardHook(cfg.Hooks[GuardHookEvent])
	if command == "" {
		if len(existing) == 0 {
			if err := api.RemoveConfigField(ctx, workspaceID, crushapi.ConfigScopeWorkspace, GuardHookKey); err != nil {
				return fmt.Errorf("guard hook removal: %w", err)
			}
			return nil
		}
		if err := api.SetConfigField(ctx, workspaceID, crushapi.ConfigScopeWorkspace, GuardHookKey, existing); err != nil {
			return fmt.Errorf("guard hook removal rewrite: %w", err)
		}
		return nil
	}

	merged := append(existing, crushapi.HookEntry{
		Name:    GuardHookName,
		Matcher: "",
		Command: command,
		Timeout: GuardHookTimeout,
	})
	if err := api.SetConfigField(ctx, workspaceID, crushapi.ConfigScopeWorkspace, GuardHookKey, merged); err != nil {
		return fmt.Errorf("guard hook registration: %w", err)
	}
	return nil
}
