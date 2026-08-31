package attachments

import (
	"bytes"
	"net/http"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// FileKind distinguishes how an attachment should be treated by the engine.
type FileKind int

const (
	KindUnknown FileKind = iota
	KindText
	KindOffice
	KindImage
	KindBinary
)

var officeExtensions = map[string]bool{
	".xlsx": true,
	".docx": true,
	".pptx": true,
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

// Classify determines the FileKind and effective MIME type for an attachment.
func Classify(fileName, declaredMime string, content []byte) (FileKind, string) {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(fileName)))
	declaredMime = strings.TrimSpace(strings.ToLower(declaredMime))

	// 1. Check office documents
	if officeExtensions[ext] {
		return KindOffice, "text/plain; charset=utf-8"
	}

	// 2. Check known image extensions or image MIME
	if imgMime, ok := imageExtensions[ext]; ok {
		return KindImage, imgMime
	}
	if strings.HasPrefix(declaredMime, "image/") {
		return KindImage, declaredMime
	}

	// 3. Check known text/code extensions
	if textExtensions[ext] || strings.HasPrefix(fileName, ".env") || strings.EqualFold(fileName, "Dockerfile") {
		return KindText, "text/plain; charset=utf-8"
	}
	if strings.HasPrefix(declaredMime, "text/") {
		return KindText, declaredMime
	}

	// 4. Inspect content: if valid UTF-8 without NUL bytes, treat as text
	if isTextContent(content) {
		return KindText, "text/plain; charset=utf-8"
	}

	// 5. Sniff MIME type using standard library
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
	// Binary files often contain NUL (0x00) bytes
	if bytes.IndexByte(content, 0x00) != -1 {
		return false
	}
	return true
}
