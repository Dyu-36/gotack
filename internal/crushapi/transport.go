package crushapi

import (
	"context"
	"net"
	"net/http"
	"time"
)

const dialTimeout = 2 * time.Second

const idleConnTimeout = 90 * time.Second

func Dial(ctx context.Context, ep Endpoint) (*http.Client, error) {
	if ep.Network != expectedNetwork {
		return nil, &dialError{ep: ep, msg: "expected " + expectedNetwork + " endpoint"}
	}
	tr := &http.Transport{
		DialContext: func(dialCtx context.Context, _, _ string) (net.Conn, error) {

			return dialConn(dialCtx, ep.Address)
		},
		MaxIdleConns:        4,
		MaxIdleConnsPerHost: 2,
		IdleConnTimeout:     idleConnTimeout,

		ForceAttemptHTTP2: false,
	}
	return &http.Client{Transport: tr}, nil
}
