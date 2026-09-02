package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/attachments"
	"github.com/Dyu-36/gotack/internal/uievents"
	"github.com/Dyu-36/gotack/internal/userstrings"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type PromptFilePick struct {
	FileName string `json:"file_name"`
	MimeType string `json:"mime_type"`
	Size     int    `json:"size"`
	Path     string `json:"path"`
}

type AttachmentLimitsInfo struct {
	MaxBytes        int `json:"max_bytes"`
	MaxDerivedLines int `json:"max_derived_lines"`
	MaxDerivedBytes int `json:"max_derived_bytes"`
}

func (a *App) AttachmentLimits() AttachmentLimitsInfo {
	return AttachmentLimitsInfo{
		MaxBytes:        appconfig.MaxAttachmentBytes,
		MaxDerivedLines: appconfig.MaxDerivedLines,
		MaxDerivedBytes: appconfig.MaxDerivedBytes,
	}
}

func (a *App) PickPromptFiles() ([]PromptFilePick, error) {
	paths, err := runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title: userstrings.PickFilesTitle,
	})
	if err != nil {
		return nil, err
	}
	return describePromptFiles(paths), nil
}

func (a *App) registerFileDrop() {
	if a.ctx == nil {
		return
	}
	runtime.OnFileDrop(a.ctx, func(_, _ int, paths []string) {
		picks := describePromptFiles(paths)
		if len(picks) == 0 {
			return
		}
		a.emit(uievents.PromptFiles, picks)
	})
}

func describePromptFiles(paths []string) []PromptFilePick {
	out := make([]PromptFilePick, 0, len(paths))
	for _, path := range paths {
		clean := strings.TrimSpace(path)
		if clean == "" {
			continue
		}
		info, err := os.Stat(clean)
		if err != nil || info.IsDir() {
			continue
		}
		name := filepath.Base(clean)
		out = append(out, PromptFilePick{
			FileName: name,
			MimeType: attachments.MimeForName(name),
			Size:     int(info.Size()),
			Path:     clean,
		})
	}
	return out
}
