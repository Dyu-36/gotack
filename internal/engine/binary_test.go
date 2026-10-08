package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewSupervisor_ExplicitBinary(t *testing.T) {
	sup := NewSupervisor(nil, "my-custom-engine.exe")
	if sup.binary != "my-custom-engine.exe" {
		t.Fatalf("expected my-custom-engine.exe, got %s", sup.binary)
	}
}

func TestDefaultBinaryResolvesTackEngineName(t *testing.T) {
	bin := defaultBinary()
	if bin == "" {
		t.Fatal("expected non-empty default engine binary")
	}
	if !strings.Contains(bin, "gotack") {
		t.Fatalf("defaultBinary() = %s, expected gotack", bin)
	}
}

func TestDefaultBinary_EnvOverride(t *testing.T) {
	dir := t.TempDir()
	fakeBinary := filepath.Join(dir, "custom-engine.exe")
	if err := os.WriteFile(fakeBinary, []byte("fake"), 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("GOTACK_ENGINE", fakeBinary)
	t.Setenv("GOTACK_TEST_ENGINE", "")

	got := defaultBinary()
	expected, _ := filepath.Abs(fakeBinary)
	if got != expected {
		t.Fatalf("defaultBinary() with GOTACK_ENGINE = %s, want %s", got, expected)
	}
}

func TestDefaultBinary_TestEnvOverride(t *testing.T) {
	dir := t.TempDir()
	fakeBinary := filepath.Join(dir, "test-engine.exe")
	if err := os.WriteFile(fakeBinary, []byte("fake"), 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("GOTACK_ENGINE", "")
	t.Setenv("GOTACK_TEST_ENGINE", fakeBinary)

	got := defaultBinary()
	expected, _ := filepath.Abs(fakeBinary)
	if got != expected {
		t.Fatalf("defaultBinary() with GOTACK_TEST_ENGINE = %s, want %s", got, expected)
	}
}
