package main

import (
	"testing"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/zalo"
)

func TestSaveZaloConfigAllowsDisabledWithoutToken(t *testing.T) {
	t.Setenv("AppData", t.TempDir())
	manager := zalo.NewManager(t.TempDir()+"/zalo.json", zalo.Runtime{}, nil)
	app := &App{cfg: appconfig.Defaults(), zalo: manager}

	status, err := app.SaveZaloConfig(ZaloConfigUpdate{Enabled: false})
	if err != nil {
		t.Fatalf("SaveZaloConfig disabled: %v", err)
	}
	if status.Configured || status.Running {
		t.Fatalf("disabled empty config should remain disconnected: %+v", status)
	}
}

func TestSaveZaloConfigRequiresTokenWhenEnabled(t *testing.T) {
	t.Setenv("AppData", t.TempDir())
	manager := zalo.NewManager(t.TempDir()+"/zalo.json", zalo.Runtime{}, nil)
	app := &App{cfg: appconfig.Defaults(), zalo: manager}

	if _, err := app.SaveZaloConfig(ZaloConfigUpdate{Enabled: true}); err == nil {
		t.Fatal("enabling Zalo without a token must fail")
	}
}
