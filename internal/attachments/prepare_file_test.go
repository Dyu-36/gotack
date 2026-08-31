package attachments

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPrepareFileReadsTextFromDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ghi chu.txt")
	if err := os.WriteFile(path, []byte("dong mot\ndong hai\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	prepared, err := PrepareFile(path, false)
	if err != nil {
		t.Fatalf("PrepareFile: %v", err)
	}
	if prepared.DisplayName != "ghi chu.txt" {
		t.Fatalf("DisplayName = %q", prepared.DisplayName)
	}
	if prepared.Path != path {
		t.Fatalf("Path = %q, want %q", prepared.Path, path)
	}
	if prepared.Size != len("dong mot\ndong hai\n") {
		t.Fatalf("Size = %d", prepared.Size)
	}
	if !strings.Contains(prepared.PromptBlock, "dong hai") {
		t.Fatalf("PromptBlock missing file text: %q", prepared.PromptBlock)
	}
	if prepared.Attachment != nil {
		t.Fatal("text file must not become a binary attachment")
	}
}

func TestPrepareFileRejectsBadPaths(t *testing.T) {
	dir := t.TempDir()

	if _, err := PrepareFile("  ", false); err == nil {
		t.Fatal("empty path must fail")
	}
	if _, err := PrepareFile(filepath.Join(dir, "khong-ton-tai.txt"), false); err == nil {
		t.Fatal("missing file must fail")
	}
	if _, err := PrepareFile(dir, false); err == nil {
		t.Fatal("directory must fail")
	}
}

func TestPruneCacheDropsExpiredEntries(t *testing.T) {
	dir := t.TempDir()
	fresh, stale := filepath.Join(dir, "fresh"), filepath.Join(dir, "stale")
	for _, sub := range []string{fresh, stale} {
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(sub, "a.txt"), []byte("noi dung"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(filepath.Join(stale, "a.txt"), old, old); err != nil {
		t.Fatal(err)
	}

	pruneCache(dir, time.Hour, 1<<40)

	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("expired entry survived: %v", err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Fatalf("fresh entry was removed: %v", err)
	}
}

func TestPruneCacheEnforcesBudget(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"one", "two"} {
		sub := filepath.Join(dir, name)
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(sub, "a.bin"), make([]byte, 1024), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Both entries are fresh, so only the budget can trim them.
	pruneCache(dir, 24*time.Hour, 512)

	left, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(left) > 1 {
		t.Fatalf("budget not enforced, %d entries left", len(left))
	}
}
