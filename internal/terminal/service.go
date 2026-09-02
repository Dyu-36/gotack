package terminal

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/Dyu-36/gotack/internal/uievents"
	"github.com/google/uuid"
)

var ErrUnknownID = errors.New("terminal: unknown session id")

type ptyBackend interface {
	Read(p []byte) (int, error)

	Write(p []byte) (int, error)

	Resize(cols, rows uint16) error

	Close() error

	Wait() (int32, error)
}

type shellSpec struct {
	commandLine string
	workDir     string
}

var openBackend func(cwd string) (ptyBackend, shellSpec, error)

type Service struct {
	log  *slog.Logger
	emit uievents.Emitter

	mu       sync.Mutex
	sessions map[string]*session
}

type session struct {
	id      string
	backend ptyBackend

	done chan struct{}
	once sync.Once
}

func New(log *slog.Logger, emit uievents.Emitter) *Service {
	if emit == nil {
		panic("terminal: nil emitter")
	}
	if log == nil {
		log = slog.Default()
	}
	return &Service{
		log:      log,
		emit:     emit,
		sessions: make(map[string]*session),
	}
}

func (s *Service) Open(cwd string) (string, error) {
	backend, spec, err := openBackend(cwd)
	if err != nil {
		return "", err
	}

	id := uuid.NewString()
	sess := &session{
		id:      id,
		backend: backend,
		done:    make(chan struct{}),
	}

	s.mu.Lock()
	s.sessions[id] = sess
	s.mu.Unlock()

	s.log.Info("terminal: opened session",
		slog.String("id", id),
		slog.String("shell", spec.commandLine),
		slog.String("cwd", spec.workDir))

	go s.pumpOutput(sess)
	go s.awaitExit(sess)

	return id, nil
}

const readChunk = 32 * 1024

func (s *Service) pumpOutput(sess *session) {
	buf := make([]byte, readChunk)
	for {
		n, err := sess.backend.Read(buf)
		if n > 0 {

			payload := string(buf[:n])
			s.emit(uievents.TerminalData, terminalDataPayload{ID: sess.id, Data: payload})
		}
		if err != nil || n == 0 {
			return
		}
	}
}

func (s *Service) awaitExit(sess *session) {
	code, err := sess.backend.Wait()
	if err != nil {
		s.log.Debug("terminal: wait returned error", slog.String("id", sess.id), slog.String("err", err.Error()))
	}

	sess.once.Do(func() {
		close(sess.done)
		s.emit(uievents.TerminalExit, terminalExitPayload{ID: sess.id, Code: code})
	})

	s.mu.Lock()
	if cur, ok := s.sessions[sess.id]; ok && cur == sess {
		delete(s.sessions, sess.id)
	}
	s.mu.Unlock()
	s.log.Info("terminal: closed session", slog.String("id", sess.id), slog.Int("code", int(code)))
}

func (s *Service) Write(id, data string) error {
	if data == "" {
		return nil
	}
	sess, err := s.lookup(id)
	if err != nil {
		return err
	}
	if _, err := sess.backend.Write([]byte(data)); err != nil {
		return fmt.Errorf("terminal: write: %w", err)
	}
	return nil
}

func (s *Service) Resize(id string, cols, rows uint16) error {
	sess, err := s.lookup(id)
	if err != nil {
		return err
	}
	if err := sess.backend.Resize(cols, rows); err != nil {
		return fmt.Errorf("terminal: resize: %w", err)
	}
	return nil
}

func (s *Service) Close(id string) error {
	s.mu.Lock()
	sess, ok := s.sessions[id]
	if !ok {
		s.mu.Unlock()
		return ErrUnknownID
	}
	delete(s.sessions, id)
	s.mu.Unlock()

	sess.once.Do(func() {

		close(sess.done)
	})

	_ = sess.backend.Close()
	s.log.Info("terminal: user closed session", slog.String("id", id))
	return nil
}

func (s *Service) lookup(id string) (*session, error) {
	s.mu.Lock()
	sess, ok := s.sessions[id]
	s.mu.Unlock()
	if !ok {
		return nil, ErrUnknownID
	}
	return sess, nil
}

type terminalDataPayload struct {
	ID   string `json:"id"`
	Data string `json:"data"`
}

type terminalExitPayload struct {
	ID   string `json:"id"`
	Code int32  `json:"code"`
}
