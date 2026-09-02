package crushapi

import "context"

// Probe dials the endpoint and closes the connection immediately. A nil error
// means a listener is reachable.
//
// This is the canonical reachability check used before launch so the desktop
// can adopt an engine that outlived the UI. The platform dial must honor ctx.
func Probe(ctx context.Context, ep Endpoint) error {
	if ep.Network == "" || ep.Address == "" {
		return &dialError{ep: ep, msg: "empty endpoint"}
	}
	if ep.Network != expectedNetwork {
		return &dialError{ep: ep, msg: "expected " + expectedNetwork + " endpoint"}
	}
	dctx, cancel := context.WithTimeout(ctx, dialTimeout)
	defer cancel()
	conn, err := dialConn(dctx, ep.Address)
	if err != nil {
		return err
	}
	return conn.Close()
}
