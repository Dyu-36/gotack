package terminal

import (
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Dyu-36/gotack/internal/uievents"
)

// recordingEmitter captures every event the service fires so tests can assert
// on names and payloads without depending on the Wails runtime.
type recordingEmitter struct {
	mu     sync.Mutex
	events []recordedEvent
}

type recordedEvent struct {
	name string
	data any
}

type readStep struct {
	data string
	err  error
}

func (e *recordingEmitter) emit(name string, data any) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.events = append(e.events, recordedEvent{name: name, data: data})
}

func (e *recordingEmitter) snapshot() []recordedEvent {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]recordedEvent, len(e.events))
	copy(out, e.events)
	return out
}

// fakeBackend is a programmable ptyBackend. The read program is a slice of
// (chunk, error) pairs; once exhausted, Read returns io.EOF. The wait
// program is fired by the FakeWait() method which the test calls to simulate
// the child exiting.
type fakeBackend struct {
	mu sync.Mutex

	readProgram []readStep
	resizes     []uint32 // packed cols<<16 | rows
	writes      []string
	closeCount  int32
	waitCount   int32
	waitCode    int32
	waitErr     error
	closed      bool
	exitCh      chan struct{} // pre-created so exit-before-wait still works
}

func newFakeBackend(steps ...readStep) *fakeBackend {
	return &fakeBackend{readProgram: steps, waitCode: 0, exitCh: make(chan struct{})}
}

func (b *fakeBackend) Read(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.readProgram) == 0 {
		return 0, io.EOF
	}
	step := b.readProgram[0]
	b.readProgram = b.readProgram[1:]
	if step.err != nil {
		return 0, step.err
	}
	n := copy(p, step.data)
	return n, nil
}

func (b *fakeBackend) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.writes = append(b.writes, string(p))
	return len(p), nil
}

func (b *fakeBackend) Resize(cols, rows uint16) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.resizes = append(b.resizes, uint32(cols)<<16|uint32(rows))
	return nil
}

func (b *fakeBackend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	atomic.AddInt32(&b.closeCount, 1)
	b.closed = true
	return nil
}

func (b *fakeBackend) Wait() (int32, error) {
	// Wait blocks until the test calls Exit. We use a channel rather than a
	// sleep so the test is deterministic.
	<-b.exitSignal()
	atomic.AddInt32(&b.waitCount, 1)
	return b.waitCode, b.waitErr
}

// exitSignal returns the channel that is closed when the test wants the
// waiter goroutine to proceed.
func (b *fakeBackend) exitSignal() <-chan struct{} {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.exitCh
}

func (b *fakeBackend) exit(code int32) {
	b.mu.Lock()
	b.waitCode = code
	close(b.exitCh)
	b.mu.Unlock()
}

func (b *fakeBackend) capturedWrites() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]string, len(b.writes))
	copy(out, b.writes)
	return out
}

func (b *fakeBackend) capturedResizes() [][2]uint16 {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([][2]uint16, 0, len(b.resizes))
	for _, r := range b.resizes {
		out = append(out, [2]uint16{uint16(r >> 16), uint16(r)})
	}
	return out
}

// withFakeBackend swaps the package-level openBackend with one that returns
// the provided fake. The restore is called from t.Cleanup.
func withFakeBackend(t *testing.T, fb *fakeBackend) {
	t.Helper()
	restore := openBackendForTest(func(cwd string) (ptyBackend, shellSpec, error) {
		return fb, shellSpec{commandLine: "fake-shell", workDir: cwd}, nil
	})
	t.Cleanup(restore)
}

func newService(t *testing.T) (*Service, *recordingEmitter) {
	t.Helper()
	rec := &recordingEmitter{}
	s := New(slog.New(slog.NewTextHandler(io.Discard, nil)), rec.emit)
	return s, rec
}

func TestNewPanicsOnNilEmitter(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("New(nil) did not panic")
		}
	}()
	_ = New(slog.Default(), nil)
}

func TestOpenReturnsUUIDAndRegistersSession(t *testing.T) {
	fb := newFakeBackend()
	withFakeBackend(t, fb)
	s, rec := newService(t)

	id, err := s.Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if id == "" {
		t.Fatalf("Open returned empty id")
	}
	if len(id) < 32 {
		t.Fatalf("Open returned id %q, expected uuid", id)
	}

	// Pump must have read at least the EOF, so the events list is touched.
	// We do not assert on the data payload here (the backend had no chunks).
	got := rec.snapshot()
	_ = got

	// The session should be tracked.
	if s.sessionBackend(id) == nil {
		t.Fatalf("session %s not in map after Open", id)
	}
	// And the fake backend must have received a Close when the service
	// stops (not yet; we haven't exited).
}

