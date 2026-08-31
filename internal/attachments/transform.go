package attachments

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Dyu-36/gotack/internal/crushapi"
	"github.com/Dyu-36/gotack/internal/office"
)

const (
	// MaxAttachmentSize is the upper bound on uploaded raw attachment bytes (5 MB).
	MaxAttachmentSize = 5 * 1024 * 1024

	// MaxDerivedLines is the maximum number of text lines sent directly to the model.
	MaxDerivedLines = 2000

	// MaxDerivedBytes is the maximum byte size of text content sent to the model.
	MaxDerivedBytes = 500 * 1024
)

// ClampText limits text lines and bytes, returning the clamped content and whether it was truncated.
func ClampText(text string) (string, bool) {
	var sb strings.Builder
	lines := strings.Split(text, "\n")
	truncated := false

	for i, line := range lines {
		if i >= MaxDerivedLines || sb.Len()+len(line)+1 > MaxDerivedBytes {
			truncated = true
			break
		}
		if i > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(line)
	}

	if truncated {
		sb.WriteString("\n... (đã cắt bớt vì quá dài)")
	}
	return sb.String(), truncated
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

// Process sanitizes, saves to disk cache, classifies, and formats an attachment for Crush.
// Backwards compatible with default vision-enabled assumption.
func Process(fileName, declaredMime string, content []byte) (crushapi.Attachment, error) {
	return ProcessWithModel(fileName, declaredMime, content, true)
}

// ProcessWithModel sanitizes, saves to disk cache, classifies, and formats an attachment for Crush,
// adapting image handling depending on whether the active model supports vision/multimodal input.
func ProcessWithModel(fileName, declaredMime string, content []byte, supportsVision bool) (crushapi.Attachment, error) {
	name := filepath.Base(strings.TrimSpace(fileName))
	if name == "" || name == "." {
		return crushapi.Attachment{}, fmt.Errorf("tên tệp là bắt buộc")
	}

	if len(content) > MaxAttachmentSize {
		return crushapi.Attachment{}, fmt.Errorf("tệp %q vượt quá giới hạn 5 MB", name)
	}

	// 1. Save raw bytes to local disk cache so the agent has a physical file path
	savedPath, err := SaveToCache(name, content)
	if err != nil {
		return crushapi.Attachment{}, fmt.Errorf("lưu tệp đính kèm: %w", err)
	}

	kind, mimeType := Classify(name, declaredMime, content)

	switch kind {
	case KindOffice:
		info, _ := office.Info(savedPath)
		rawDoc, readErr := office.Read(savedPath, "")
		if readErr == nil && strings.TrimSpace(rawDoc) != "" {
			clamped, _ := ClampText(rawDoc)
			var sb strings.Builder
			sb.WriteString("> **Bản chuyển văn bản do Gotack tạo tự động từ tệp đính kèm.**\n")
			sb.WriteString(fmt.Sprintf("> - Tệp gốc: `%s`\n", name))
			sb.WriteString(fmt.Sprintf("> - Đường dẫn tệp trên máy: `%s`\n", savedPath))
			if info != "" {
				sb.WriteString(fmt.Sprintf("> - Tóm tắt: %s\n", info))
			}
			sb.WriteString("\n<NỘI_DUNG_TỆP>\n")
			sb.WriteString(clamped)
			sb.WriteString("\n</NỘI_DUNG_TỆP>")

			return crushapi.Attachment{
				FilePath: savedPath,
				FileName: name,
				MimeType: "text/plain; charset=utf-8",
				Content:  []byte(sb.String()),
			}, nil
		}

		// Fallback if office parsing fails
		var sb strings.Builder
		sb.WriteString("> **Tệp tài liệu Office đã được lưu trên máy tính:**\n")
		sb.WriteString(fmt.Sprintf("> - Tệp gốc: `%s`\n", name))
		sb.WriteString(fmt.Sprintf("> - Đường dẫn tệp trên máy: `%s`\n", savedPath))
		sb.WriteString(fmt.Sprintf("> - Kích thước: %s\n", formatSize(len(content))))
		sb.WriteString("> - Ghi chú cho Agent: Tệp đã được lưu tại đường dẫn trên. Bạn có thể sử dụng các công cụ office_read, office_edit hoặc python để đọc và chỉnh sửa tệp này.\n")

		return crushapi.Attachment{
			FilePath: savedPath,
			FileName: name,
			MimeType: "text/plain; charset=utf-8",
			Content:  []byte(sb.String()),
		}, nil

	case KindText:
		rawText := string(content)
		clamped, _ := ClampText(rawText)
		var sb strings.Builder
		sb.WriteString("> **Tệp văn bản đính kèm:**\n")
		sb.WriteString(fmt.Sprintf("> - Tệp gốc: `%s`\n", name))
		sb.WriteString(fmt.Sprintf("> - Đường dẫn tệp trên máy: `%s`\n", savedPath))
		sb.WriteString("\n<NỘI_DUNG_TỆP>\n")
		sb.WriteString(clamped)
		sb.WriteString("\n</NỘI_DUNG_TỆP>")

		return crushapi.Attachment{
			FilePath: savedPath,
			FileName: name,
			MimeType: "text/plain; charset=utf-8",
			Content:  []byte(sb.String()),
		}, nil

	case KindImage:
		if supportsVision {
			return crushapi.Attachment{
				FilePath: savedPath,
				FileName: name,
				MimeType: mimeType,
				Content:  bytes.Clone(content),
			}, nil
		}

		// Text-only model fallback: perform OCR extraction
		ocrText := ExtractTextFromImage(savedPath)
		var sb strings.Builder
		sb.WriteString("> **Tệp hình ảnh đính kèm (chế độ Model thuần văn bản - Text Only):**\n")
		sb.WriteString(fmt.Sprintf("> - Tệp gốc: `%s`\n", name))
		sb.WriteString(fmt.Sprintf("> - Đường dẫn tệp trên máy: `%s`\n", savedPath))
		sb.WriteString(fmt.Sprintf("> - Kích thước: %s\n", formatSize(len(content))))

		if strings.TrimSpace(ocrText) != "" {
			clamped, _ := ClampText(ocrText)
			sb.WriteString("> - *Hệ thống đã tự động trích xuất nội dung văn bản (OCR) từ hình ảnh cho bạn:*\n")
			sb.WriteString("\n<NỘI_DUNG_TRÍCH_XUẤT_TỪ_ẢNH>\n")
			sb.WriteString(clamped)
			sb.WriteString("\n</NỘI_DUNG_TRÍCH_XUẤT_TỪ_ẢNH>")
		} else {
			sb.WriteString("> - Ghi chú cho Agent: Model hiện tại không hỗ trợ thị giác trực tiếp và không phát hiện thấy văn bản rõ ràng trong ảnh. Tệp ảnh đã được lưu tại đường dẫn trên. Bạn có thể sử dụng các công cụ Python (PIL, OpenCV) hoặc lệnh hệ thống để kiểm tra/xử lý tệp này.\n")
		}

		return crushapi.Attachment{
			FilePath: savedPath,
			FileName: name,
			MimeType: "text/plain; charset=utf-8",
			Content:  []byte(sb.String()),
		}, nil

	default: // KindBinary (e.g. .pdf, .zip, etc.)
		var sb strings.Builder
		sb.WriteString("> **Tệp đính kèm đã được lưu trên máy tính:**\n")
		sb.WriteString(fmt.Sprintf("> - Tệp gốc: `%s`\n", name))
		sb.WriteString(fmt.Sprintf("> - Đường dẫn tệp trên máy: `%s`\n", savedPath))
		sb.WriteString(fmt.Sprintf("> - Kích thước: %s\n", formatSize(len(content))))
		sb.WriteString("> - Ghi chú cho Agent: Tệp đã được lưu sẵn tại đường dẫn cục bộ ở trên. Bạn có thể sử dụng các công cụ hệ thống (bash, python, v.v.) để truy cập và xử lý tệp này.\n")

		return crushapi.Attachment{
			FilePath: savedPath,
			FileName: name,
			MimeType: "text/plain; charset=utf-8",
			Content:  []byte(sb.String()),
		}, nil
	}
}
