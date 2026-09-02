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

type DialFunc func(ctx context.Context, ep crushapi.Endpoint) (*http.Client, error)

type ReadyFunc func(ctx context.Context, api *crushapi.Client, ep crushapi.Endpoint, version string) error

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
	l.ep = crushapi.Endpoint{}
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