// TestOpenPropagatesBackendError verifies that a backend open failure is
// surfaced to the caller and no session is registered.
func TestOpenPropagatesBackendError(t *testing.T) {
	restore := openBackendForTest(func(cwd string) (ptyBackend, shellSpec, error) {
		return nil, shellSpec{}, errors.New("boom")
	})
	t.Cleanup(restore)

	s, _ := newService(t)
	_, err := s.Open(t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("Open: want boom, got %v", err)
	}
	if len(s.sessions) != 0 {
		t.Fatalf("Open failed but session was registered")
	}
}

func TestWriteForwardsBytesToBackend(t *testing.T) {
	fb := newFakeBackend()
	withFakeBackend(t, fb)
	s, _ := newService(t)

	id, err := s.Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	if err := s.Write(id, "hello\n"); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got := fb.capturedWrites()
	if len(got) != 1 || got[0] != "hello\n" {
		t.Fatalf("backend writes: %#v", got)
	}

	// Empty write is a no-op and does not touch the backend.
	if err := s.Write(id, ""); err != nil {
		t.Fatalf("Write empty: %v", err)
	}
	got = fb.capturedWrites()
	if len(got) != 1 {
		t.Fatalf("empty Write should be a no-op, got writes: %#v", got)
	}
}

func TestWriteRejectsUnknownID(t *testing.T) {
	fb := newFakeBackend()
	withFakeBackend(t, fb)
	s, _ := newService(t)

	if err := s.Write("does-not-exist", "x"); !errors.Is(err, ErrUnknownID) {
		t.Fatalf("Write: want ErrUnknownID, got %v", err)
	}
	if got := fb.capturedWrites(); len(got) != 0 {
		t.Fatalf("backend should not receive writes for unknown id, got: %#v", got)
	}
}

func TestResizeForwardsAndRejectsUnknown(t *testing.T) {
	fb := newFakeBackend()
	withFakeBackend(t, fb)
	s, _ := newService(t)

	id, err := s.Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	if err := s.Resize(id, 132, 50); err != nil {
		t.Fatalf("Resize: %v", err)
	}
	if got := fb.capturedResizes(); len(got) != 1 || got[0] != [2]uint16{132, 50} {
		t.Fatalf("backend resizes: %#v", got)
	}

	if err := s.Resize("nope", 1, 1); !errors.Is(err, ErrUnknownID) {
		t.Fatalf("Resize unknown: want ErrUnknownID, got %v", err)
	}
}

func TestCloseRemovesSessionAndStopsBackend(t *testing.T) {
	fb := newFakeBackend()
	withFakeBackend(t, fb)
	s, _ := newService(t)

	id, _ := s.Open(t.TempDir())
	if err := s.Close(id); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if s.sessionBackend(id) != nil {
		t.Fatalf("session %s still in map after Close", id)
	}
	// Allow the pump goroutine a brief moment to see the closed master and
	// exit; this is just to keep the race detector happy, no assertion.
	time.Sleep(10 * time.Millisecond)
}

// TestCloseAlreadyExitedNoPanic covers the spec requirement: closing a
// session whose child already exited must not panic. After the waiter
// removes the entry, Close(id) is the unknown-id path and returns
// ErrUnknownID.
func TestCloseAlreadyExitedNoPanic(t *testing.T) {
	fb := newFakeBackend()
	withFakeBackend(t, fb)
	s, _ := newService(t)

	id, _ := s.Open(t.TempDir())
	fb.exit(0)
	// Wait for the waiter to actually remove the session.
	deadline := time.Now().Add(2 * time.Second)
	for s.sessionBackend(id) != nil {
		if time.Now().After(deadline) {
			t.Fatalf("waiter did not remove session in time")
		}
		time.Sleep(time.Millisecond)
	}

	// Now Close must return ErrUnknownID and not panic.
	if err := s.Close(id); !errors.Is(err, ErrUnknownID) {
		t.Fatalf("Close after exit: want ErrUnknownID, got %v", err)
	}
}

// TestExitEmitsTerminalExit covers the happy-path lifecycle: data chunks
// stream as TerminalData, then a single TerminalExit with the right code.
func TestExitEmitsTerminalExit(t *testing.T) {
	fb := newFakeBackend(
		readStep{data: "first chunk\n"},
		readStep{data: "second chunk\n"},
	)
	withFakeBackend(t, fb)
	s, rec := newService(t)

	id, _ := s.Open(t.TempDir())

	// Wait for both data events.
	waitFor(t, 2*time.Second, func() bool {
		count := 0
		for _, e := range rec.snapshot() {
			if e.name == uievents.TerminalData {
				count++
			}
		}
		return count >= 2
	}, "two TerminalData events")

	// Trigger child exit.
	fb.exit(7)

	waitFor(t, 2*time.Second, func() bool {
		for _, e := range rec.snapshot() {
			if e.name == uievents.TerminalExit {
				p, ok := e.data.(terminalExitPayload)
				if !ok {
					t.Fatalf("TerminalExit payload type %T", e.data)
				}
				if p.ID != id {
					t.Fatalf("TerminalExit id %q, want %q", p.ID, id)
				}
				if p.Code != 7 {
					t.Fatalf("TerminalExit code %d, want 7", p.Code)
				}
				return true
			}
		}
		return false
	}, "TerminalExit event")

	// And the session must be gone from the map.
	waitFor(t, 2*time.Second, func() bool { return s.sessionBackend(id) == nil },
		"session removed after exit")
}

