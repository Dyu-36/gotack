// strings.go -- role: centralized user-facing Vietnamese string table.
//
// Package userstrings is the single home of the user-facing Vietnamese
// strings the desktop shell shows outside the webview: native dialog titles,
// attachment pipeline errors and the charset label. Keeping them byte-stable
// in one table means wording changes happen in exactly one place.
//
// Out of scope by design: prompt-block text addressed to the model
// (internal/attachments transform and prompt builders) and the Zalo channel
// wording (internal/zalo), which stay with their owners.
package userstrings

// Native OS dialog titles.
const (
	PickFilesTitle     = "Chọn tệp để gửi cho agent"
	PickWorkspaceTitle = "Chọn thư mục làm việc"
)

// Session and composer messages surfaced to the user as errors or warnings.
const (
	ErrNoModelSelected      = "chưa chọn mô hình"
	AttachmentTooLarge      = "vượt quá giới hạn 5 MB"
	AttachmentInvalidUpload = "nội dung tải lên không hợp lệ"
)

// Attachment pipeline messages: the charset label plus the conversion and
// extraction failures that reach the composer as per-file warnings. The Fmt
// entries are format strings; keep their verbs in sync with the call sites.
const (
	EncodingUTF16NoBOM = "UTF-16 (không BOM)"

	FmtUnsupportedConversion   = "không hỗ trợ chuyển đổi %q"
	ErrConversionResultMissing = "không tạo được tệp kết quả"
	ErrLibreOfficeMissing      = "không tìm thấy LibreOffice (soffice)"
	FmtNoCOMFormat             = "không có định dạng COM cho %q"
	FmtTempDirCreate           = "tạo thư mục tạm: %w"
	ErrTextExtractionFailed    = "không trích xuất được văn bản"
)
