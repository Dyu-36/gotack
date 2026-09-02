package skillmanage

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

const (
	maxDescriptionLength = 1024
	maxCompatibility     = 500
)

var skillNamePattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type frontmatter struct {
	Name          string `yaml:"name"`
	Description   string `yaml:"description"`
	Compatibility string `yaml:"compatibility"`
}

func validateOperation(operation Operation) error {
	if err := validateName(operation.Name); err != nil {
		return err
	}
	switch operation.Action {
	case actionCreate:
		if err := validateCategory(operation.Category); err != nil {
			return err
		}
		if operation.OldString != "" || operation.NewString != nil || operation.FilePath != "" || operation.FileContent != nil || operation.AbsorbedInto != "" {
			return errors.New("create accepts only name, content, and optional category")
		}
		if err := validateSkillContent(operation.Name, operation.Content, true); err != nil {
			return err
		}
	case actionPatch:
		if operation.Content != "" || operation.Category != "" || operation.FileContent != nil || operation.AbsorbedInto != "" {
			return errors.New("patch accepts only name, old_string, new_string, and optional file_path")
		}
		if operation.OldString == "" {
			return errors.New("old_string is required for patch; load the exact current file with Crush view first")
		}
		if operation.NewString == nil {
			return errors.New("new_string is required for patch; use an empty string to delete the match")
		}
		if operation.OldString == *operation.NewString {
			return errors.New("old_string and new_string are identical")
		}
		if operation.FilePath != "" {
			if _, err := normalizeSupportPath(operation.FilePath); err != nil {
				return err
			}
		}
	case actionDelete:
		if operation.Content != "" || operation.Category != "" || operation.OldString != "" || operation.NewString != nil || operation.FilePath != "" || operation.FileContent != nil {
			return errors.New("delete accepts only name and optional absorbed_into")
		}
		if operation.AbsorbedInto != "" {
			if err := validateName(operation.AbsorbedInto); err != nil {
				return fmt.Errorf("absorbed_into: %w", err)
			}
		}
	case actionWriteFile:
		if operation.Content != "" || operation.Category != "" || operation.OldString != "" || operation.NewString != nil || operation.AbsorbedInto != "" {
			return errors.New("write_file accepts only name, file_path, and file_content")
		}
		if _, err := normalizeSupportPath(operation.FilePath); err != nil {
			return err
		}
		if operation.FileContent == nil {
			return errors.New("file_content is required for write_file")
		}
		if err := validateSupportContent(*operation.FileContent, operation.FilePath); err != nil {
			return err
		}
	case actionRemoveFile:
		if operation.Content != "" || operation.Category != "" || operation.OldString != "" || operation.NewString != nil || operation.FileContent != nil || operation.AbsorbedInto != "" {
			return errors.New("remove_file accepts only name and file_path")
		}
		if _, err := normalizeSupportPath(operation.FilePath); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown action %q; use create, patch, delete, write_file, or remove_file", operation.Action)
	}
	return nil
}

func validateName(name string) error {
	if name == "" {
		return errors.New("skill name is required")
	}
	if utf8.RuneCountInString(name) > MaxNameLength {
		return fmt.Errorf("skill name exceeds %d characters", MaxNameLength)
	}
	if !skillNamePattern.MatchString(name) {
		return fmt.Errorf("invalid skill name %q: use lowercase letters, numbers, and single hyphens", name)
	}
	return nil
}

func validateCategory(category string) error {
	if category == "" {
		return nil
	}
	if err := validateName(category); err != nil {
		return fmt.Errorf("invalid category: %w", err)
	}
	return nil
}

func validCategoryName(category string) bool {
	return validateCategory(category) == nil && category != ""
}

