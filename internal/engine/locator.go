package engine

import (
	"context"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/crushapi"
)

// locator.go -- role: find an already reachable Crush endpoint.
//
// Probes the expected socket or named pipe before anything is launched, so the
// desktop can attach to an engine that outlived the previous UI session.

// Locate probes the default engine endpoint. It returns (endpoint, true) if a
// listener is reachable, otherwise (zero, false). The supervisor's owned state
// is not affected: a successful probe is an adoption, not a launch.
//
// The dial itself lives in internal/crushapi, which already owns the per-OS
// transport. Keeping a second copy here is what let the two drift apart.
func (s *Supervisor) Locate(ctx context.Context) (crushapi.Endpoint, bool) {
	ep := appconfig.PipeEndpoint()
	if err := crushapi.Probe(ctx, ep); err != nil {
		s.log.Debug("engine: probe failed", "endpoint", ep, "err", err)
		return crushapi.Endpoint{}, false
	}
	// Only the endpoint is recorded. The UI-visible status is owned by App,
	// which is what actually emits engine:status.
	s.mu.Lock()
	s.endpoint = ep
	s.mu.Unlock()
	return ep, true
}
