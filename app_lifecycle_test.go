package main

import (
	"context"
	"testing"

	"github.com/Dyu-36/gotack/internal/crushapi"
)

type lifecycleEngine struct {
	stopCalls int
}

func (*lifecycleEngine) Owned() bool { return true }

func (*lifecycleEngine) Locate(context.Context) (crushapi.Endpoint, bool) {
	return crushapi.Endpoint{}, false
}

func (*lifecycleEngine) Start() (crushapi.Endpoint, error) {
	return crushapi.Endpoint{}, nil
}

func (e *lifecycleEngine) Stop() error {
	e.stopCalls++
	return nil
}

func TestShutdownLeavesEngineRunning(t *testing.T) {
	app := NewApp()
	engine := &lifecycleEngine{}
	app.sup = engine

	streamCancelled := false
	app.swapConn(func(c *conn) *conn {
		c.cancelStream = func() { streamCancelled = true }
		return c
	})

	app.shutdown(context.Background())

	if !streamCancelled {
		t.Fatal("shutdown must disconnect the UI event stream")
	}
	if engine.stopCalls != 0 {
		t.Fatalf("shutdown stopped the warm engine %d time(s); only StopEngine may stop it", engine.stopCalls)
	}
}
