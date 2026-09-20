package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/zalo"
)

func TestZaloStartRespectsPersistedEnabledFlag(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/getMe") {
			_, _ = w.Write([]byte(`{"ok":true,"result":{"id":"bot","name":"Test"}}`))
		} else {
			_, _ = w.Write([]byte(`{"ok":true,"result":[]}`))
		}
	}))
	defer server.Close()
	t.Setenv("ZALO_BOT_API_BASE", server.URL)
	app := NewApp()
	app.cfg = appconfig.Defaults()
	app.zalo = zalo.NewManager(filepath.Join(t.TempDir(), "zalo.json"), zalo.Runtime{}, nil)
	defer app.zalo.Stop()
	if _, err := app.zalo.SetToken(context.Background(), "test-token"); err != nil {
		t.Fatal(err)
	}
	app.startZaloIfEnabled()
	if app.zalo.Status().Running {
		t.Fatal("disabled channel auto-started")
	}
	app.cfg.Zalo.Enabled = true
	app.startZaloIfEnabled()
	if !app.zalo.Status().Running {
		t.Fatal("enabled channel did not start")
	}
	app.cfg.Zalo.Enabled = false
	app.startZaloIfEnabled()
	if app.zalo.Status().Running {
		t.Fatal("disabled channel remained running")
	}
	if _, err := app.zalo.TestConnection(context.Background()); err != nil {
		t.Fatal(err)
	}
	if app.zalo.Status().Running {
		t.Fatal("connection test enabled the disabled channel")
	}
}