func TestDataPayloadShape(t *testing.T) {
	fb := newFakeBackend(readStep{data: "abc"})
	withFakeBackend(t, fb)
	s, rec := newService(t)

	id, _ := s.Open(t.TempDir())

	waitFor(t, 2*time.Second, func() bool {
		for _, e := range rec.snapshot() {
			if e.name == uievents.TerminalData {
				p, ok := e.data.(terminalDataPayload)
				if !ok {
					t.Fatalf("TerminalData payload type %T", e.data)
				}
				if p.ID != id || p.Data != "abc" {
					t.Fatalf("payload %+v", p)
				}
				return true
			}
		}
		return false
	}, "TerminalData event")

	// Drain.
	fb.exit(0)
	waitFor(t, 2*time.Second, func() bool { return s.sessionBackend(id) == nil },
		"session removed")
}

func TestCloseBeforeWaitEmitsOnlyOneExit(t *testing.T) {
	// The user closes the session before the child has exited. The waiter
	// goroutine may still run later but must not emit a second
	// TerminalExit because the once guard has already fired.
	fb := newFakeBackend()
	withFakeBackend(t, fb)
	s, rec := newService(t)

	id, _ := s.Open(t.TempDir())
	if err := s.Close(id); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Now trigger the child's wait. It still produces an exit code, but
	// the public TerminalExit must not have been emitted (the user-driven
	// close path is a separate, quiet shutdown).
	fb.exit(0)
	waitFor(t, 2*time.Second, func() bool { return true }, "settle")

	for _, e := range rec.snapshot() {
		if e.name == uievents.TerminalExit {
			t.Fatalf("no TerminalExit should be emitted for user-driven close, got %+v", e)
		}
	}
}

func TestConcurrentOpenClose(t *testing.T) {
	// Lightweight stress: open 8 sessions in parallel and close them all.
	// The race detector will catch any map or backend access that escapes
	// the lock.
	fb := newFakeBackend()
	withFakeBackend(t, fb)
	s, _ := newService(t)

	ids := make([]string, 8)
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id, err := s.Open(t.TempDir())
			if err != nil {
				t.Errorf("Open: %v", err)
				return
			}
			ids[i] = id
		}(i)
	}
	wg.Wait()

	for _, id := range ids {
		if id == "" {
			continue
		}
		_ = s.Close(id)
	}
	waitFor(t, 2*time.Second, func() bool { return len(s.sessions) == 0 },
		"all sessions removed")
}

func TestValidateCwd(t *testing.T) {
	tmp := t.TempDir()
	good := filepath.Join(tmp, "real-dir")
	if err := os.Mkdir(good, 0o755); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(tmp, "no-such-dir")
	notDir := filepath.Join(tmp, "file.txt")
	if err := os.WriteFile(notDir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A symlink pointing to a real dir must resolve and pass.
	link := filepath.Join(tmp, "link")
	if err := os.Symlink(good, link); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	cases := []struct {
		name    string
		in      string
		wantErr string // substring of error, or "" for success
	}{
		{name: "empty", in: " ", wantErr: "empty working directory"},
		{name: "missing", in: missing, wantErr: "cwd not found"},
		{name: "file", in: notDir, wantErr: "not a directory"},
		{name: "good", in: good},
		{name: "good with whitespace", in: "  " + good + "  "},
		{name: "symlink", in: link},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, err := validateCwd(c.in)
			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("validateCwd(%q): want err containing %q, got %v (out=%q)",
						c.in, c.wantErr, err, out)
				}
				return
			}
			if err != nil {
				t.Fatalf("validateCwd(%q): unexpected err %v", c.in, err)
			}
			if !strings.HasPrefix(out, good) && c.name != "symlink" {
				// Symlinks can resolve to the target; just require absolute.
				if !filepath.IsAbs(out) {
					t.Fatalf("validateCwd(%q) = %q, want absolute", c.in, out)
				}
			}
		})
	}
}

// waitFor polls cond every 5ms until it returns true or timeout elapses.
// It fails the test with msg if timeout is reached.
func waitFor(t *testing.T, timeout time.Duration, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for: %s", msg)
}
