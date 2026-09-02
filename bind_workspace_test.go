package main

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Dyu-36/gotack/internal/appconfig"
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

func TestPermissionsSkipRequiresExplicitOptIn(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  *appconfig.Config
		want bool
	}{
		{name: "nil config", cfg: nil, want: false},
		{name: "default config", cfg: appconfig.Defaults(), want: false},
		{name: "explicit false", cfg: &appconfig.Config{AutoApprove: false}, want: false},
		{name: "explicit true", cfg: &appconfig.Config{AutoApprove: true}, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := &App{cfg: tc.cfg}
			if got := app.permissionsSkip(); got != tc.want {
				t.Fatalf("permissionsSkip() = %v, want %v", got, tc.want)
			}
		})
	}
}
