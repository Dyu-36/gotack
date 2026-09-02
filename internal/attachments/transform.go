package attachments

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/crushapi"
	"github.com/Dyu-36/gotack/internal/office"
)

// transform.go -- role: turn one uploaded file into (a) text the model reads in
// the prompt and (b) at most one native Crush attachment.
//
// Crush's createUserMessage converts every attachment into a binary content
// part, so text placed in Attachment.Content never reaches the prompt the model
// sees. Derived text therefore belongs in PromptBlock, and Attachment stays
// reserved for bytes a multimodal model can consume directly.

// The limits live in internal/appconfig so the host, this package and the
// composer cannot drift apart; App.AttachmentLimits() serves the same numbers to
// the UI instead of letting it hardcode them again.
const (
	// MaxAttachmentSize is the upper bound on raw attachment bytes.
	MaxAttachmentSize = appconfig.MaxAttachmentBytes

	// MaxDerivedLines is the maximum number of text lines sent to the model.
	MaxDerivedLines = appconfig.MaxDerivedLines

	// MaxDerivedBytes is the maximum byte size of text content sent to the model.
	MaxDerivedBytes = appconfig.MaxDerivedBytes
)

// Prepared is one composer file after saving, classification and extraction.
type Prepared struct {
	DisplayName string
	Path        string
	MimeType    string
	Size        int
	// PromptBlock is model-readable text appended to the prompt by ComposePrompt.
	PromptBlock string
	// Attachment carries raw bytes only when the model can consume them.
	Attachment *crushapi.Attachment
	// Warning explains why a file could not be processed; the turn still runs.
	Warning string
}

// Failed builds a Prepared that only reports a problem, so one unreadable file
// never cancels a turn that also carries text or readable attachments.
func Failed(fileName, reason string) Prepared {
	return Prepared{
		DisplayName: fileName,
		Warning:     fmt.Sprintf("Không xử lý được tệp `%s`: %s", fileName, reason),
	}
}

// Prepare saves the file into the attachment cache, classifies it and extracts
// text the model can read. supportsVision selects between sending image bytes
// and falling back to OCR.
func Prepare(fileName, declaredMime string, content []byte, supportsVision bool) (Prepared, error) {
	name := BaseName(fileName)
	if name == "" {
		return Prepared{}, fmt.Errorf("tên tệp là bắt buộc")
	}
	if len(content) > MaxAttachmentSize {
		return Prepared{}, fmt.Errorf("vượt quá giới hạn %s", formatSize(MaxAttachmentSize))
	}

	savedPath, err := SaveToCache(name, content)
	if err != nil {
		return Prepared{}, fmt.Errorf("lưu tệp đính kèm: %w", err)
	}

	return transform(name, declaredMime, content, savedPath, supportsVision), nil
}

// PrepareFile handles a file the host learned about by path: the native picker,
// an OS file drop or an @[path] tag typed in the composer. The bytes are read
// here and the original path is kept, so nothing is copied into the cache and
// the payload never travels through the webview.
func PrepareFile(path string, supportsVision bool) (Prepared, error) {
	clean := strings.TrimSpace(path)
	if clean == "" {
		return Prepared{}, fmt.Errorf("thiếu đường dẫn tệp")
	}
	info, err := os.Stat(clean)
	if err != nil {
		return Prepared{}, fmt.Errorf("không đọc được tệp: %w", err)
	}
	if info.IsDir() {
		return Prepared{}, fmt.Errorf("đường dẫn là thư mục, không phải tệp")
	}
	if info.Size() > int64(MaxAttachmentSize) {
		return Prepared{}, fmt.Errorf("vượt quá hạn mức %s", formatSize(MaxAttachmentSize))
	}
	content, err := os.ReadFile(clean)
	if err != nil {
		return Prepared{}, fmt.Errorf("không đọc được tệp: %w", err)
	}
	return transform(filepath.Base(clean), "", content, clean, supportsVision), nil
}

// transform classifies and extracts a file that already exists on disk. Both
// entry points share it, so an upload and a dropped path build the same prompt.
func transform(name, declaredMime string, content []byte, path string, supportsVision bool) Prepared {
	kind, mimeType := Classify(name, declaredMime, content)
	out := Prepared{DisplayName: name, Path: path, MimeType: mimeType, Size: len(content)}

	switch kind {
	case KindOffice:
		out.PromptBlock = officeBlock(path, "")
	case KindLegacyOffice:
		converted, convErr := ConvertLegacyOffice(path)
		if convErr != nil {
			out.PromptBlock = fallbackBlock(len(content), fmt.Sprintf("Chưa chuyển được sang OOXML để đọc tự động (%s).", convErr))
			break
		}
		out.PromptBlock = officeBlock(converted, fmt.Sprintf("Đã tự động chuyển `%s` sang `%s` để đọc nội dung: `%s`.", filepath.Ext(name), filepath.Ext(converted), converted))
	case KindPDF:
		text, convErr := ExtractTextFromPDF(path)
		if convErr != nil {
			out.PromptBlock = fallbackBlock(len(content), fmt.Sprintf("Chưa trích xuất được văn bản từ PDF (%s). Hãy dùng công cụ đọc tệp vậy đường dẫn ở thuộc tính `path`.", convErr))
			break
		}
		out.PromptBlock = textBlock(text, "")
	case KindText:
		text, encoding := DecodeText(content)
		out.PromptBlock = textBlock(text, encoding)
	case KindImage:
		if supportsVision {
			// The image itself is the payload, so no derived text block.
			attachment := crushapi.Attachment{FilePath: path, FileName: name, MimeType: mimeType, Content: bytes.Clone(content)}
			out.Attachment = &attachment
			break
		}
		out.PromptBlock = imageBlock(path, len(content))
	default:
		out.PromptBlock = fallbackBlock(len(content), "")
	}
	return out
}

