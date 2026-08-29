package engine

import (
	"log/slog"
	"os/exec"
	"testing"

	"github.com/Dyu-36/gotack/internal/crushapi"
)

// quietSupervisor returns a Supervisor with a discard logger and no
// resolved binary path. Used as the starting point for tests that need to
// drive the lifecycle through field injection rather than a real Start.
func quietSupervisor() *Supervisor {
	return NewSupervisor(slog.New(slog.DiscardHandler), "")
}

// markStartedForTest sets the post-Start fields on a Supervisor without
// actually launching a process. The synthesized *exec.Cmd has a nil Process,
// so Stop() short-circuits on the "cmd.Process == nil" guard and returns nil
// without ever issuing a real signal. This keeps the test deterministic and
// free of any real child process.
func markStartedForTest(s *Supervisor, ep crushapi.Endpoint) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cmd = &exec.Cmd{Path: "test-stub"}
	s.owned = true
	s.endpoint = ep
}

// TestEngineAPI_StartStop proves the *Supervisor satisfies EngineAPI via
// Start/Stop: after a synthetic post-Start state, Owned() reports true and
// Stop() returns nil without panicking.
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

// TestEngineAPI_AdoptedNotKilled proves the adopted path: an unowned
// supervisor must short-circuit Stop() without clearing cmd. This mirrors
// the production guard `if cmd == nil || cmd.Process == nil || !s.owned { ... }`
// where a server we did not launch is left alone on shutdown.
func TestEngineAPI_AdoptedNotKilled(t *testing.T) {
	s := quietSupervisor()

	// Simulate adoption: a running cmd was discovered by Locate, not
	// launched by us, so owned stays false. Process is nil here so any
	// accidental branch that bypassed the owned check would still no-op,
	// but the real guard is `!s.owned`.
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

	// Critical: the adopted cmd pointer must be untouched. If Stop had
	// cleared it, that would mean we nulled out someone else's process
	// handle, breaking the no-kill-adopted contract.
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd != adoptedCmd {
		t.Fatalf("Stop cleared the adopted cmd handle; adopted servers must not be touched")
	}
}

// TestEngineAPI_StopOnUnowned proves the never-started case: calling Stop on
// a Supervisor whose Start was never invoked must be a no-op (nil, no
// panic, state preserved).
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
