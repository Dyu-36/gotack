// transport_unix.go -- role: dial the Tack engine unix socket on Linux and macOS.
//
//go:build unix

package engineapi

import (
	"context"
	"net"
)

const expectedNetwork = "unix"

func dialConn(ctx context.Context, address string) (net.Conn, error) {
	var d net.Dialer
	return d.DialContext(ctx, "unix", address)
}
