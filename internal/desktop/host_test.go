package desktop

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/Dyu-36/gotack/internal/engine"
	"github.com/Dyu-36/gotack/internal/engineapi"
)

func TestWorkspaceInitializationMayReplaceHandshakeScope(t *testing.T) {
	h := New()
	t.Cleanup(h.Close)
	scope, _ := h.Link.BeginConnect(context.Background())
	ready := false
	var status EngineInfo
	h.Emit = func(_ string, value any) { status = value.(EngineInfo) }
	h.Hooks.Initialize = func(*Connection) string {
		h.Link.ReplaceStreamScope(context.Background())
		return "workspace warning"
	}
	h.Hooks.Ready = func() { ready = true }
	if err := h.attach(scope, engineapi.NewClient(nil), engineapi.Endpoint{}, "test"); err != nil {
		t.Fatal(err)
	}
	if scope.Err() == nil {
		t.Fatal("initialization did not replace the handshake scope")
	}
	if !ready || !status.Running || status.Error != "workspace warning" {
		t.Fatalf("ready=%v, status=%+v", ready, status)
	}
}

func TestSupersededAttachmentCannotPublishReady(t *testing.T) {
	for _, phase := range []string{"prepare", "initialize"} {
		t.Run(phase, func(t *testing.T) {
			h := New()
			t.Cleanup(h.Close)
			scope, _ := h.Link.BeginConnect(context.Background())
			ready := false
			h.Hooks.Ready = func() { ready = true }
			if phase == "prepare" {
				h.Hooks.Prepare = func(*Connection) { h.Disconnect() }
			} else {
				h.Hooks.Initialize = func(*Connection) string { h.Disconnect(); return "" }
			}
			err := h.attach(scope, engineapi.NewClient(nil), engineapi.Endpoint{}, "test")
			if !errors.Is(err, engine.ErrAttachSuperseded) || ready || h.Link.Status() != engine.StatusStopped {
				t.Fatalf("err=%v, ready=%v, status=%v", err, ready, h.Link.Status())
			}
		})
	}
}

func TestCloseRejectsLateAttachment(t *testing.T) {
	h := New()
	scope, _ := h.Link.BeginConnect(context.Background())
	h.Close()
	if h.CommitAttach(scope, &Connection{}, engineapi.Endpoint{}, "late") || h.Get() != nil {
		t.Fatal("a closed host accepted a late engine attachment")
	}
}

func TestDisconnectAndAttachCommitStayConsistent(t *testing.T) {
	for range 100 {
		h := New()
		scope, _ := h.Link.BeginConnect(context.Background())
		attached := &Connection{API: engineapi.NewClient(nil)}
		var workers sync.WaitGroup
		workers.Add(2)
		go func() { defer workers.Done(); h.CommitAttach(scope, attached, engineapi.Endpoint{}, "test") }()
		go func() { defer workers.Done(); h.Disconnect() }()
		workers.Wait()
		if h.Get().API != nil || h.Link.Status() != engine.StatusStopped {
			t.Fatal("disconnect left a partially attached connection")
		}
		h.Close()
	}
}
