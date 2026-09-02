package terminal

func openBackendForTest(fn func(cwd string) (ptyBackend, shellSpec, error)) func() {
	prev := openBackend
	openBackend = fn
	return func() { openBackend = prev }
}

func (s *Service) sessionBackend(id string) ptyBackend {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[id]
	if !ok {
		return nil
	}
	return sess.backend
}
