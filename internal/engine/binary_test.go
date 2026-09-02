package engine

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestNewSupervisor_ExplicitBinary(t *testing.T) {
	sup := NewSupervisor(nil, "my-custom-engine.exe")
	if sup.binary != "my-custom-engine.exe" {
		t.Fatalf("expected my-custom-engine.exe, got %s", sup.binary)
	}
}

func TestDefaultBinary_ResolvesEngineName(t *testing.T) {
	bin := defaultBinary()
	if bin == "" {
		t.Fatal("expected non-empty default engine binary")
	}

	hasTackEngine := strings.Contains(bin, "tack-engine")
	hasCrush := strings.Contains(bin, "crush")
	if !hasTackEngine && !hasCrush {
		t.Fatalf("defaultBinary() = %s, expected tack-engine or crush", bin)
	}
}

func TestBinaryCandidateSearchOrder(t *testing.T) {
	tmpDir := t.TempDir()
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}

	primaryName := "tack-engine" + ext
	fallbackName := "crush" + ext

	resDir := filepath.Join(tmpDir, "resources")
	if err := os.MkdirAll(resDir, 0o755); err != nil {
		t.Fatal(err)
	}

	primaryPath := filepath.Join(resDir, primaryName)
	fallbackPath := filepath.Join(resDir, fallbackName)

	if err := os.WriteFile(fallbackPath, []byte("dummy fallback"), 0o755); err != nil {
		t.Fatal(err)
	}

	candidatesFallback := []string{
		filepath.Join(tmpDir, "resources", primaryName),
		filepath.Join(tmpDir, primaryName),
		filepath.Join(tmpDir, "resources", fallbackName),
		filepath.Join(tmpDir, fallbackName),
	}
	var foundFallback string
	for _, c := range candidatesFallback {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			foundFallback = c
			break
		}
	}
	if foundFallback != fallbackPath {
		t.Fatalf("expected fallback %s, got %s", fallbackPath, foundFallback)
	}

	if err := os.WriteFile(primaryPath, []byte("dummy primary"), 0o755); err != nil {
		t.Fatal(err)
	}

	var foundPrimary string
	for _, c := range candidatesFallback {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			foundPrimary = c
			break
		}
	}
	if foundPrimary != primaryPath {
		t.Fatalf("expected primary %s, got %s", primaryPath, foundPrimary)
	}
}
