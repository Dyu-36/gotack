package engine

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Dyu-36/gotack/internal/crushapi"
)

const pollInterval = 300 * time.Millisecond

var ErrEngineUnhealthy = errors.New("engine: not healthy within timeout")

func WaitForHealthy(ctx context.Context, api *crushapi.Client, timeout time.Duration) error {
	if api == nil {
		return errors.New("engine: nil client")
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	dctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

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
