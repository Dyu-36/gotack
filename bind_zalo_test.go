package main

import (
	"encoding/json"
	"strings"
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

func TestGetZaloConfigUsesEmptyPairedChatArray(t *testing.T) {
	manager := zalo.NewManager(t.TempDir()+"/zalo.json", zalo.Runtime{}, nil)
	app := &App{cfg: appconfig.Defaults(), zalo: manager}

	payload, err := json.Marshal(app.GetZaloConfig())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), `"paired_chats":[]`) {
		t.Fatalf("empty paired chats must serialize as an array: %s", payload)
	}
}
