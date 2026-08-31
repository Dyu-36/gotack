// Package enginelink owns the engine connection state machine: status
// transitions, the single live attach/stream scope, the dial and handshake
// sequence, and the workspace SSE subscription.
//
// Contexts are explicit parameters on every method and are never stored in a
// struct. The only context-derived value kept on Link is the CancelFunc of
// the current scope, so each scope has exactly one owner and cancellation is
// unambiguous.
//
// Retry model: the host never retries on its own timer. A failed connect or a
// lost transport moves the link to StatusError, which surfaces through the
// engine:status event; the frontend applies bounded exponential backoff
// (750ms doubling, capped at 30s) and then calls back into the host, which
// routes through Stop-then-BeginConnect. Link's job in that cycle is to make
// every retry safe: BeginConnect rejects a link that is already starting or
// running, a superseded scope can never clobber the current one
// (CommitAttach), and an intentional disconnect is never misreported as a
// transport loss (TransportLost checks the scope before the status).
//
// Process supervision stays in internal/engine; HTTP shapes stay in
// internal/crushapi. This package wires the two into one lifecycle.
package enginelink
