// transport_windows.go -- role: dial the Crush named pipe on Windows.
//
// Only the dial is per-OS; the shared client config lives in transport.go.
package crushapi

import (
	"context"
	"net"

	"github.com/Microsoft/go-winio"
)

// expectedNetwork is the only Endpoint.Network this build accepts. There is
// deliberately no TCP fallback.
const expectedNetwork = "npipe"

// dialConn opens the named pipe at address. DialPipeContext honors ctx, so
// nothing has to race the dial against cancellation.
func dialConn(ctx context.Context, address string) (net.Conn, error) {
	return winio.DialPipeContext(ctx, address)
}
