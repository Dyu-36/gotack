package guard

import (
	"path/filepath"
	"runtime"
	"strings"
)

var readTools = map[string]bool{
	"ls":          true,
	"glob":        true,
	"grep":        true,
	"view":        true,
	"sourcegraph": true,
}

var writeTools = map[string]bool{
	"write":     true,
	"edit":      true,
	"multiedit": true,
}

var backgroundReviewReadTools = map[string]bool{
	"ls":   true,
	"glob": true,
	"grep": true,
	"view": true,
}

func isReadTool(name string) bool  { return readTools[name] }
func isWriteTool(name string) bool { return writeTools[name] }

func isSkillTool(name string) bool {
	switch name {
	case "skill_view", "mcp_gotack-skills_skill_view", "skill_manage", "mcp_gotack-skills_skill_manage":
		return true
	default:
		return false
	}
}

func isBackgroundReviewTool(name string) bool {
	return backgroundReviewReadTools[name] ||
		name == "memory" || name == "mcp_gotack-memory_memory" ||
		isSkillTool(name)
}

func resolvePath(cwd, target string) string {
	if !filepath.IsAbs(target) && cwd != "" {
		target = filepath.Join(cwd, target)
	}
	return filepath.Clean(target)
}

func withinPath(root, target string) bool {
	if root == "" || target == "" {
		return false
	}
	root = filepath.Clean(root)
	target = filepath.Clean(target)
	if equalPath(root, target) {
		return true
	}
	prefix := root + string(filepath.Separator)
	return hasPathPrefix(target, prefix)
}

func equalPath(a, b string) bool {
	if caseInsensitiveFS() {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func hasPathPrefix(s, prefix string) bool {
	if caseInsensitiveFS() {
		return len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix)
	}
	return strings.HasPrefix(s, prefix)
}

func caseInsensitiveFS() bool {
	return runtime.GOOS == "windows" || runtime.GOOS == "darwin"
}
