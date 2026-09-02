// Package terminal exposes a lazy PTY session service.
//
// Nothing here runs at construction time; sessions are spawned on demand from
// the UI. Each session owns one OS pty (ConPTY on Windows, pty(7) on Unix) and
// two background goroutines: a reader that streams chunks to the UI as
// uievents.TerminalData, and a waiter that emits uievents.TerminalExit and
// removes the session from the map once the underlying shell exits.
package terminal

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/Dyu-36/gotack/internal/uievents"
	"github.com/google/uuid"
)

// ErrUnknownID is returned by Write, Resize and Close when the session id is
// not (or no longer) registered with the service.
var ErrUnknownID = errors.New("terminal: unknown session id")

// ptyBackend abstracts the OS pty so service.go can stay cross-platform.
type ptyBackend interface {
	// Read blocks until at least one byte is available, the pty closes, or an
	// error occurs. n == 0 with err == nil is treated as EOF.
	Read(p []byte) (int, error)
	// Write hands the bytes to the pty master. A short write is allowed.
	Write(p []byte) (int, error)
	// Resize changes the visible window of the pty.
	Resize(cols, rows uint16) error
	// Close releases handles and terminates the child if it is still alive.
	// It is safe to call multiple times; subsequent calls return nil.
	Close() error
	// Wait blocks until the child exits and returns its exit code. It must be
	// safe to call from a single goroutine only; the service uses it from the
	// dedicated waiter goroutine.
	Wait() (int32, error)
}

// shellSpec describes the command line to spawn for a new session. It is
// computed once per Open call by the platform-specific openBackend helper.
type shellSpec struct {
	commandLine string
	workDir     string
}

// openBackend spawns a new pty-backed child for cwd. It is a variable rather
// than a function so the test build can substitute a fake backend without
// depending on a real shell. The platform-specific files assign it once at
// init time.
var openBackend func(cwd string) (ptyBackend, shellSpec, error)

// Service owns live PTY sessions. The zero value is unusable; construct via
// New. The service is safe for concurrent use.
type Service struct {
	log  *slog.Logger
	emit uievents.Emitter

	mu       sync.Mutex
	sessions map[string]*session
}

type session struct {
	id      string
	backend ptyBackend
	// done is closed exactly once, by either the waiter goroutine (on child
	// exit, after emitting TerminalExit) or by Close (on user request, no
	// exit event). Callers synchronise on this channel to know the session
	// is fully torn down.
	done chan struct{}
	once sync.Once // guards close of done + final exit code assignment
}

// New returns a ready-to-use service. It performs no I/O; sessions are
// spawned lazily by Open. emit must be non-nil; the service panics on a nil
// emitter because that would be a programming error, not a runtime condition.
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

// Open validates cwd, spawns a new PTY-backed shell and returns the session
// id. The id is a fresh UUIDv4 string. Two background goroutines are started
// per session: a reader and a waiter. The reader streams chunks as
// uievents.TerminalData events; the waiter emits a single uievents.TerminalExit
// when the child exits and removes the session from the map.
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

// readChunk balances IPC overhead against terminal responsiveness. It bounds
// each read, not event rate; terminal output intentionally emits one event per
// successful PTY read.
const readChunk = 32 * 1024

// pumpOutput reads from the pty and forwards each chunk as a TerminalData
// event. The loop ends when Read returns an error or a zero-byte read (EOF);
// the waiter goroutine is responsible for final cleanup.
func (s *Service) pumpOutput(sess *session) {
	buf := make([]byte, readChunk)
	for {
		n, err := sess.backend.Read(buf)
		if n > 0 {
			// Allocate a fresh string per chunk: the emitter copies into the
			// Wails event bus asynchronously, so reusing the buffer would race
			// with the next Read.
			payload := string(buf[:n])
			s.emit(uievents.TerminalData, terminalDataPayload{ID: sess.id, Data: payload})
		}
		if err != nil || n == 0 {
			return
		}
	}
}

// awaitExit waits for the child process to exit, then emits TerminalExit and
// removes the session from the map exactly once. The exit code is whatever
// the platform backend reports: 0 for a clean exit, a non-zero value for a
// shell error, and -1 for "exit by signal" (no signal number is exposed by
// the Go runtime). Windows additionally normalises STILL_ACTIVE to -1.
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

// Write forwards raw bytes to the session's pty. Empty data is a no-op. The
// unknown-id case is checked under the map lock; the actual Write call is
// made outside the lock to avoid serialising unrelated sessions.
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

// Resize changes the pty window for id. The unknown-id case returns
// ErrUnknownID; an already-exited session still returns ErrUnknownID because
// the waiter removes the entry before this lookup could observe it.
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

// Close terminates the session and removes it from the map. It returns
// ErrUnknownID if id is not currently tracked. Calling Close on a session
// whose child has already exited (and was therefore removed from the map
// by the waiter goroutine) is the unknown-id path: it returns ErrUnknownID
// and does not panic, satisfying the spec requirement that
// "Close on already-exited must not panic".
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
		// The waiter hasn't fired yet, so the channel is still open. Close it
		// here so any code blocked on the session can proceed. We do not emit
		// TerminalExit because the waiter is the canonical source for that
		// event; the closing path is an explicit user action, not a child
		// exit.
		close(sess.done)
	})

	// Best-effort: the backend Close terminates the child if alive. Errors
	// here are normal when the process already exited on its own.
	_ = sess.backend.Close()
	s.log.Info("terminal: user closed session", slog.String("id", id))
	return nil
}

// lookup returns the live session for id, or ErrUnknownID. Callers must not
// hold s.mu when performing I/O on the returned session.
func (s *Service) lookup(id string) (*session, error) {
	s.mu.Lock()
	sess, ok := s.sessions[id]
	s.mu.Unlock()
	if !ok {
		return nil, ErrUnknownID
	}
	return sess, nil
}

// terminalDataPayload is the JSON shape of a uievents.TerminalData event. The
// UI reads it as {id, data:string}.
type terminalDataPayload struct {
	ID   string `json:"id"`
	Data string `json:"data"`
}

// terminalExitPayload is the JSON shape of a uievents.TerminalExit event. The
// UI reads it as {id, code:int32}.
type terminalExitPayload struct {
	ID   string `json:"id"`
	Code int32  `json:"code"`
}
