package engine

import (
	"log/slog"
	"os/exec"
	"testing"

	"github.com/Dyu-36/gotack/internal/crushapi"
)

func quietSupervisor() *Supervisor {
	return NewSupervisor(slog.New(slog.DiscardHandler), "")
}

func markStartedForTest(s *Supervisor, ep crushapi.Endpoint) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cmd = &exec.Cmd{Path: "test-stub"}
	s.owned = true
	s.endpoint = ep
}

func TestEngineAPI_StartStop(t *testing.T) {
	var api EngineAPI = quietSupervisor()

	if api.Owned() {
		t.Fatalf("fresh supervisor must not be owned")
	}

	ep := crushapi.Endpoint{Network: "pipe", Address: "test-engine"}
	markStartedForTest(api.(*Supervisor), ep)

	if !api.Owned() {
		t.Fatalf("supervisor must report owned after a synthetic Start")
	}

	if err := api.Stop(); err != nil {
		t.Fatalf("Stop on a synthetic owned supervisor returned error: %v", err)
	}
}

func TestEngineAPI_AdoptedNotKilled(t *testing.T) {
	s := quietSupervisor()

	s.mu.Lock()
	s.cmd = &exec.Cmd{Path: "test-adopted"}
	s.owned = false
	s.endpoint = crushapi.Endpoint{Network: "pipe", Address: "adopted"}
	adoptedCmd := s.cmd
	s.mu.Unlock()

	var api EngineAPI = s
	if api.Owned() {
		t.Fatalf("adopted supervisor must report not owned")
	}

	if err := api.Stop(); err != nil {
		t.Fatalf("Stop on adopted supervisor returned error: %v", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd != adoptedCmd {
		t.Fatalf("Stop cleared the adopted cmd handle; adopted servers must not be touched")
	}
}

func TestEngineAPI_StopOnUnowned(t *testing.T) {
	s := quietSupervisor()
	var api EngineAPI = s

	if api.Owned() {
		t.Fatalf("a never-started supervisor must not be owned")
	}

	if err := api.Stop(); err != nil {
		t.Fatalf("Stop on a never-started supervisor returned error: %v", err)
	}
	if s.cmd != nil {
		t.Fatalf("Stop mutated cmd on a never-started supervisor")
	}
	if s.owned {
		t.Fatalf("Stop mutated owned flag on a never-started supervisor")
	}
}
