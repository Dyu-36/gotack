package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDescribePromptFilesSkipsNonFiles(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "bang diem.xlsx")
	if err := os.WriteFile(file, []byte("PK"), 0o644); err != nil {
		t.Fatal(err)
	}

	picks := describePromptFiles([]string{file, dir, filepath.Join(dir, "khong-co.txt")})

	if len(picks) != 1 {
		t.Fatalf("picks = %#v, want exactly the regular file", picks)
	}
	if picks[0].FileName != "bang diem.xlsx" {
		t.Fatalf("FileName = %q", picks[0].FileName)
	}
	if picks[0].Path != file {
		t.Fatalf("Path = %q, want %q", picks[0].Path, file)
	}
	if picks[0].Size != 2 {
		t.Fatalf("Size = %d, want 2", picks[0].Size)
	}
}

func TestDecodePromptAttachmentsReadsPath(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "ghi chu.txt")
	if err := os.WriteFile(file, []byte("noi dung tep"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := decodePromptAttachments([]PromptAttachment{{FileName: "ghi chu.txt", Path: file}}, false)

	if len(out) != 1 {
		t.Fatalf("out = %#v", out)
	}
	if out[0].Warning != "" {
		t.Fatalf("Warning = %q, want none", out[0].Warning)
	}
	if !strings.Contains(out[0].PromptBlock, "noi dung tep") {
		t.Fatalf("PromptBlock missing file text: %q", out[0].PromptBlock)
	}
}

func TestDecodePromptAttachmentsFailsSoftOnMissingPath(t *testing.T) {
	out := decodePromptAttachments([]PromptAttachment{{
		FileName: "khong-co.txt",
		Path:     filepath.Join(t.TempDir(), "khong-co.txt"),
	}}, false)

	if len(out) != 1 {
		t.Fatalf("out = %#v", out)
	}
	if out[0].Warning == "" {
		t.Fatal("an unreadable path must become a warning, not an error")
	}
}
