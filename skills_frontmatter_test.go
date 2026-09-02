package main

// skills_frontmatter_test.go -- role: regression test pinning the bundled
// skills' frontmatter to the contract the vendored engine actually parses.
// Crush's skill loader requires a YAML frontmatter block with a
// valid `name` (<= 64 chars, ^[a-zA-Z0-9]+(-[a-zA-Z0-9]+)*$, equal to the
// skill directory's base name) and a `description` (<= 1024 chars). The
// check is line-based on purpose: no new YAML dependency is allowed, and the
// bundled frontmatter is flat key/value by convention.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

var skillNamePattern = regexp.MustCompile(`^[a-zA-Z0-9]+(-[a-zA-Z0-9]+)*$`)

// parseSkillFrontmatter extracts the flat key/value frontmatter of a
// SKILL.md: the file must open with `---`, and the block ends at the next
// `---` line. Only top-level `key: value` lines are read; nested or quoted
// structures would be a contract violation in a bundled skill anyway.
func parseSkillFrontmatter(content string) (map[string]string, error) {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil, fmt.Errorf("does not open with a --- frontmatter fence")
	}
	fields := map[string]string{}
	for i := 1; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], "\r")
		if strings.TrimSpace(line) == "---" {
			return fields, nil
		}
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			return nil, fmt.Errorf("line %d is not a key: value pair: %q", i+1, line)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(strings.Trim(strings.TrimSpace(value), `"'`))
		if key != "" {
			fields[key] = value
		}
	}
	return nil, fmt.Errorf("frontmatter fence never closed")
}

// TestBundledSkillsFrontmatterMatchesEngineContract walks every bundled
// skill and asserts the fields the engine's skill loader requires. A skill
// that fails this test is silently skipped by Crush at runtime, which is
// exactly the failure the bundled seeding exists to prevent.
func TestBundledSkillsFrontmatterMatchesEngineContract(t *testing.T) {
	dirs, err := filepath.Glob(filepath.Join("resources", "skills", "*"))
	if err != nil {
		t.Fatalf("glob bundled skills: %v", err)
	}
	if len(dirs) == 0 {
		t.Fatal("no bundled skills found under resources/skills")
	}
	checked := 0
	for _, dir := range dirs {
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			continue
		}
		skillFile := filepath.Join(dir, "SKILL.md")
		content, err := os.ReadFile(skillFile)
		if err != nil {
			t.Errorf("%s: read SKILL.md: %v", dir, err)
			continue
		}
		fields, err := parseSkillFrontmatter(string(content))
		if err != nil {
			t.Errorf("%s: %v", dir, err)
			continue
		}
		checked++

		name := fields["name"]
		if name == "" {
			t.Errorf("%s: frontmatter has no name", dir)
		} else {
			if utf8.RuneCountInString(name) > 64 {
				t.Errorf("%s: name %q exceeds 64 characters", dir, name)
			}
			if !skillNamePattern.MatchString(name) {
				t.Errorf("%s: name %q does not match %s", dir, name, skillNamePattern)
			}
			if base := filepath.Base(dir); name != base {
				t.Errorf("%s: name %q must equal the directory base %q", dir, name, base)
			}
		}
		description := fields["description"]
		if description == "" {
			t.Errorf("%s: frontmatter has no description", dir)
		} else if utf8.RuneCountInString(description) > 1024 {
			t.Errorf("%s: description exceeds 1024 characters", dir)
		}
	}
	if checked == 0 {
		t.Fatal("no SKILL.md files found under resources/skills")
	}
}
