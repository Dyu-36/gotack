package main

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestDefaultWorkspacePath(t *testing.T) {
	got := filepath.Clean(defaultWorkspacePath())
	if runtime.GOOS == "windows" {
		if got != filepath.Clean(`C:\`) {
			t.Fatalf("defaultWorkspacePath() = %q, want C:\\", got)
		}
	} else if got != filepath.Clean(string(filepath.Separator)) {
		t.Fatalf("defaultWorkspacePath() = %q, want filesystem root", got)
	}
	if !isDefaultWorkspace(defaultWorkspacePath()) {
		t.Fatal("default workspace path is not recognized as default")
	}
}
