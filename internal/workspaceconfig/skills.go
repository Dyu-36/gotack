package workspaceconfig

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/Dyu-36/gotack/internal/engineapi"
	"github.com/Dyu-36/gotack/internal/workspace"
)

func ProjectSkillsDir(workspacePath string) string {
	return filepath.Join(workspacePath, ".agents", "skills")
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

func RegisterSkillsPaths(base context.Context, api *engineapi.Client, workspaceID string, desc workspace.Descriptor, userSkillsDir string) error {
	ctx, cancel := registrationContext(base)
	defer cancel()

	additions := make([]string, 0, 2)
	if userSkillsDir != "" {
		additions = append(additions, userSkillsDir)
	}
	if desc.Path != "" {
		additions = append(additions, ProjectSkillsDir(desc.Path))
	}
	if len(additions) == 0 {
		return nil
	}

	current, err := api.GetWorkspaceConfig(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("skills config read: %w", err)
	}
	merged := MergeSkillsPaths(current.SkillsPaths(), additions...)
	if slices.Equal(merged, current.SkillsPaths()) {
		return nil
	}
	if err := api.SetConfigField(ctx, workspaceID, engineapi.ConfigScopeWorkspace, "options.skills_paths", merged); err != nil {
		return fmt.Errorf("skills config registration: %w", err)
	}
	return nil
}
