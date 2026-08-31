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

// bind_files.go -- role: path-based attachments. Native multi-file picker, OS
// file drop, and the limits the composer enforces before it reads a byte.
//
// A path send carries no base64 body: the UI passes name/mime/size/path and
// SendPrompt reads the bytes host-side. That keeps a multi-megabyte spreadsheet
// out of the webview and off the Wails call payload entirely.

// PromptFilePick describes one file chosen or dropped by path.
type PromptFilePick struct {
	FileName string `json:"file_name"`
	MimeType string `json:"mime_type"`
	Size     int    `json:"size"`
	Path     string `json:"path"`
}

// AttachmentLimitsInfo mirrors internal/appconfig so the composer enforces the
// same numbers as the host instead of hardcoding 5 MB a second time.
type AttachmentLimitsInfo struct {
	MaxBytes        int `json:"max_bytes"`
	MaxDerivedLines int `json:"max_derived_lines"`
	MaxDerivedBytes int `json:"max_derived_bytes"`
}

// AttachmentLimits reports the attachment limits enforced by the host.
func (a *App) AttachmentLimits() AttachmentLimitsInfo {
	return AttachmentLimitsInfo{
		MaxBytes:        appconfig.MaxAttachmentBytes,
		MaxDerivedLines: appconfig.MaxDerivedLines,
		MaxDerivedBytes: appconfig.MaxDerivedBytes,
	}
}

// PickPromptFiles opens the native multi-select dialog and returns the chosen
// files as paths. An empty slice means the user cancelled.
func (a *App) PickPromptFiles() ([]PromptFilePick, error) {
	paths, err := runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title: userstrings.PickFilesTitle,
	})
	if err != nil {
		return nil, err
	}
	return describePromptFiles(paths), nil
}

// registerFileDrop wires the OS drop target to the composer. Wails reports
// absolute paths, so dropped files take the same route as the picker and the
// webview never reads or encodes their bytes.
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

// describePromptFiles stats each path so the UI can show a chip immediately.
// Directories and unreadable paths are skipped here; an oversized file is kept
// on purpose so SendPrompt reports it as a per-file warning instead of silently
// dropping a file the user can see in the composer.
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
