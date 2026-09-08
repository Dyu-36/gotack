package engineapi

import (
	"context"
	"net"
	"net/http"
	"time"
)

const dialTimeout = 2 * time.Second

const idleConnTimeout = 90 * time.Second

func Dial(ep Endpoint) (*http.Client, error) {
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

type dialError struct {
	ep  Endpoint
	err error
	msg string
}

func (e *dialError) Error() string {
	target := e.ep.Network + "://" + e.ep.Address
	if e.err != nil {
		return "engineapi: dial " + target + ": " + e.err.Error()
	}
	return "engineapi: dial " + target + ": " + e.msg
}

func (e *dialError) Unwrap() error { return e.err }

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
