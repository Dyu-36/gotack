package enginelink

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/Dyu-36/gotack/internal/crushapi"
	"github.com/Dyu-36/gotack/internal/engine"
)

// Status is one step of the connection state machine:
// stopped -> starting -> running, with error reachable from starting and
// running, and stopped reachable from every state via Disconnect.
type Status string

const (
	StatusStopped  Status = "stopped"
	StatusStarting Status = "starting"
	StatusRunning  Status = "running"
	StatusError    Status = "error"
)

// defaultHandshakeTimeout bounds the version-poll handshake after dial.
const defaultHandshakeTimeout = 15 * time.Second

// ErrAttachSuperseded reports that the attach scope died while the connect
// sequence was running, so the attempt was abandoned without an error
// transition: whichever scope superseded it now owns the state machine.
var ErrAttachSuperseded = errors.New("enginelink: attach scope superseded")

// ErrNoSupervisor reports a Connect attempt before a supervisor was wired.
var ErrNoSupervisor = errors.New("enginelink: engine supervisor unavailable")

// DialFunc builds the HTTP transport for one endpoint. Production uses
// crushapi.Dial; tests inject a fake transport.
type DialFunc func(ctx context.Context, ep crushapi.Endpoint) (*http.Client, error)

// ReadyFunc is the host hook invoked after dial and handshake succeed. It
// owns building and committing the per-connection services; returning an
// error fails the connect, and returning ErrAttachSuperseded abandons it
// without an error transition.
type ReadyFunc func(ctx context.Context, api *crushapi.Client, ep crushapi.Endpoint, version string) error

// Link is the engine connection state machine. It serializes every status
// transition and owns the single live scope. Contexts are passed explicitly
// to every method; only their CancelFunc is retained, so each scope keeps
// exactly one owner.
type Link struct {
	sup              engine.EngineAPI
	dial             DialFunc
	handshakeTimeout time.Duration

	mu          sync.Mutex
	status      Status
	lastError   string
	ep          crushapi.Endpoint
	version     string
	scopeCancel context.CancelFunc
}

// NewLink returns a stopped Link driven by sup. sup may be nil until the
// host wires its supervisor; Connect reports ErrNoSupervisor until then.
func NewLink(sup engine.EngineAPI) *Link {
	return &Link{
		sup:              sup,
		dial:             crushapi.Dial,
		handshakeTimeout: defaultHandshakeTimeout,
		status:           StatusStopped,
	}
}

func (l *Link) Status() Status {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.status
}

func (l *Link) LastError() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.lastError
}

func (l *Link) Endpoint() crushapi.Endpoint {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.ep
}

func (l *Link) Version() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.version
}

// BeginConnect moves a stopped or failed link to starting and arms a fresh
// attach scope derived from parent, cancelling whichever scope was live
// before. It returns the new scope, or false when the link is already
// starting or running: that rejection is what keeps concurrent Start and
// Reconnect calls from launching duplicate attach attempts.
func (l *Link) BeginConnect(parent context.Context) (context.Context, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.status == StatusRunning || l.status == StatusStarting {
		return nil, false
	}
	if l.scopeCancel != nil {
		l.scopeCancel()
	}
	scope, cancel := context.WithCancel(parent)
	l.scopeCancel = cancel
	l.status = StatusStarting
	l.lastError = ""
	return scope, true
}

// Connect runs one attach sequence inside scope: locate-or-launch the engine,
// dial, handshake, then hand the live API to ready. It returns nil once
// ready returns nil, ready's error verbatim, or the wrapped failure of any
// earlier step. Status transitions around the attempt are the caller's: the
// host decides which failures become error transitions and which (a
// superseded scope) stay silent.
func (l *Link) Connect(scope context.Context, ready ReadyFunc) error {
	if l.sup == nil {
		return ErrNoSupervisor
	}
	ep, found := l.sup.Locate(scope)
	if !found {
		var err error
		ep, err = l.sup.Start()
		if err != nil {
			return fmt.Errorf("launch engine: %w", err)
		}
	}

	hc, err := l.dial(scope, ep)
	if err != nil {
		return fmt.Errorf("dial %s %s: %w", ep.Network, ep.Address, err)
	}

	api := crushapi.NewClient(hc)
	if err := engine.WaitForHealthy(scope, api, l.handshakeTimeout); err != nil {
		return fmt.Errorf("handshake: %w", err)
	}
	vi, err := api.Version(scope)
	if err != nil {
		return fmt.Errorf("version: %w", err)
	}

	return ready(scope, api, ep, vi.Version)
}

// CommitAttach records the endpoint and version of a successful handshake,
// but only while scope is still live. A Disconnect that ran while the
// sequence was dialing must not be clobbered by a connection tied to a dead
// scope; false tells the caller to abandon the attempt.
func (l *Link) CommitAttach(scope context.Context, ep crushapi.Endpoint, version string) bool {
	if scope.Err() != nil {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.ep = ep
	l.version = version
	l.lastError = ""
	return true
}

// MarkRunning promotes the link to running after the host finished wiring
// the connection (services committed, default workspace attached).
func (l *Link) MarkRunning() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.status = StatusRunning
	l.lastError = ""
}

// Fail moves the link to error and records the reason. Callers report the
// new state to the UI afterwards.
func (l *Link) Fail(reason string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.status = StatusError
	l.lastError = reason
}

// TransportLost marks a running link unhealthy, but only when scope is
// still live and the status is still running. Intentional Stop/Reconnect
// cancels the scope first and therefore never gets misreported as a loss;
// true means the transition happened and the caller should notify the UI.
func (l *Link) TransportLost(scope context.Context, reason string) bool {
	if scope.Err() != nil {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.status != StatusRunning {
		return false
	}
	l.status = StatusError
	l.lastError = reason
	return true
}

// Disconnect cancels the live scope, reports stopped, and drops the recorded
// endpoint and version. Per-connection services are owned by the host and
// cleared separately.
func (l *Link) Disconnect() {
	l.mu.Lock()
	cancel := l.scopeCancel
	l.scopeCancel = nil
	l.status = StatusStopped
	l.lastError = ""
	l.ep = crushapi.Endpoint{}
	l.version = ""
	l.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// ReplaceStreamScope cancels the previous scope and arms a fresh one derived
// from parent. Workspace activation and session-presence re-attach share it:
// there is exactly one live event-stream scope at any time.
func (l *Link) ReplaceStreamScope(parent context.Context) context.Context {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.scopeCancel != nil {
		l.scopeCancel()
	}
	scope, cancel := context.WithCancel(parent)
	l.scopeCancel = cancel
	return scope
}

// CancelScope cancels and forgets the live scope without touching the
// status. Used to roll back a freshly installed scope whose stream attach
// failed.
func (l *Link) CancelScope() {
	l.mu.Lock()
	cancel := l.scopeCancel
	l.scopeCancel = nil
	l.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}
