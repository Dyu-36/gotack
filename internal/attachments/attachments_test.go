package attachments

import (
	"os"
	"strings"
	"testing"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		name         string
		fileName     string
		declaredMime string
		content      []byte
		wantKind     FileKind
		wantMime     string
	}{
		{
			name:         "Excel spreadsheet",
			fileName:     "report.xlsx",
			declaredMime: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
			content:      []byte("PK\x03\x04..."),
			wantKind:     KindOffice,
			wantMime:     "text/plain; charset=utf-8",
		},
		{
			name:         "Legacy Excel spreadsheet",
			fileName:     "phan cong.xls",
			declaredMime: "application/vnd.ms-excel",
			content:      []byte("\xd0\xcf\x11\xe0\xa1\xb1\x1a\xe1"),
			wantKind:     KindLegacyOffice,
			wantMime:     "text/plain; charset=utf-8",
		},
		{
			name:         "Legacy Word document",
			fileName:     "bien ban.doc",
			declaredMime: "application/msword",
			content:      []byte("\xd0\xcf\x11\xe0\xa1\xb1\x1a\xe1"),
			wantKind:     KindLegacyOffice,
			wantMime:     "text/plain; charset=utf-8",
		},
		{
			name:         "JSON file",
			fileName:     "package.json",
			declaredMime: "application/json",
			content:      []byte(`{"name": "test"}`),
			wantKind:     KindText,
			wantMime:     "text/plain; charset=utf-8",
		},
		{
			name:         "PNG image",
			fileName:     "photo.png",
			declaredMime: "image/png",
			content:      []byte("\x89PNG\r\n\x1a\n"),
			wantKind:     KindImage,
			wantMime:     "image/png",
		},
		{
			name:         "Binary archive",
			fileName:     "archive.zip",
			declaredMime: "application/zip",
			content:      []byte("PK\x03\x04\x00"),
			wantKind:     KindBinary,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kind, mime := Classify(tt.fileName, tt.declaredMime, tt.content)
			if kind != tt.wantKind {
				t.Errorf("Classify(%q) kind = %v, want %v", tt.fileName, kind, tt.wantKind)
			}
			if tt.wantMime != "" && mime != tt.wantMime {
				t.Errorf("Classify(%q) mime = %q, want %q", tt.fileName, mime, tt.wantMime)
			}
		})
	}
}

func TestClampText(t *testing.T) {
	short := "line 1\nline 2\nline 3"
	got, truncated := ClampText(short)
	if truncated || got != short {
		t.Errorf("ClampText(short) = %q, truncated = %v", got, truncated)
	}

	longLines := make([]string, MaxDerivedLines+50)
	for i := range longLines {
		longLines[i] = "content"
	}
	clamped, truncated := ClampText(strings.Join(longLines, "\n"))
	if !truncated {
		t.Errorf("ClampText(long) expected truncated=true")
	}
	if !strings.Contains(clamped, "cắt bớt") {
		t.Errorf("ClampText(long) missing truncation notice: %s", clamped)
	}
}

func TestPrepare(t *testing.T) {
	payload := []byte(`{"a": 1}`)
	prepared, err := Prepare("test.json", "application/json", payload, true)
	if err != nil {
		t.Fatalf("Prepare(test.json) error = %v", err)
	}
	if prepared.MimeType != "text/plain; charset=utf-8" {
		t.Errorf("Prepare() MimeType = %q, want text/plain; charset=utf-8", prepared.MimeType)
	}

	if prepared.Attachment != nil {
		t.Errorf("Prepare() Attachment = %+v, want nil for derived text", prepared.Attachment)
	}
	if !strings.Contains(prepared.PromptBlock, `{"a": 1}`) {
		t.Errorf("Prepare() PromptBlock = %q, want the json payload", prepared.PromptBlock)
	}
	if prepared.Size != len(payload) {
		t.Errorf("Prepare() Size = %d, want %d", prepared.Size, len(payload))
	}
	if _, err := os.Stat(prepared.Path); err != nil {
		t.Errorf("saved attachment missing at %q: %v", prepared.Path, err)
	}

	tooLarge := make([]byte, MaxAttachmentSize+1)
	if _, err := Prepare("large.txt", "text/plain", tooLarge, true); err == nil {
		t.Errorf("Prepare(tooLarge) expected an error for exceeding the limit")
	}
}