func validateSkillContent(expectedName, content string, newSkill bool) error {
	if !utf8.ValidString(content) {
		return errors.New("SKILL.md must be valid UTF-8")
	}
	if err := validateCharacterLimit(content, "SKILL.md"); err != nil {
		return err
	}
	metadata, body, err := parseSkillMetadata(content)
	if err != nil {
		return err
	}
	if metadata.Name != expectedName {
		return fmt.Errorf("frontmatter name %q must match skill name %q", metadata.Name, expectedName)
	}
	if err := validateName(metadata.Name); err != nil {
		return fmt.Errorf("frontmatter: %w", err)
	}
	description := strings.TrimSpace(metadata.Description)
	if description == "" {
		return errors.New("frontmatter description is required")
	}
	limit := maxDescriptionLength
	if newSkill {
		limit = MaxDescriptionChars
	}
	if count := utf8.RuneCountInString(description); count > limit {
		return fmt.Errorf("frontmatter description is %d characters (limit: %d)", count, limit)
	}
	if count := utf8.RuneCountInString(metadata.Compatibility); count > maxCompatibility {
		return fmt.Errorf("frontmatter compatibility is %d characters (limit: %d)", count, maxCompatibility)
	}
	if strings.TrimSpace(body) == "" {
		return errors.New("SKILL.md needs instructions after frontmatter")
	}
	return nil
}

func parseSkillMetadata(content string) (frontmatter, string, error) {
	normalized := strings.TrimPrefix(content, "\uFEFF")
	normalized = strings.ReplaceAll(normalized, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	lines := strings.Split(normalized, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return frontmatter{}, "", errors.New("SKILL.md must start with YAML frontmatter (---)")
	}
	end := -1
	for index := 1; index < len(lines); index++ {
		if strings.TrimSpace(lines[index]) == "---" {
			end = index
			break
		}
	}
	if end < 0 {
		return frontmatter{}, "", errors.New("SKILL.md frontmatter is not closed")
	}
	var metadata frontmatter
	if err := yaml.Unmarshal([]byte(strings.Join(lines[1:end], "\n")), &metadata); err != nil {
		return frontmatter{}, "", fmt.Errorf("parse YAML frontmatter: %w", err)
	}
	if metadata.Name == "" {
		return frontmatter{}, "", errors.New("frontmatter name is required")
	}
	return metadata, strings.Join(lines[end+1:], "\n"), nil
}

func validateCharacterLimit(content, label string) error {
	if count := utf8.RuneCountInString(content); count > MaxSkillContent {
		return fmt.Errorf("%s is %d characters (limit: %d)", label, count, MaxSkillContent)
	}
	return nil
}

func validateSupportContent(content, label string) error {
	if !utf8.ValidString(content) {
		return fmt.Errorf("%s must be valid UTF-8", label)
	}
	if size := len([]byte(content)); size > MaxSupportFileBytes {
		return fmt.Errorf("%s is %d bytes (limit: %d / 1 MiB)", label, size, MaxSupportFileBytes)
	}
	return nil
}

func normalizeSupportPath(path string) (string, error) {
	if path == "" {
		return "", errors.New("file_path is required")
	}
	if strings.ContainsRune(path, 0) || strings.Contains(path, ":") {
		return "", errors.New("file_path contains an unsafe character")
	}
	normalized := strings.ReplaceAll(path, "\\", "/")
	if strings.HasPrefix(normalized, "/") || filepath.IsAbs(filepath.FromSlash(normalized)) || filepath.VolumeName(filepath.FromSlash(normalized)) != "" {
		return "", errors.New("file_path must be relative to the skill directory")
	}
	for _, part := range strings.Split(normalized, "/") {
		if part == ".." {
			return "", errors.New("file_path traversal is not allowed")
		}
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(normalized)))
	parts := strings.Split(clean, "/")
	if len(parts) < 2 || parts[len(parts)-1] == "." || parts[len(parts)-1] == "" {
		return "", errors.New("file_path must name a file below an allowed support directory")
	}
	switch parts[0] {
	case "references", "templates", "scripts", "assets":
	default:
		return "", errors.New("file_path must be under references, templates, scripts, or assets")
	}
	return clean, nil
}
