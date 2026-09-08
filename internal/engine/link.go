package engine

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/Dyu-36/gotack/internal/engineapi"
)

type Status string

const (
	StatusStopped  Status = "stopped"
	StatusStarting Status = "starting"
	StatusRunning  Status = "running"
	StatusError    Status = "error"
)

const defaultHandshakeTimeout = 15 * time.Second

var ErrAttachSuperseded = errors.New("enginelink: attach scope superseded")

var ErrNoSupervisor = errors.New("enginelink: engine supervisor unavailable")

const pollInterval = 300 * time.Millisecond

var ErrEngineUnhealthy = errors.New("engine: not healthy within timeout")

type DialFunc func(ep engineapi.Endpoint) (*http.Client, error)

type ReadyFunc func(ctx context.Context, api *engineapi.Client, ep engineapi.Endpoint, version string) error

type supervisor interface {
	Locate(ctx context.Context) (engineapi.Endpoint, bool)
	Start() (engineapi.Endpoint, error)
}

type Link struct {
	sup              supervisor
	dial             DialFunc
	handshakeTimeout time.Duration

	mu          sync.Mutex
	status      Status
	lastError   string
	ep          engineapi.Endpoint
	version     string
	scopeCancel context.CancelFunc
}

func NewLink(sup supervisor) *Link {
	return &Link{
		sup:              sup,
		dial:             engineapi.Dial,
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

func (l *Link) Endpoint() engineapi.Endpoint {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.ep
}

func (l *Link) Version() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.version
}

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

	hc, err := l.dial(ep)
	if err != nil {
		return fmt.Errorf("dial %s %s: %w", ep.Network, ep.Address, err)
	}

	api := engineapi.NewClient(hc)
	vi, err := WaitForHealthy(scope, api, l.handshakeTimeout)
	if err != nil {
		return fmt.Errorf("handshake: %w", err)
	}

	return ready(scope, api, ep, vi.Version)
}

func (l *Link) CommitAttach(scope context.Context, ep engineapi.Endpoint, version string) bool {
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

func (l *Link) MarkRunning() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.status = StatusRunning
	l.lastError = ""
}

func (l *Link) Fail(reason string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.status = StatusError
	l.lastError = reason
}

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

func (l *Link) Disconnect() {
	l.mu.Lock()
	cancel := l.scopeCancel
	l.scopeCancel = nil
	l.status = StatusStopped
	l.lastError = ""
	l.ep = engineapi.Endpoint{}
	l.version = ""
	l.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

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

func (l *Link) CancelScope() {
	l.mu.Lock()
	cancel := l.scopeCancel
	l.scopeCancel = nil
	l.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func WaitForHealthy(ctx context.Context, api *engineapi.Client, timeout time.Duration) (engineapi.VersionInfo, error) {
	if api == nil {
		return engineapi.VersionInfo{}, errors.New("engine: nil client")
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	dctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if version, err := api.Version(dctx); err == nil {
		return version, nil
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-dctx.Done():
			if errors.Is(dctx.Err(), context.DeadlineExceeded) {
				return engineapi.VersionInfo{}, fmt.Errorf("%w (after %s)", ErrEngineUnhealthy, timeout)
			}
			return engineapi.VersionInfo{}, dctx.Err()
		case <-ticker.C:
			if version, err := api.Version(dctx); err == nil {
				return version, nil
			}
		}
	}
}

var (
	ErrWorkspaceIDRequired = errors.New("workspace id is required for event stream")
	ErrNoConnection        = errors.New("engine connection unavailable")
	ErrTransportNotWired   = errors.New("event stream unavailable: transport not wired")
)

var StreamKinds = []string{
	"message", "run_complete", "task_progress", "permission_request", "file",
}

type EventConsumer interface {
	Consume(events <-chan engineapi.StreamEvent)
}

func AttachStream(scope context.Context, api *engineapi.Client, consumer EventConsumer, workspaceID string, lost func(scope context.Context, reason string)) error {
	events, _, err := api.Stream(scope, workspaceID, StreamKinds...)
	if err != nil {
		return fmt.Errorf("event stream attach failed: %w", err)
	}

	go func() {
		consumer.Consume(events)
		if scope.Err() == nil {
			lost(scope, "engine event stream disconnected")
		}
	}()
	return nil
}
