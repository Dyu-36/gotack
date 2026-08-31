package attachments

import (
	"os"
	"strings"
	"testing"

	"github.com/Dyu-36/gotack/internal/crushapi"
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
			name:         "Word document",
			fileName:     "document.docx",
			declaredMime: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
			content:      []byte("PK\x03\x04..."),
			wantKind:     KindOffice,
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
			name:         "Python script",
			fileName:     "main.py",
			declaredMime: "text/x-python",
			content:      []byte("print('hello')\n"),
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
	long := strings.Join(longLines, "\n")
	clamped, truncated := ClampText(long)
	if !truncated {
		t.Errorf("ClampText(long) expected truncated=true")
	}
	if !strings.Contains(clamped, "cắt bớt") {
		t.Errorf("ClampText(long) missing truncation notice: %s", clamped)
	}
}

func TestProcess(t *testing.T) {
	att, err := Process("test.json", "application/json", []byte(`{"a": 1}`))
	if err != nil {
		t.Fatalf("Process(test.json) error = %v", err)
	}
	if att.MimeType != "text/plain; charset=utf-8" {
		t.Errorf("Process() MimeType = %q, want text/plain; charset=utf-8", att.MimeType)
	}
	if !strings.Contains(string(att.Content), `{"a": 1}`) {
		t.Errorf("Process() Content = %q, want to contain json payload", string(att.Content))
	}
	if !strings.Contains(string(att.Content), "Đường dẫn tệp trên máy") {
		t.Errorf("Process() Content missing file path metadata: %s", string(att.Content))
	}

	// Verify file was saved to disk cache
	if _, err := os.Stat(att.FilePath); err != nil {
		t.Errorf("Saved attachment file does not exist at %q: %v", att.FilePath, err)
	}

	tooLarge := make([]byte, MaxAttachmentSize+1)
	if _, err := Process("large.txt", "text/plain", tooLarge); err == nil {
		t.Errorf("Process(tooLarge) expected error for exceeding limit")
	}
}

func TestComposePrompt(t *testing.T) {
	att := crushapi.Attachment{
		FilePath: `C:\Users\Admin\AppData\Roaming\gotack\attachments\12345678-file.xlsx`,
		FileName: "file.xlsx",
	}

	gotCustom := ComposePrompt("Hãy tính tổng doanh thu", []crushapi.Attachment{att})
	if !strings.HasPrefix(gotCustom, "Hãy tính tổng doanh thu") || !strings.Contains(gotCustom, `@[C:\Users\Admin\AppData\Roaming\gotack\attachments\12345678-file.xlsx]`) {
		t.Errorf("ComposePrompt with text = %q", gotCustom)
	}

	gotSingle := ComposePrompt("", []crushapi.Attachment{att})
	if !strings.Contains(gotSingle, "Hãy xem và xử lý tệp đính kèm sau:") || !strings.Contains(gotSingle, `@[C:\Users\Admin\AppData\Roaming\gotack\attachments\12345678-file.xlsx]`) {
		t.Errorf("ComposePrompt single = %q", gotSingle)
	}

	att2 := crushapi.Attachment{
		FilePath: `C:\Users\Admin\AppData\Roaming\gotack\attachments\87654321-photo.png`,
		FileName: "photo.png",
	}
	gotMulti := ComposePrompt("   ", []crushapi.Attachment{att, att2})
	if !strings.Contains(gotMulti, "Hãy xem và xử lý các tệp đính kèm sau:") || !strings.Contains(gotMulti, `@[C:\Users\Admin\AppData\Roaming\gotack\attachments\87654321-photo.png]`) {
		t.Errorf("ComposePrompt multi = %q", gotMulti)
	}

	if gotEmpty := ComposePrompt("", nil); gotEmpty != "" {
		t.Errorf("ComposePrompt empty = %q, want empty", gotEmpty)
	}
}
