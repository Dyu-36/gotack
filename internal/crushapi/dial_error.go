// dial_error.go -- role: shared error type for transport dials.
//
// dialError is returned by Dial on every platform; defining it in a
// non-build-tagged file keeps the unix and windows transports from
// having to redeclare the same type.
package crushapi

// dialError tags a failed Dial with the endpoint for the caller's logs.
// It is returned by both transport_windows.go and transport_unix.go.
type dialError struct {
	ep  Endpoint
	err error
	msg string
}

func (e *dialError) Error() string {
	target := e.ep.Network + "://" + e.ep.Address
	if e.err != nil {
		return "crushapi: dial " + target + ": " + e.err.Error()
	}
	return "crushapi: dial " + target + ": " + e.msg
}

func (e *dialError) Unwrap() error { return e.err }
