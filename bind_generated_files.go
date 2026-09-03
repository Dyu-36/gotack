package main

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Dyu-36/gotack/internal/fileopen"
)

var generatedFileExtensions = map[string]struct{}{
	".csv": {}, ".doc": {}, ".docx": {}, ".jpeg": {}, ".jpg": {},
	".json": {}, ".md": {}, ".pdf": {}, ".png": {}, ".ppt": {},
	".pptx": {}, ".txt": {}, ".webp": {}, ".xls": {}, ".xlsx": {},
}

func decodeGeneratedFilePath(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("generated file path is required")
	}
	if !strings.HasPrefix(strings.ToLower(value), "file:") {
		return value, nil
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("parse generated file URL: %w", err)
	}
	if parsed.Scheme != "file" {
		return "", fmt.Errorf("unsupported generated file scheme %q", parsed.Scheme)
	}
	if parsed.Host != "" && !strings.EqualFold(parsed.Host, "localhost") {
		return "", errors.New("network file URLs are not supported")
	}
	path, err := url.PathUnescape(parsed.EscapedPath())
	if err != nil {
		return "", fmt.Errorf("decode generated file URL: %w", err)
	}
	if runtime.GOOS == "windows" && len(path) >= 3 && path[0] == '/' && path[2] == ':' {
		path = path[1:]
	}
	return filepath.FromSlash(path), nil
}

func validateGeneratedFilePath(value string) (string, error) {
	path, err := decodeGeneratedFilePath(value)
	if err != nil {
		return "", err
	}
	path = filepath.Clean(path)
	lowerPath := strings.ToLower(path)
	if strings.HasPrefix(lowerPath, `\\.\`) || strings.HasPrefix(lowerPath, `\\?\`) {
		return "", errors.New("device paths are not supported")
	}
	if !filepath.IsAbs(path) {
		return "", errors.New("generated file path must be absolute")
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("stat generated file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("generated file path is not a regular file")
	}
	if _, ok := generatedFileExtensions[strings.ToLower(filepath.Ext(path))]; !ok {
		return "", fmt.Errorf("generated file type %q is not openable", filepath.Ext(path))
	}
	return path, nil
}

func (a *App) OpenGeneratedFile(path string) error {
	validated, err := validateGeneratedFilePath(path)
	if err != nil {
		return err
	}
	return fileopen.Open(validated)
}
func (a *App) RevealGeneratedFile(path string) error {
	validated, err := validateGeneratedFilePath(path)
	if err != nil {
		return err
	}
	return fileopen.Reveal(validated)
}
