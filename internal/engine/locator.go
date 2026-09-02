package engine

import (
	"context"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/crushapi"
)

func (s *Supervisor) Locate(ctx context.Context) (crushapi.Endpoint, bool) {
	ep := appconfig.PipeEndpoint()
	if err := crushapi.Probe(ctx, ep); err != nil {
		s.log.Debug("engine: probe failed", "endpoint", ep, "err", err)
		return crushapi.Endpoint{}, false
	}

	s.mu.Lock()
	s.endpoint = ep
	s.mu.Unlock()
	return ep, true
}
