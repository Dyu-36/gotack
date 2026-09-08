package engineapi

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
