package main

// skills_frontmatter_test.go -- role: regression test pinning the bundled
// skills' frontmatter to the contract the vendored engine actually parses.
// Crush's skill loader requires YAML frontmatter with a valid `name`
// (<= 64 bytes, ^[a-zA-Z0-9]+(-[a-zA-Z0-9]+)*$, equal to the skill directory's
// base name) and a `description` (<= 1024 bytes).

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

var skillNamePattern = regexp.MustCompile(`^[a-zA-Z0-9]+(-[a-zA-Z0-9]+)*$`)

type bundledSkillFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// parseSkillFrontmatter extracts and parses the same YAML block Crush reads.
// Using yaml.v3 here is deliberate: a line-based key/value parser accepts
// invalid YAML such as an unquoted colon inside a description.
func parseSkillFrontmatter(content string) (map[string]string, error) {
	content = strings.TrimPrefix(content, "\uFEFF")
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	lines := strings.Split(content, "\n")

	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) != "" {
			start = i
			break
		}
	}
	if start == -1 || strings.TrimSpace(lines[start]) != "---" {
		return nil, fmt.Errorf("does not open with a --- frontmatter fence")
	}

	end := -1
	for i := start + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return nil, fmt.Errorf("frontmatter fence never closed")
	}

	var parsed bundledSkillFrontmatter
	frontmatter := strings.Join(lines[start+1:end], "\n")
	if err := yaml.Unmarshal([]byte(frontmatter), &parsed); err != nil {
		return nil, fmt.Errorf("invalid YAML frontmatter: %w", err)
	}
	return map[string]string{
		"name":        parsed.Name,
		"description": parsed.Description,
	}, nil
}

func TestParseSkillFrontmatterRejectsUnquotedColon(t *testing.T) {
	content := "---\nname: timetable\ndescription: Create a timetable: from assignments\n---\n"
	if _, err := parseSkillFrontmatter(content); err == nil {
		t.Fatal("invalid YAML with an unquoted colon was accepted")
	}
}

func TestParseSkillFrontmatterAcceptsQuotedColon(t *testing.T) {
	content := "---\nname: timetable\ndescription: \"Create a timetable: from assignments\"\n---\n"
	fields, err := parseSkillFrontmatter(content)
	if err != nil {
		t.Fatalf("quoted YAML description was rejected: %v", err)
	}
	if fields["description"] != "Create a timetable: from assignments" {
		t.Fatalf("description = %q", fields["description"])
	}
}

// TestBundledSkillsFrontmatterMatchesEngineContract walks every bundled
// skill and asserts the fields the engine's skill loader requires. A skill
// that fails this test is skipped by Crush at runtime.
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
			if len(name) > 64 {
				t.Errorf("%s: name %q exceeds 64 bytes", dir, name)
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
		} else if len(description) > 1024 {
			t.Errorf("%s: description exceeds 1024 bytes", dir)
		}
	}
	if checked == 0 {
		t.Fatal("no SKILL.md files found under resources/skills")
	}
}
