package attachments

import (
	"bytes"
	"net/http"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

type FileKind int

const (
	KindUnknown FileKind = iota
	KindText
	KindOffice
	KindLegacyOffice
	KindPDF
	KindImage
	KindBinary
)

var officeExtensions = map[string]bool{
	".xlsx": true,
	".docx": true,
	".pptx": true,
}

var legacyOfficeExtensions = map[string]bool{
	".xls":  true,
	".xlsm": true,
	".xlsb": true,
	".ods":  true,
	".doc":  true,
	".rtf":  true,
	".odt":  true,
	".ppt":  true,
	".pps":  true,
	".odp":  true,
}

var textExtensions = map[string]bool{
	".txt":        true,
	".md":         true,
	".markdown":   true,
	".rs":         true,
	".toml":       true,
	".json":       true,
	".yaml":       true,
	".yml":        true,
	".slint":      true,
	".py":         true,
	".js":         true,
	".ts":         true,
	".tsx":        true,
	".jsx":        true,
	".html":       true,
	".htm":        true,
	".css":        true,
	".scss":       true,
	".sh":         true,
	".bat":        true,
	".ps1":        true,
	".sql":        true,
	".ini":        true,
	".cfg":        true,
	".conf":       true,
	".log":        true,
	".xml":        true,
	".c":          true,
	".h":          true,
	".cpp":        true,
	".hpp":        true,
	".java":       true,
	".kt":         true,
	".go":         true,
	".rb":         true,
	".php":        true,
	".lua":        true,
	".env":        true,
	".gitignore":  true,
	".csv":        true,
	".svg":        true,
	".proto":      true,
	".graphql":    true,
	".dockerfile": true,
	".diff":       true,
	".patch":      true,
	".r":          true,
	".swift":      true,
	".dart":       true,
	".zig":        true,
	".nim":        true,
	".scala":      true,
	".perl":       true,
	".asm":        true,
	".vue":        true,
	".svelte":     true,
}

var imageExtensions = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
	".bmp":  "image/bmp",
	".ico":  "image/x-icon",
	".tiff": "image/tiff",
}

func Classify(fileName, declaredMime string, content []byte) (FileKind, string) {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(fileName)))
	declaredMime = strings.TrimSpace(strings.ToLower(declaredMime))

	if officeExtensions[ext] {
		return KindOffice, "text/plain; charset=utf-8"
	}

	if legacyOfficeExtensions[ext] {
		return KindLegacyOffice, "text/plain; charset=utf-8"
	}

	if ext == ".pdf" || declaredMime == "application/pdf" || bytes.HasPrefix(content, []byte("%PDF-")) {
		return KindPDF, "application/pdf"
	}

	if imgMime, ok := imageExtensions[ext]; ok {
		return KindImage, imgMime
	}
	if strings.HasPrefix(declaredMime, "image/") {
		return KindImage, declaredMime
	}

	if textExtensions[ext] || strings.HasPrefix(fileName, ".env") || strings.EqualFold(fileName, "Dockerfile") {
		return KindText, "text/plain; charset=utf-8"
	}
	if strings.HasPrefix(declaredMime, "text/") {
		return KindText, declaredMime
	}

	if isTextContent(content) {
		return KindText, "text/plain; charset=utf-8"
	}

	sniffLen := min(512, len(content))
	sniffed := http.DetectContentType(content[:sniffLen])
	if strings.HasPrefix(sniffed, "image/") {
		return KindImage, sniffed
	}
	if strings.HasPrefix(sniffed, "text/") {
		return KindText, sniffed
	}

	return KindBinary, sniffed
}

func isTextContent(content []byte) bool {
	if len(content) == 0 {
		return true
	}
	if !utf8.Valid(content) {
		return false
	}

	if bytes.IndexByte(content, 0x00) != -1 {
		return false
	}
	return true
}

func MimeForName(fileName string) string {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(fileName)))
	if imgMime, ok := imageExtensions[ext]; ok {
		return imgMime
	}
	switch {
	case ext == ".pdf":
		return "application/pdf"
	case textExtensions[ext]:
		return "text/plain; charset=utf-8"
	}
	return "application/octet-stream"
}
