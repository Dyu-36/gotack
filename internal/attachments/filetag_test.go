package attachments

import "testing"

func TestFileTags(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantVisible string
		wantPaths   []string
	}{
		{
			name:        "bracket tag is removed and collected",
			input:       `doc file @[C:\Users\Admin\report.xls] roi tra loi`,
			wantVisible: "doc file roi tra loi",
			wantPaths:   []string{`C:\Users\Admin\report.xls`},
		},
		{
			name:        "quoted tag with spaces in the path",
			input:       `@"C:\so lieu\bang diem.xlsx" tong hop giup`,
			wantVisible: "tong hop giup",
			wantPaths:   []string{`C:\so lieu\bang diem.xlsx`},
		},
		{
			name:        "same path twice yields one attachment",
			input:       `@[C:\a.txt] @[C:\a.txt]`,
			wantVisible: "",
			wantPaths:   []string{`C:\a.txt`},
		},
		{
			name:        "unterminated tag stays literal text",
			input:       `@[C:\a.txt`,
			wantVisible: `@[C:\a.txt`,
			wantPaths:   nil,
		},
		{
			name:        "plain text is untouched",
			input:       "khong co tag nao trong cau nay",
			wantVisible: "khong co tag nao trong cau nay",
			wantPaths:   nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			visible, paths := FileTags(tc.input)
			if visible != tc.wantVisible {
				t.Fatalf("visible = %q, want %q", visible, tc.wantVisible)
			}
			if len(paths) != len(tc.wantPaths) {
				t.Fatalf("paths = %#v, want %#v", paths, tc.wantPaths)
			}
			for i := range paths {
				if paths[i] != tc.wantPaths[i] {
					t.Fatalf("paths[%d] = %q, want %q", i, paths[i], tc.wantPaths[i])
				}
			}
		})
	}
}
