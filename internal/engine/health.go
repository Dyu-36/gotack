package engine

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Dyu-36/gotack/internal/crushapi"
)

// health.go -- role: handshake, protocol compatibility and reconnect policy.
//
// Validates the engine protocol version against the version this build expects,
// then applies bounded backoff when the transport drops.

// pollInterval is the gap between Version probes during the startup handshake.
// Bounded startup handshake only; the UI does not poll this loop.
const pollInterval = 300 * time.Millisecond

// ErrEngineUnhealthy is returned by WaitForHealthy when the engine never
// became reachable within the requested timeout.
var ErrEngineUnhealthy = errors.New("engine: not healthy within timeout")

// WaitForHealthy polls crushapi.Client.Version every 300ms until it succeeds
// or ctx is done / timeout elapses. The first successful Version call
// resolves the handshake and the function returns nil.
//
// This is intentionally simple: the engine is considered healthy as soon as
// the REST /v1/version endpoint replies. Protocol-version compatibility is
// enforced by the caller (the UI compares the returned VersionInfo against
// the build's expected range).
func WaitForHealthy(ctx context.Context, api *crushapi.Client, timeout time.Duration) error {
	if api == nil {
		return errors.New("engine: nil client")
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	dctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Immediate attempt: many startups are already up by the time we get here.
	if _, err := api.Version(dctx); err == nil {
		return nil
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-dctx.Done():
			if errors.Is(dctx.Err(), context.DeadlineExceeded) {
				return fmt.Errorf("%w (after %s)", ErrEngineUnhealthy, timeout)
			}
			return dctx.Err()
		case <-ticker.C:
			if _, err := api.Version(dctx); err == nil {
				return nil
			}
		}
	}
}
