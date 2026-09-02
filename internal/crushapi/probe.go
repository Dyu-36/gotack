package crushapi

import "context"

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
