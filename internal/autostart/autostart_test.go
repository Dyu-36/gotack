package autostart

import (
	"runtime"
	"testing"
)

func TestSetEnabledRoundTrip(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("registry-backed autostart is Windows-only")
	}
	// Start from a known-off state so the test is idempotent on a dev
	// machine that has the entry toggled through the UI.
	if err := SetEnabled(false); err != nil {
		t.Fatalf("initial SetEnabled(false): %v", err)
	}
	if Enabled() {
		t.Fatal("Enabled() = true right after SetEnabled(false), want false")
	}
	if err := SetEnabled(true); err != nil {
		t.Fatalf("SetEnabled(true): %v", err)
	}
	if !Enabled() {
		t.Fatal("Enabled() = false right after SetEnabled(true), want true")
	}
	if err := SetEnabled(false); err != nil {
		t.Fatalf("final SetEnabled(false): %v", err)
	}
	if Enabled() {
		t.Fatal("Enabled() = true after cleanup, want false")
	}
}

func TestSetEnabledUnsupportedElsewhere(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("stub behavior only applies off-Windows")
	}
	if err := SetEnabled(true); err == nil {
		t.Fatal("SetEnabled on non-Windows returned nil error, want unsupported error")
	}
	if Enabled() {
		t.Fatal("Enabled() on non-Windows = true, want false")
	}
}
