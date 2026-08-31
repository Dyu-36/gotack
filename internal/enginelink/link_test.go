package enginelink

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Dyu-36/gotack/internal/crushapi"
)

// fakeSupervisor implements engine.EngineAPI without launching a process.
type fakeSupervisor struct {
	mu         sync.Mutex
	ep         crushapi.Endpoint
	found      bool
	startErr   error
	startCalls int
	owned      bool
}

func (f *fakeSupervisor) Owned() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.owned
}

func (f *fakeSupervisor) Locate(context.Context) (crushapi.Endpoint, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.ep, f.found
}

func (f *fakeSupervisor) Start() (crushapi.Endpoint, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.startCalls++
	if f.startErr != nil {
		return crushapi.Endpoint{}, f.startErr
	}
	f.owned = true
	return f.ep, nil
}

func (f *fakeSupervisor) Stop() error { return nil }

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

// engineTransport answers the two routes the connect sequence and the event
// stream touch. streamBody controls what the SSE endpoint returns: a fixed
// payload or a reader that blocks until the request context dies.
type engineTransport struct {
	version    string
	streamBody func(req *http.Request) io.ReadCloser
	streamHits int
	mu         sync.Mutex
}

func (tr *engineTransport) roundTrip() http.RoundTripper {
	return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		respond := func(status int, body io.ReadCloser, contentType string) (*http.Response, error) {
			header := make(http.Header)
			if contentType != "" {
				header.Set("Content-Type", contentType)
			}
			return &http.Response{StatusCode: status, Body: body, Header: header, Request: req}, nil
		}
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/version":
			payload, _ := json.Marshal(map[string]string{"version": tr.version, "platform": "test"})
			return respond(http.StatusOK, io.NopCloser(strings.NewReader(string(payload))), "application/json")
		case req.Method == http.MethodGet && strings.HasPrefix(req.URL.Path, "/v1/workspaces/") && strings.HasSuffix(req.URL.Path, "/events"):
			tr.mu.Lock()
			tr.streamHits++
			tr.mu.Unlock()
			return respond(http.StatusOK, tr.streamBody(req), "text/event-stream")
		default:
			return respond(http.StatusNotFound, io.NopCloser(strings.NewReader(`{"message":"not found"}`)), "")
		}
	})
}

// newTestLink builds a Link whose dial reaches the fake engine transport.
func newTestLink(t *testing.T, sup *fakeSupervisor, tr *engineTransport) *Link {
	t.Helper()
	sup.ep = crushapi.Endpoint{Network: "pipe", Address: "fake-engine"}
	sup.found = true
	link := NewLink(sup)
	link.dial = func(context.Context, crushapi.Endpoint) (*http.Client, error) {
		return &http.Client{Transport: tr.roundTrip()}, nil
	}
	// The fake version endpoint answers immediately, so the handshake never
	// needs its real timeout; keep tests fast if it ever polls anyway.
	link.handshakeTimeout = 2 * time.Second
	return link
}

func readyRecorder(calls *int, epOut *crushapi.Endpoint, versionOut *string) ReadyFunc {
	return func(_ context.Context, _ *crushapi.Client, ep crushapi.Endpoint, version string) error {
		*calls++
		if epOut != nil {
			*epOut = ep
		}
		if versionOut != nil {
			*versionOut = version
		}
		return nil
	}
}

func sseBody(payload string) func(*http.Request) io.ReadCloser {
	return func(*http.Request) io.ReadCloser {
		return io.NopCloser(strings.NewReader(payload))
	}
}

// blockingBody keeps the stream open until the request context is cancelled,
// mirroring a live engine connection.
func blockingBody(req *http.Request) io.ReadCloser {
	return contextReader{ctx: req.Context()}
}

type contextReader struct {
	ctx context.Context
}

func (r contextReader) Read([]byte) (int, error) {
	<-r.ctx.Done()
	return 0, r.ctx.Err()
}

func (contextReader) Close() error { return nil }

