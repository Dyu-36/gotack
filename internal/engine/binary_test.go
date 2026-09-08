package engine

import (
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
	if !strings.Contains(bin, "tack-engine") {
		t.Fatalf("defaultBinary() = %s, expected tack-engine", bin)
	}
}