func TestComposePromptRoundTrip(t *testing.T) {
	item := Prepared{
		DisplayName: "phan cong.xls",
		Path:        `C:\Users\Admin\AppData\Roaming\gotack\attachments\1a2b3c4d\phan_cong.xls`,
		MimeType:    "text/plain; charset=utf-8",
		Size:        506,
		PromptBlock: "> Tổng 1 dòng.\n\n<NỘI_DUNG_TỆP>\n     1| a\n</NỘI_DUNG_TỆP>",
	}

	prompt := ComposePrompt("Hãy tính tổng doanh thu", []Prepared{item})
	if !strings.HasPrefix(prompt, "Hãy tính tổng doanh thu") {
		t.Fatalf("ComposePrompt() = %q, want the user text first", prompt)
	}
	if !strings.Contains(prompt, item.PromptBlock) {
		t.Errorf("ComposePrompt() dropped the derived content: %q", prompt)
	}

	text, refs := ParseAttachmentBlocks(prompt)
	if text != "Hãy tính tổng doanh thu" {
		t.Errorf("ParseAttachmentBlocks() text = %q, want the original prompt text", text)
	}
	if len(refs) != 1 {
		t.Fatalf("ParseAttachmentBlocks() refs = %d, want 1", len(refs))
	}
	if refs[0].FileName != item.DisplayName || refs[0].Path != item.Path || refs[0].Size != item.Size {
		t.Errorf("ParseAttachmentBlocks() ref = %+v, want %q/%q/%d", refs[0], item.DisplayName, item.Path, item.Size)
	}

	if single := ComposePrompt("", []Prepared{item}); !strings.Contains(single, "tệp đính kèm sau:") {
		t.Errorf("ComposePrompt(no text) = %q, want a generated instruction", single)
	}
	if multi := ComposePrompt("   ", []Prepared{item, item}); !strings.Contains(multi, "các tệp đính kèm sau:") {
		t.Errorf("ComposePrompt(multi) = %q, want the plural instruction", multi)
	}
	if got := ComposePrompt("", nil); got != "" {
		t.Errorf("ComposePrompt(empty) = %q, want an empty string", got)
	}
}

func TestComposePromptFailSoft(t *testing.T) {
	prompt := ComposePrompt("Đọc giúp tôi", []Prepared{Failed("scan.pdf", "chưa hỗ trợ")})
	if !strings.HasPrefix(prompt, "Đọc giúp tôi") {
		t.Errorf("ComposePrompt() dropped the user text: %q", prompt)
	}
	if !strings.Contains(prompt, "scan.pdf") || !strings.Contains(prompt, "chưa hỗ trợ") {
		t.Errorf("ComposePrompt() lost the warning: %q", prompt)
	}
	if strings.Contains(prompt, "<"+attachmentTag) {
		t.Errorf("ComposePrompt() should not wrap a failed file: %q", prompt)
	}
}

func TestDecodeText(t *testing.T) {
	tests := []struct {
		name    string
		content []byte
		want    string
	}{
		{"UTF-8 with BOM", append([]byte{0xEF, 0xBB, 0xBF}, []byte("Xin chào")...), "Xin chào"},
		{"UTF-16LE with BOM", []byte{0xFF, 0xFE, 0x58, 0x00, 0x69, 0x00, 0x6E, 0x00}, "Xin"},
		{"UTF-16BE with BOM", []byte{0xFE, 0xFF, 0x00, 0x58, 0x00, 0x69, 0x00, 0x6E}, "Xin"},
		{"UTF-16LE without BOM", []byte{0x58, 0x00, 0x69, 0x00, 0x6E, 0x00, 0x0A, 0x00}, "Xin\n"},
		{"Windows-1252", []byte{0x93, 'H', 'i', 0x94}, "“Hi”"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, encoding := DecodeText(tt.content)
			if got != tt.want {
				t.Errorf("DecodeText() = %q (encoding %s), want %q", got, encoding, tt.want)
			}
		})
	}
}

func TestSanitizeFileName(t *testing.T) {
	got := SanitizeFileName("phan cong CM HK 2. 25-26 ( Lần 2).XLS")
	if strings.ContainsAny(got, " ()") {
		t.Errorf("SanitizeFileName() = %q, want a shell-safe name", got)
	}
	if !strings.HasSuffix(got, ".xls") {
		t.Errorf("SanitizeFileName() = %q, want the .xls extension preserved", got)
	}
	if !strings.Contains(got, "Lần") {
		t.Errorf("SanitizeFileName() = %q, want Vietnamese letters preserved", got)
	}
	if blank := SanitizeFileName("   "); blank != "attachment.bin" {
		t.Errorf("SanitizeFileName(blank) = %q, want attachment.bin", blank)
	}
}