func TestConnectHandshakeSuccess(t *testing.T) {
	sup := &fakeSupervisor{}
	tr := &engineTransport{version: "1.2.3", streamBody: sseBody("")}
	link := newTestLink(t, sup, tr)

	scope, started := link.BeginConnect(context.Background())
	if !started {
		t.Fatal("stopped link must accept a connect attempt")
	}
	if got := link.Status(); got != StatusStarting {
		t.Fatalf("status = %q, want starting", got)
	}
	if link.LastError() != "" {
		t.Fatalf("last error = %q, want empty at connect start", link.LastError())
	}

	var calls int
	var ep crushapi.Endpoint
	var version string
	err := link.Connect(scope, func(ctx context.Context, api *crushapi.Client, gotEP crushapi.Endpoint, gotVersion string) error {
		if !link.CommitAttach(ctx, gotEP, gotVersion) {
			t.Fatal("commit attach rejected the live connect scope")
		}
		return readyRecorder(&calls, &ep, &version)(ctx, api, gotEP, gotVersion)
	})
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("ready calls = %d, want 1", calls)
	}
	if version != "1.2.3" {
		t.Fatalf("version = %q, want 1.2.3", version)
	}
	if ep.Address != "fake-engine" {
		t.Fatalf("endpoint = %+v, want the supervisor endpoint", ep)
	}

	link.MarkRunning()
	if got := link.Status(); got != StatusRunning {
		t.Fatalf("status = %q, want running", got)
	}
	if got := link.Endpoint(); got != ep {
		t.Fatalf("recorded endpoint = %+v, want %+v", got, ep)
	}
	if got := link.Version(); got != "1.2.3" {
		t.Fatalf("recorded version = %q, want 1.2.3", got)
	}

	// A running link rejects duplicate connect attempts.
	if _, started := link.BeginConnect(context.Background()); started {
		t.Fatal("running link accepted a second connect attempt")
	}
}

func TestConnectLaunchesEngineWhenAbsent(t *testing.T) {
	sup := &fakeSupervisor{}
	tr := &engineTransport{version: "9.9.9", streamBody: sseBody("")}
	link := newTestLink(t, sup, tr)
	sup.found = false

	scope, started := link.BeginConnect(context.Background())
	if !started {
		t.Fatal("stopped link must accept a connect attempt")
	}
	var calls int
	if err := link.Connect(scope, readyRecorder(&calls, nil, nil)); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	if sup.startCalls != 1 {
		t.Fatalf("supervisor starts = %d, want 1", sup.startCalls)
	}
	if calls != 1 {
		t.Fatalf("ready calls = %d, want 1", calls)
	}
}

