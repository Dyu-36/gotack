package terminal

// This file exposes internal helpers to the test package only. It must not be
// imported by anything outside the test build.

// openBackendForTest installs a fake backend factory and returns a restore
// function. Tests use it to inject a deterministic ptyBackend so they can
// exercise the session lifecycle without spawning a real shell.
func openBackendForTest(fn func(cwd string) (ptyBackend, shellSpec, error)) func() {
	prev := openBackend
	openBackend = fn
	return func() { openBackend = prev }
}

// sessionBackend returns the backend of a live session so tests can drive
// Read/Close/Resize directly. Returns nil if id is not tracked.
func (s *Service) sessionBackend(id string) ptyBackend {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[id]
	if !ok {
		return nil
	}
	return sess.backend
}
