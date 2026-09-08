package memory

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// These are byte/character budgets, not tokenizer-specific token counts.
const MaxPromptFileBytes = 64 * 1024

var ErrPromptFileTooLarge = errors.New("memory file exceeds the bounded read limit")

func Directory(dataDir string) string { return filepath.Join(dataDir, "assistant") }

// ReadPromptFile bounds allocation even when a file was edited outside the tool.
// Missing files are optional; other errors must not silently replace a good snapshot.
func ReadPromptFile(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("memory prompt source must be a regular file")
	}
	if info.Size() > MaxPromptFileBytes {
		return nil, ErrPromptFileTooLarge
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return nil, errors.New("memory prompt source changed during read")
	}
	data, err := io.ReadAll(io.LimitReader(file, MaxPromptFileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxPromptFileBytes {
		return nil, ErrPromptFileTooLarge
	}
	return data, nil
}

// BoundedPrompt sanitizes first, then admits whole entries under the same cap as
// the write API. It never truncates a fact midway or changes the stored file.
func BoundedPrompt(target Target, data []byte) (string, int, error) {
	if len(data) > MaxPromptFileBytes {
		return "", 0, ErrPromptFileTooLarge
	}
	text, err := SanitizeFileForPrompt(target, data)
	if err != nil {
		return "", 0, err
	}
	entries := parseFile(text).Entries
	included := make([]string, 0, len(entries))
	used, omitted := 0, 0
	for _, entry := range entries {
		cost := utf8.RuneCountInString(entry)
		if len(included) > 0 {
			cost += utf8.RuneCountInString(EntryDelimiter)
		}
		if used+cost > CapFor(target) {
			omitted++
			continue
		}
		included = append(included, entry)
		used += cost
	}
	return Render(target, strings.Join(included, EntryDelimiter)), omitted, nil
}

// PromptBodyChars excludes the decorative header from the data budget.
func PromptBodyChars(text string) int {
	return utf8.RuneCountInString(serializeEntries(parseFile(text).Entries))
}

func promptReadError(path string, err error) error {
	return fmt.Errorf("read bounded memory %s: %w", filepath.Base(path), err)
}
