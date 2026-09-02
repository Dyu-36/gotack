package guard

import (
	"path/filepath"
	"runtime"
	"strings"
)

// tiers.go -- role: classify Crush tools into approval tiers and provide the
// path arithmetic the write boundary needs.
//
// Tool names mirror the vendored engine's tool registry. The classification
// is deliberately coarse: any tool the guard does not recognise as read-only
// or file-writing lands in the ask tier, because unknown tools must never be
// auto-approved.

// Read-only tools cannot mutate state, so they sit in the auto tier in every
// posture.
var readTools = map[string]bool{
	"ls":          true,
	"glob":        true,
	"grep":        true,
	"view":        true,
	"sourcegraph": true,
}

// File-writing tools carry a file_path and are bounded by the write-safe
// root and the memory context dir.
var writeTools = map[string]bool{
	"write":     true,
	"edit":      true,
	"multiedit": true,
}

// backgroundReviewReadTools are the local tools permitted to a detached
// review. Sourcegraph is intentionally excluded because it is network-backed.
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

// resolvePath turns the tool input's file_path into an absolute path. Crush
// passes workspace-relative paths for the file tools, so a relative target
// is joined against the session's working directory before cleaning.
func resolvePath(cwd, target string) string {
	if !filepath.IsAbs(target) && cwd != "" {
		target = filepath.Join(cwd, target)
	}
	return filepath.Clean(target)
}

// withinPath reports whether target equals root or lives under it. Both
// sides are cleaned first, and the prefix match terminates on a separator so
// "/work/proj" never matches "/work/proj-x". Comparison is case-insensitive
// on the OSes whose default volumes are case-insensitive, matching
// internal/workspace.samePath.
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

// caseInsensitiveFS mirrors the filesystem assumption internal/workspace
// already makes for workspace identity.
func caseInsensitiveFS() bool {
	return runtime.GOOS == "windows" || runtime.GOOS == "darwin"
}
