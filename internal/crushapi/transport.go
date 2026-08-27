// transport.go -- role: shared HTTP-over-local-socket client for Crush.
//
// Only the dial differs per OS (transport_windows.go, transport_unix.go);
// everything else about the client is identical and lives here once.
package crushapi

import (
	"context"
	"net"
	"net/http"
	"time"
)

// dialTimeout caps a single dial attempt. Cancellation is via ctx; this is only
// the upper bound Probe applies.
const dialTimeout = 2 * time.Second

// idleConnTimeout closes pooled connections nothing has used for a while, so an
// idle desktop holds no kernel handles open.
const idleConnTimeout = 90 * time.Second

// Dial returns an *http.Client that speaks HTTP over the local endpoint. The
// client has no global timeout: streams are long-lived and rely on the
// per-request context.
//
// Keep-alives are enabled. Redialing per request cost a fresh pipe or socket
// plus a read and a write goroutine for every REST call, and the startup health
// loop alone paid for roughly fifty of them. A two-connection idle pool removes
// that at negligible memory cost.
func Dial(ctx context.Context, ep Endpoint) (*http.Client, error) {
	if ep.Network != expectedNetwork {
		return nil, &dialError{ep: ep, msg: "expected " + expectedNetwork + " endpoint"}
	}
	tr := &http.Transport{
		DialContext: func(dialCtx context.Context, _, _ string) (net.Conn, error) {
			// Honor the caller's context: the stream subscriber's cancel, the
			// request deadline, and so on.
			return dialConn(dialCtx, ep.Address)
		},
		MaxIdleConns:        4,
		MaxIdleConnsPerHost: 2,
		IdleConnTimeout:     idleConnTimeout,
		// A local pipe or socket never negotiates TLS or HTTP/2, so neither a
		// handshake timeout nor the h2 upgrade attempt buys anything here.
		ForceAttemptHTTP2: false,
	}
	return &http.Client{Transport: tr}, nil
}
