package main

import (
	"context"
	"testing"
)

type lifecycleEngine struct {
	stopCalls int
	owned     bool
}

func (e *lifecycleEngine) Owned() bool { return e.owned }
func (e *lifecycleEngine) Stop() error { e.stopCalls++; return nil }

func TestShutdownStopsOnlyOwnedEngineOnce(t *testing.T) {
	for _, owned := range []bool{false, true} {
		app := NewApp()
		sup := &lifecycleEngine{owned: owned}
		app.sup = sup
		scope := app.link.ReplaceStreamScope(context.Background())
		app.shutdown(context.Background())
		app.shutdown(context.Background())
		if scope.Err() == nil {
			t.Fatal("event stream was not cancelled")
		}
		expected := 0
		if owned {
			expected = 1
		}
		if sup.stopCalls != expected {
			t.Fatalf("owned=%v: stop calls=%d", owned, sup.stopCalls)
		}
		if app.getConn() != nil {
			t.Fatal("closed desktop retains a live connection")
		}
	}
}

func TestWindowCloseAndExplicitQuit(t *testing.T) {
	if !shouldHideOnClose("windows", false) {
		t.Fatal("normal Windows close should hide to tray")
	}
	for _, platform := range []string{"windows", "linux", "darwin"} {
		if shouldHideOnClose(platform, true) {
			t.Fatal("explicit quit must not hide")
		}
	}
	if shouldHideOnClose("linux", false) || shouldHideOnClose("darwin", false) {
		t.Fatal("no-tray platforms must close normally")
	}
}
