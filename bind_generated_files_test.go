package main

import (
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestValidateGeneratedFilePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "phân công chuyên môn.xlsx")
	if err := os.WriteFile(path, []byte("xlsx"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := validateGeneratedFilePath(path)
	if err != nil {
		t.Fatalf("validateGeneratedFilePath: %v", err)
	}
	if got != path {
		t.Fatalf("path = %q, want %q", got, path)
	}
}

func TestValidateGeneratedFileURL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kết quả.xlsx")
	if err := os.WriteFile(path, []byte("xlsx"), 0o644); err != nil {
		t.Fatal(err)
	}
	urlPath := filepath.ToSlash(path)
	if runtime.GOOS == "windows" {
		urlPath = "/" + urlPath
	}
	fileURL := (&url.URL{Scheme: "file", Path: urlPath}).String()
	got, err := validateGeneratedFilePath(fileURL)
	if err != nil {
		t.Fatalf("validateGeneratedFilePath URL: %v", err)
	}
	if runtime.GOOS == "windows" {
		path = filepath.Clean(path)
	}
	if got != path {
		t.Fatalf("URL path = %q, want %q", got, path)
	}
}

func TestValidateGeneratedFilePathRejectsUnsafeTargets(t *testing.T) {
	dir := t.TempDir()
	if _, err := validateGeneratedFilePath(dir); err == nil {
		t.Fatal("directory should be rejected")
	}
	executable := filepath.Join(dir, "run.exe")
	if err := os.WriteFile(executable, []byte("not executable"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := validateGeneratedFilePath(executable); err == nil {
		t.Fatal("executable should be rejected")
	}
	missing := filepath.Join(dir, "missing.xlsx")
	if _, err := validateGeneratedFilePath(missing); err == nil {
		t.Fatal("missing file should be rejected")
	}
	if _, err := validateGeneratedFilePath("relative.xlsx"); err == nil {
		t.Fatal("relative path should be rejected")
	}
}

func TestDecodeGeneratedFilePathRejectsNetworkURL(t *testing.T) {
	if _, err := decodeGeneratedFilePath("file://server/share/report.xlsx"); err == nil {
		t.Fatal("network file URL should be rejected")
	}
}
