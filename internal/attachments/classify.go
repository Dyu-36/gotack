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
	KindLegacyOffice
	KindPDF
	KindImage
	KindBinary
)

// officeExtensions are the OOXML formats internal/office reads directly.
var officeExtensions = map[string]bool{
	".xlsx": true,
	".docx": true,
	".pptx": true,
}

// legacyOfficeExtensions are documents internal/office cannot open. They are
// converted to their OOXML twin first (see legacy_office.go); without this a
// .xls attachment fell through to KindBinary and the model received a bare path.
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

// Classify determines the FileKind and effective MIME type for an attachment.
func Classify(fileName, declaredMime string, content []byte) (FileKind, string) {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(fileName)))
	declaredMime = strings.TrimSpace(strings.ToLower(declaredMime))

	// 1. Check office documents internal/office reads directly
	if officeExtensions[ext] {
		return KindOffice, "text/plain; charset=utf-8"
	}

	// 2. Check legacy/OpenDocument files, converted to OOXML before extraction
	if legacyOfficeExtensions[ext] {
		return KindLegacyOffice, "text/plain; charset=utf-8"
	}

	// 3. Check PDFs, turned into text by pdf.go. The magic number matters: a
	// PDF saved from a browser often arrives without a usable extension.
	if ext == ".pdf" || declaredMime == "application/pdf" || bytes.HasPrefix(content, []byte("%PDF-")) {
		return KindPDF, "application/pdf"
	}

	// 4. Check known image extensions or image MIME
	if imgMime, ok := imageExtensions[ext]; ok {
		return KindImage, imgMime
	}
	if strings.HasPrefix(declaredMime, "image/") {
		return KindImage, declaredMime
	}

	// 5. Check known text/code extensions
	if textExtensions[ext] || strings.HasPrefix(fileName, ".env") || strings.EqualFold(fileName, "Dockerfile") {
		return KindText, "text/plain; charset=utf-8"
	}
	if strings.HasPrefix(declaredMime, "text/") {
		return KindText, declaredMime
	}

	// 6. Inspect content: if valid UTF-8 without NUL bytes, treat as text
	if isTextContent(content) {
		return KindText, "text/plain; charset=utf-8"
	}

	// 7. Sniff MIME type using standard library
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

// MimeForName is a best-effort MIME type derived from a file extension. It is
// used when the host learns about a file by path (picker, OS drop, @[tag]) and
// no browser supplied a type; Classify still sniffs the content afterwards.
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
