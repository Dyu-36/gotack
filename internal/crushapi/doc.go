// Package crushapi is the only package that speaks the Crush wire protocol.
//
// Transport: REST + SSE over a unix socket or a Windows named pipe. No public
// TCP port is opened.
//
// Important: gotack cannot import third_party/crush/internal/... because Go
// forbids internal packages across module boundaries. The wire contract is
// therefore re-declared in contract.go and versioned independently of the UI.
package crushapi