// officeBlock extracts document text through internal/office.
func officeBlock(path, note string) string {
	var sb strings.Builder
	if note != "" {
		sb.WriteString("> " + note + "\n")
	}
	if info, err := office.Info(path); err == nil && strings.TrimSpace(info) != "" {
		sb.WriteString("> Cấu trúc: " + strings.TrimSpace(info) + "\n")
	}

	raw, err := office.Read(path, "")
	if err != nil || strings.TrimSpace(raw) == "" {
		sb.WriteString("> Chưa trích xuất được nội dung văn bản. Hãy dùng công cụ `office_read` với đượng dẫn ở thuộc tính `path`.\n")
		return sb.String()
	}
	sb.WriteString(derivedContent(raw))
	return sb.String()
}

// textBlock renders a plain-text or code file, naming the source encoding when
// it was not already UTF-8.
func textBlock(text, encoding string) string {
	var sb strings.Builder
	if encoding != "" && encoding != "UTF-8" {
		sb.WriteString(fmt.Sprintf("> Mã hoá gốc: %s (đã chuyển sang UTF-8).\n", encoding))
	}
	sb.WriteString(derivedContent(text))
	return sb.String()
}

// imageBlock is the text-only-model path: OCR instead of raw image bytes.
func imageBlock(path string, size int) string {
	var sb strings.Builder
	sb.WriteString("> Model hiện tại không nhận ảnh trực tiếp; hệ thống đã thử OCR.\n")
	sb.WriteString(fmt.Sprintf("> Kích thước: %s\n", formatSize(size)))

	ocr := ExtractTextFromImage(path)
	if strings.TrimSpace(ocr) == "" {
		sb.WriteString("> Không phát hiện văn bản trong ảnh. Ảnh đã lưu tại đường dẫn ở thuộc tính `path`.\n")
		return sb.String()
	}
	sb.WriteString(derivedContent(ocr))
	return sb.String()
}

// fallbackBlock is used when no extractor applies. It points at the bundled
// office tooling instead of leaving the agent to install packages at runtime.
func fallbackBlock(size int, reason string) string {
	var sb strings.Builder
	if reason != "" {
		sb.WriteString("> " + reason + "\n")
	}
	sb.WriteString(fmt.Sprintf("> Kích thước: %s\n", formatSize(size)))
	sb.WriteString("> Tệp đã được lưu tại đượng dẫn ở thuộc tính `path`. Hãy đọc bằng công cụ `office_read` hoặc `officecli` đã đóng gói sẵn; không cần cài thêm gói nào.\n")
	return sb.String()
}

// derivedContent renders extracted text with line numbers plus a coverage note,
// so the model knows whether it is looking at the whole file.
func derivedContent(raw string) string {
	normalized := strings.ReplaceAll(strings.ReplaceAll(raw, "\r\n", "\n"), "\r", "\n")
	kept, truncated, total := clampLines(normalized)

	var sb strings.Builder
	if truncated {
		sb.WriteString(fmt.Sprintf("> Hiển thị %d/%d dòng đầu tiên (đã cắt bớt vì quá dài).\n", len(kept), total))
	} else {
		sb.WriteString(fmt.Sprintf("> Tổng %d dòng.\n", total))
	}
	sb.WriteString("\n<NỘI_DUNG_TỆP>\n")
	for i, line := range kept {
		sb.WriteString(fmt.Sprintf("%6d| %s\n", i+1, line))
	}
	sb.WriteString("</NỘI_DUNG_TỆP>")
	return sb.String()
}

// clampLines bounds extracted text by both line count and byte size.
func clampLines(text string) ([]string, bool, int) {
	all := strings.Split(text, "\n")
	kept := make([]string, 0, min(len(all), MaxDerivedLines))
	size := 0
	for i, line := range all {
		if i >= MaxDerivedLines || size+len(line)+1 > MaxDerivedBytes {
			return kept, true, len(all)
		}
		kept = append(kept, line)
		size += len(line) + 1
	}
	return kept, false, len(all)
}

// ClampText limits text by lines and bytes, returning the clamped content and
// whether anything was dropped.
func ClampText(text string) (string, bool) {
	kept, truncated, _ := clampLines(text)
	out := strings.Join(kept, "\n")
	if truncated {
		out += "\n... (đã cắt bớt vì quá dài)"
	}
	return out, truncated
}

func formatSize(bytes int) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}
	if bytes < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
}
