package terminal

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
