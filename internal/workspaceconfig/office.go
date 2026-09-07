package workspaceconfig

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Dyu-36/gotack/internal/crushapi"
	"github.com/Dyu-36/gotack/internal/workspace"
)

const LegacyOfficeMCPName = "gotack-office"

type OfficeRuntime interface {
	CrushEnv() map[string]string
	SkillsPath() string
}

func ProjectSkillsDir(workspacePath string) string {
	return filepath.Join(workspacePath, ".agents", "skills")
}

func MergeConfigEnv(existing, additions map[string]string) map[string]string {
	merged := make(map[string]string, len(existing)+len(additions))
	for key, value := range existing {
		merged[key] = value
	}
	for key, value := range additions {
		merged[key] = value
	}
	return merged
}

func skillPathKey(path string) string {
	key := filepath.Clean(path)
	if runtime.GOOS == "windows" {
		key = strings.ToLower(key)
	}
	return key
}

func MergeSkillsPaths(existing []string, additions ...string) []string {
	merged := make([]string, 0, len(existing)+len(additions))
	seen := make(map[string]struct{}, len(existing)+len(additions))
	appendPath := func(path string) {
		if path == "" {
			return
		}
		key := skillPathKey(path)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		merged = append(merged, path)
	}
	for _, path := range existing {
		appendPath(path)
	}
	for _, path := range additions {
		appendPath(path)
	}
	return merged
}

func RegisterOffice(base context.Context, api *crushapi.Client, workspaceID string, desc workspace.Descriptor, office OfficeRuntime, userSkillsDir string) error {
	if office == nil {
		return nil
	}
	ctx, cancel := registrationContext(base)
	defer cancel()

	cleanupErr := api.RemoveConfigField(ctx, workspaceID, crushapi.ConfigScopeWorkspace, "mcp_servers."+LegacyOfficeMCPName)
	env := office.CrushEnv()
	additions := make([]string, 0, 3)
	if skillsPath := office.SkillsPath(); skillsPath != "" {
		additions = append(additions, skillsPath)
	}
	if userSkillsDir != "" {
		additions = append(additions, userSkillsDir)
	}
	if desc.Path != "" {
		additions = append(additions, ProjectSkillsDir(desc.Path))
	}
	if len(env) == 0 && len(additions) == 0 {
		if cleanupErr != nil {
			return fmt.Errorf("legacy office MCP cleanup: %w", cleanupErr)
		}
		return nil
	}

	current, err := api.GetWorkspaceConfig(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("office runtime config read: %w", err)
	}
	fields := make(map[string]any, 2)
	if len(env) > 0 {
		fields["env"] = MergeConfigEnv(current.Env, env)
	}
	if len(additions) > 0 {
		fields["options.skills_paths"] = MergeSkillsPaths(current.SkillsPaths(), additions...)
	}
	if err := api.SetConfigFields(ctx, workspaceID, crushapi.ConfigScopeWorkspace, fields); err != nil {
		return fmt.Errorf("office runtime config registration: %w", err)
	}
	if cleanupErr != nil {
		return fmt.Errorf("legacy office MCP cleanup: %w", cleanupErr)
	}
	return nil
}
