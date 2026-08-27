// probe.go -- role: reachability check for a Crush endpoint.
//
// Adoption path: the desktop probes the expected socket or pipe before it
// launches anything, so it can attach to an engine that outlived the UI.
package crushapi

import "context"

// Probe dials the endpoint and closes the connection immediately. A nil error
// means a listener is reachable.
//
// This is the canonical reachability check. internal/engine used to carry its
// own per-OS copy of this dial, which drifted: the Windows copy called the
// context-less winio.DialPipe and leaked a goroutine whenever ctx won the race.
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
