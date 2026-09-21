package main

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/Dyu-36/gotack/internal/engineapi"
)

var coreAgentTools = []string{"read", "powershell", "edit", "write", "grep", "glob"}

type AgentSettingsInfo struct {
	Tools         []string `json:"tools"`
	DisabledTools []string `json:"disabled_tools"`
}

func normalizeDisabledTools(input []string) ([]string, error) {
	seen := make(map[string]struct{}, len(input))
	out := make([]string, 0, len(input))
	for _, raw := range input {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		if !slices.Contains(coreAgentTools, name) {
			return nil, fmt.Errorf("unknown core tool %q", name)
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	slices.Sort(out)
	return out, nil
}

func (a *App) GetAgentSettings() (AgentSettingsInfo, error) {
	svc, err := a.services()
	if err != nil {
		return AgentSettingsInfo{}, err
	}
	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Second)
	defer cancel()
	workspaceID, err := a.configWorkspaceID(ctx, svc)
	if err != nil {
		return AgentSettingsInfo{}, err
	}
	cfg, err := svc.api.GetWorkspaceConfig(ctx, workspaceID)
	if err != nil {
		return AgentSettingsInfo{}, err
	}
	var disabled []string
	if cfg.Options != nil {
		disabled, err = normalizeDisabledTools(cfg.Options.DisabledTools)
		if err != nil {
			return AgentSettingsInfo{}, err
		}
	}
	return AgentSettingsInfo{Tools: append([]string(nil), coreAgentTools...), DisabledTools: disabled}, nil
}

func (a *App) SaveAgentSettings(disabledTools []string) (AgentSettingsInfo, error) {
	disabled, err := normalizeDisabledTools(disabledTools)
	if err != nil {
		return AgentSettingsInfo{}, err
	}
	svc, err := a.services()
	if err != nil {
		return AgentSettingsInfo{}, err
	}
	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Second)
	defer cancel()
	workspaceID, err := a.configWorkspaceID(ctx, svc)
	if err != nil {
		return AgentSettingsInfo{}, err
	}
	if len(disabled) == 0 {
		if err := svc.api.RemoveConfigField(ctx, workspaceID, engineapi.ConfigScopeGlobal, "options.disabled_tools"); err != nil {
			return AgentSettingsInfo{}, err
		}
	} else if err := svc.api.SetConfigField(ctx, workspaceID, engineapi.ConfigScopeGlobal, "options.disabled_tools", disabled); err != nil {
		return AgentSettingsInfo{}, err
	}
	if desc, ok := svc.ws.Current(); ok {
		_ = svc.api.RefreshPromptContext(ctx, desc.WorkspaceID)
	}
	return AgentSettingsInfo{Tools: append([]string(nil), coreAgentTools...), DisabledTools: disabled}, nil
}