func TestConnectFailureAllowsRetry(t *testing.T) {
	sup := &fakeSupervisor{}
	tr := &engineTransport{version: "1.2.3", streamBody: sseBody("")}
	link := newTestLink(t, sup, tr)
	dialErr := errors.New("pipe busy")
	link.dial = func(context.Context, crushapi.Endpoint) (*http.Client, error) {
		return nil, dialErr
	}

	scope, started := link.BeginConnect(context.Background())
	if !started {
		t.Fatal("stopped link must accept a connect attempt")
	}
	err := link.Connect(scope, readyRecorder(new(int), nil, nil))
	if !errors.Is(err, dialErr) {
		t.Fatalf("Connect() error = %v, want it to wrap the dial error", err)
	}

	// The host mirrors failConnect: the failure is recorded and the next
	// attempt is allowed once the frontend backoff fires.
	link.Fail(err.Error())
	if got := link.Status(); got != StatusError {
		t.Fatalf("status = %q, want error", got)
	}
	if !strings.Contains(link.LastError(), "dial") {
		t.Fatalf("last error = %q, want the dial reason", link.LastError())
	}

	// The retry succeeds once the transport recovers.
	link.dial = func(context.Context, crushapi.Endpoint) (*http.Client, error) {
		return &http.Client{Transport: tr.roundTrip()}, nil
	}
	scope2, started := link.BeginConnect(context.Background())
	if !started {
		t.Fatal("failed link must accept a retry")
	}
	var calls int
	if err := link.Connect(scope2, readyRecorder(&calls, nil, nil)); err != nil {
		t.Fatalf("retry Connect() error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("ready calls on retry = %d, want 1", calls)
	}
}

func TestConnectReportsLaunchFailure(t *testing.T) {
	sup := &fakeSupervisor{startErr: errors.New("binary missing")}
	tr := &engineTransport{version: "1.2.3", streamBody: sseBody("")}
	link := newTestLink(t, sup, tr)
	sup.found = false

	scope, _ := link.BeginConnect(context.Background())
	err := link.Connect(scope, readyRecorder(new(int), nil, nil))
	if err == nil || !strings.Contains(err.Error(), "launch engine") {
		t.Fatalf("Connect() error = %v, want the launch failure", err)
	}
}

func TestConnectWithoutSupervisor(t *testing.T) {
	link := NewLink(nil)
	scope, _ := link.BeginConnect(context.Background())
	if err := link.Connect(scope, readyRecorder(new(int), nil, nil)); !errors.Is(err, ErrNoSupervisor) {
		t.Fatalf("Connect() error = %v, want ErrNoSupervisor", err)
	}
}

func TestCommitAttachRejectsDeadScope(t *testing.T) {
	link := NewLink(&fakeSupervisor{})
	scope, cancel := context.WithCancel(context.Background())
	cancel()
	if link.CommitAttach(scope, crushapi.Endpoint{Address: "late"}, "v1") {
		t.Fatal("commit attach accepted a cancelled scope")
	}
	if got := link.Endpoint(); got.Address == "late" {
		t.Fatalf("endpoint = %+v, dead scope must not clobber state", got)
	}
}

func TestTransportLost(t *testing.T) {
	link := NewLink(&fakeSupervisor{})

	// Not running yet: a loss report must be dropped.
	scope, cancel := context.WithCancel(context.Background())
	defer cancel()
	if link.TransportLost(scope, "early") {
		t.Fatal("transport loss accepted before the link was running")
	}

	link.MarkRunning()
	if !link.TransportLost(scope, "stream gone") {
		t.Fatal("running link refused a transport loss report")
	}
	if got := link.Status(); got != StatusError {
		t.Fatalf("status = %q, want error", got)
	}
	if link.LastError() != "stream gone" {
		t.Fatalf("last error = %q, want the loss reason", link.LastError())
	}

	// A second report on an already-unhealthy link must not re-fire.
	if link.TransportLost(scope, "again") {
		t.Fatal("transport loss fired twice for one outage")
	}
}

func TestTransportLostIgnoresCancelledScope(t *testing.T) {
	link := NewLink(&fakeSupervisor{})
	link.MarkRunning()

	scope, cancel := context.WithCancel(context.Background())
	cancel()
	if link.TransportLost(scope, "intentional stop") {
		t.Fatal("cancelled scope reported as a transport loss")
	}
	if got := link.Status(); got != StatusRunning {
		t.Fatalf("status = %q, an intentional disconnect must not error the link", got)
	}
}

func TestDisconnectResetsState(t *testing.T) {
	link := NewLink(&fakeSupervisor{})
	scope, _ := link.BeginConnect(context.Background())
	link.CommitAttach(scope, crushapi.Endpoint{Address: "ep"}, "v9")
	link.MarkRunning()

	link.Disconnect()

	if scope.Err() == nil {
		t.Fatal("disconnect must cancel the live scope")
	}
	if got := link.Status(); got != StatusStopped {
		t.Fatalf("status = %q, want stopped", got)
	}
	if link.LastError() != "" || link.Version() != "" || link.Endpoint().Address != "" {
		t.Fatal("disconnect must clear the recorded connection facts")
	}
	// Disconnecting twice is a no-op, not a panic.
	link.Disconnect()
}

func TestReplaceStreamScopeCancelsPrevious(t *testing.T) {
	link := NewLink(&fakeSupervisor{})

	first := link.ReplaceStreamScope(context.Background())
	second := link.ReplaceStreamScope(context.Background())
	if first.Err() == nil {
		t.Fatal("replacing the stream scope must cancel the previous one")
	}
	if second.Err() != nil {
		t.Fatal("the fresh stream scope must be live")
	}

	link.CancelScope()
	if second.Err() == nil {
		t.Fatal("cancel scope must cancel the live stream scope")
	}
	// Rolling back twice must not panic.
	link.CancelScope()
}
