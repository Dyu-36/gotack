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

func ProjectSkillsDirs(workspacePath string) []string {
	if strings.TrimSpace(workspacePath) == "" {
		return nil
	}
	return []string{
		filepath.Join(workspacePath, ".pi", "skills"),
		filepath.Join(workspacePath, ".agents", "skills"),
	}
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

// RegisterSkillsPathsWithTrust keeps user and bundled skills available in every
// workspace while including project-local .pi/skills and .agents/skills only
// after the desktop host has explicitly trusted that project. Existing managed
// project-skill entries are removed again when trust is revoked.
func RegisterSkillsPathsWithTrust(base context.Context, api *engineapi.Client, workspaceID string, desc workspace.Descriptor, userSkillsDir string, projectTrusted bool, bundledSkillsDirs ...string) error {
	ctx, cancel := registrationContext(base)
	defer cancel()

	projectSkills := ProjectSkillsDirs(desc.Path)
	additions := make([]string, 0, 1+len(projectSkills)+len(bundledSkillsDirs))
	if userSkillsDir != "" {
		additions = append(additions, userSkillsDir)
	}
	if projectTrusted {
		additions = append(additions, projectSkills...)
	}
	if len(additions) == 0 && len(bundledSkillsDirs) == 0 && len(projectSkills) == 0 {
		return nil
	}

	current, err := api.GetWorkspaceConfig(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("skills config read: %w", err)
	}

	kept := make([]string, 0, len(current.SkillsPaths())+len(bundledSkillsDirs))
	kept = append(kept, bundledSkillsDirs...)
	for _, path := range current.SkillsPaths() {
		if slices.ContainsFunc(projectSkills, func(projectPath string) bool {
			return skillPathKey(path) == skillPathKey(projectPath)
		}) {
			continue
		}
		managed := slices.ContainsFunc(bundledSkillsDirs, func(bundled string) bool {
			return bundled != "" && skillPathKey(filepath.Dir(path)) == skillPathKey(filepath.Dir(bundled))
		})
		standard := userSkillsDir != "" && skillPathKey(path) == skillPathKey(userSkillsDir)
		if !managed && !standard {
			kept = append(kept, path)
		}
	}
	merged := MergeSkillsPaths(kept, additions...)
	if slices.Equal(merged, current.SkillsPaths()) {
		return nil
	}
	if err := api.SetConfigField(ctx, workspaceID, engineapi.ConfigScopeWorkspace, "options.skills_paths", merged); err != nil {
		return fmt.Errorf("skills config registration: %w", err)
	}
	return nil
}
