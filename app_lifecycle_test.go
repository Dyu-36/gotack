package main

import (
	"context"
	"testing"
)

type lifecycleEngine struct {
	stopCalls int
}

func (*lifecycleEngine) Owned() bool { return true }

func (e *lifecycleEngine) Stop() error {
	e.stopCalls++
	return nil
}

func TestShutdownLeavesEngineRunning(t *testing.T) {
	app := NewApp()
	sup := &lifecycleEngine{}
	app.sup = sup

	scope := app.link.ReplaceStreamScope(context.Background())

	app.shutdown(context.Background())

	if scope.Err() == nil {
		t.Fatal("shutdown must disconnect the UI event stream")
	}
	if sup.stopCalls != 0 {
		t.Fatalf("shutdown stopped the warm engine %d time(s); only StopEngine may stop it", sup.stopCalls)
	}
}
