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

const (
	MaxAttachmentSize = appconfig.MaxAttachmentBytes

	MaxDerivedLines = appconfig.MaxDerivedLines

	MaxDerivedBytes = appconfig.MaxDerivedBytes
)

type Prepared struct {
	DisplayName string
	Path        string
	MimeType    string
	Size        int

	PromptBlock string

	Attachment *crushapi.Attachment

	Warning string
}

func Failed(fileName, reason string) Prepared {
	return Prepared{
		DisplayName: fileName,
		Warning:     fmt.Sprintf("Không xử lý được tệp `%s`: %s", fileName, reason),
	}
}

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
		sb.WriteString("> Chưa trích xuất được nội dung văn bản. Hãy dùng công cụ `officecli` với đường dẫn ở thuộc tính `path`.\n")
		return sb.String()
	}
	sb.WriteString(derivedContent(raw))
	return sb.String()
}

func textBlock(text, encoding string) string {
	var sb strings.Builder
	if encoding != "" && encoding != "UTF-8" {
		sb.WriteString(fmt.Sprintf("> Mã hoá gốc: %s (đã chuyển sang UTF-8).\n", encoding))
	}
	sb.WriteString(derivedContent(text))
	return sb.String()
}

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

func fallbackBlock(size int, reason string) string {
	var sb strings.Builder
	if reason != "" {
		sb.WriteString("> " + reason + "\n")
	}
	sb.WriteString(fmt.Sprintf("> Kích thước: %s\n", formatSize(size)))
	sb.WriteString("> Tệp đã được lưu tại đường dẫn ở thuộc tính `path`. Hãy đọc bằng công cụ `officecli` hoặc `officecli` đã đóng gói sẵn; không cần cài thêm gói nào.\n")
	return sb.String()
}

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
