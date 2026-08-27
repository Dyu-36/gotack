package terminal

// CloseAll tears down every live terminal session. It is used by application
// shutdown so shells and ConPTY/pty handles cannot outlive Gotack. The map is
// detached under the lock, then backend I/O happens without holding it.
func (s *Service) CloseAll() {
	if s == nil {
		return
	}

	s.mu.Lock()
	live := make([]*session, 0, len(s.sessions))
	for _, sess := range s.sessions {
		live = append(live, sess)
	}
	s.sessions = make(map[string]*session)
	s.mu.Unlock()

	for _, sess := range live {
		sess.once.Do(func() { close(sess.done) })
		_ = sess.backend.Close()
	}
}
