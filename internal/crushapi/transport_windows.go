package crushapi

import (
	"context"
	"net"

	"github.com/Microsoft/go-winio"
)

const expectedNetwork = "npipe"

func dialConn(ctx context.Context, address string) (net.Conn, error) {
	return winio.DialPipeContext(ctx, address)
}
