package engine

import (
	"context"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/engineapi"
)

func (s *Supervisor) Locate(ctx context.Context) (engineapi.Endpoint, bool) {
	ep := appconfig.PipeEndpoint()
	if err := engineapi.Probe(ctx, ep); err != nil {
		s.log.Debug("engine: probe failed", "endpoint", ep, "err", err)
		return engineapi.Endpoint{}, false
	}

	s.mu.Lock()
	s.endpoint = ep
	s.mu.Unlock()
	return ep, true
}
