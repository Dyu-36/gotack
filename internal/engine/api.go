package engine

import (
	"context"

	"github.com/Dyu-36/gotack/internal/crushapi"
)

// EngineAPI is the seam the desktop host uses to drive the Crush engine
// lifecycle. Defined here so app.go depends on a small interface, not the
// concrete *Supervisor. The interface is intentionally narrow: every method
// corresponds to a real call from the bind layer.
type EngineAPI interface {
	// Owned reports whether the supervised process was launched by us (true)
	// or adopted from an external start (false). Adopted servers are never
	// killed on shutdown.
	Owned() bool
	// Locate probes for an already-running engine on the canonical endpoint
	// and returns its descriptor when one is reachable. Adopted servers are
	// reused; only an absent or unreachable engine is a miss.
	Locate(ctx context.Context) (crushapi.Endpoint, bool)
	// Start launches the binary as a child process. Returns the endpoint the
	// new server is reachable on. If the binary is already running, returns
	// the existing endpoint.
	Start() (crushapi.Endpoint, error)
	// Stop terminates the child process tree when Owned() is true. Returns
	// nil on a no-op (adopted server or already stopped).
	Stop() error
}
