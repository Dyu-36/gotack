package workspaceconfig

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Dyu-36/gotack/internal/engineapi"
)

const (
	LegacyGuardHookName  = "gotack-guard"
	LegacyGuardHookEvent = "PreToolUse"
	LegacyGuardHookKey   = "hooks.PreToolUse"

	legacyMemoryMCP = "gotack-memory"
	legacySkillsMCP = "gotack-skills"
	legacyRecallMCP = "gotack-recall"
	legacyOfficeMCP = "gotack-office"
)

func FilterGuardHook(hooks []engineapi.HookEntry) []engineapi.HookEntry {
	out := make([]engineapi.HookEntry, 0, len(hooks))
	for _, hook := range hooks {
		if hook.Name == LegacyGuardHookName {
			continue
		}
		out = append(out, hook)
	}
	return out
}

func removeLegacyTools(base context.Context, api *engineapi.Client, workspaceID, managedRoot string) error {
	ctx, cancel := registrationContext(base)
	defer cancel()

	var removalErrs []error
	for _, name := range []string{legacyMemoryMCP, legacySkillsMCP, legacyRecallMCP, legacyOfficeMCP} {
		if err := api.RemoveConfigField(ctx, workspaceID, engineapi.ConfigScopeWorkspace, "mcp_servers."+name); err != nil {
			removalErrs = append(removalErrs, fmt.Errorf("remove mcp_servers.%s: %w", name, err))
		}
	}

	cfg, err := api.GetWorkspaceConfig(ctx, workspaceID)
	if err != nil {
		return errors.Join(errors.Join(removalErrs...), fmt.Errorf("legacy config read: %w", err))
	}

	hooks := FilterGuardHook(cfg.Hooks[LegacyGuardHookEvent])
	if len(hooks) != len(cfg.Hooks[LegacyGuardHookEvent]) {
		if len(hooks) == 0 {
			if err := api.RemoveConfigField(ctx, workspaceID, engineapi.ConfigScopeWorkspace, LegacyGuardHookKey); err != nil {
				return errors.Join(errors.Join(removalErrs...), fmt.Errorf("guard hook removal: %w", err))
			}
		} else if err := api.SetConfigField(ctx, workspaceID, engineapi.ConfigScopeWorkspace, LegacyGuardHookKey, hooks); err != nil {
			return errors.Join(errors.Join(removalErrs...), fmt.Errorf("guard hook cleanup: %w", err))
		}
	}

	if cfg.Options != nil {
		kept := make([]string, 0, len(cfg.Options.GlobalContextPaths))
		for _, path := range cfg.Options.GlobalContextPaths {
			if isManagedPath(path, managedRoot) {
				continue
			}
			kept = append(kept, path)
		}
		if len(kept) != len(cfg.Options.GlobalContextPaths) {
			if len(kept) == 0 {
				if err := api.RemoveConfigField(ctx, workspaceID, engineapi.ConfigScopeWorkspace, "options.global_context_paths"); err != nil {
					return errors.Join(errors.Join(removalErrs...), fmt.Errorf("managed context path removal: %w", err))
				}
			} else if err := api.SetConfigField(ctx, workspaceID, engineapi.ConfigScopeWorkspace, "options.global_context_paths", kept); err != nil {
				return errors.Join(errors.Join(removalErrs...), fmt.Errorf("managed context path cleanup: %w", err))
			}
		}
	}

	if env := legacyEnvWithoutOffice(cfg.Env, managedRoot); env != nil {
		if err := api.SetConfigField(ctx, workspaceID, engineapi.ConfigScopeWorkspace, "env", env); err != nil {
			return errors.Join(errors.Join(removalErrs...), fmt.Errorf("office env cleanup: %w", err))
		}
	}

	return errors.Join(removalErrs...)
}

func isManagedPath(path, managedRoot string) bool {
	if managedRoot == "" {
		return false
	}
	root := skillPathKey(filepath.Join(managedRoot, "context-prompt"))
	candidate := skillPathKey(path)
	if !filepath.IsAbs(root) || !filepath.IsAbs(candidate) {
		return false
	}
	rel, err := filepath.Rel(root, candidate)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func legacyEnvWithoutOffice(env map[string]string, managedRoot string) map[string]string {
	if len(env) == 0 || !filepath.IsAbs(managedRoot) {
		return nil
	}
	var cleaned map[string]string
	for key, path := range env {
		if !strings.EqualFold(key, "PATH") {
			continue
		}
		entries := strings.Split(path, string(filepath.ListSeparator))
		kept := make([]string, 0, len(entries))
		for _, entry := range entries {
			if skillPathKey(strings.Trim(entry, "\"")) != skillPathKey(filepath.Join(managedRoot, "bin")) {
				kept = append(kept, entry)
			}
		}
		if len(kept) == len(entries) {
			continue
		}
		if cleaned == nil {
			cleaned = make(map[string]string, len(env))
			for name, value := range env {
				cleaned[name] = value
			}
		}
		cleaned[key] = strings.Join(kept, string(filepath.ListSeparator))
	}
	return cleaned
}
