// transport_unix.go -- role: dial the Crush unix socket on linux and macOS.
//
//go:build unix

package crushapi

import (
	"context"
	"net"
)

// expectedNetwork is the only Endpoint.Network this build accepts.
const expectedNetwork = "unix"

// dialConn opens the unix socket at address.
func dialConn(ctx context.Context, address string) (net.Conn, error) {
	var d net.Dialer
	return d.DialContext(ctx, "unix", address)
}
