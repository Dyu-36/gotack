package engine

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

	"github.com/Dyu-36/gotack/internal/engineapi"
)

type fakeSupervisor struct {
	mu         sync.Mutex
	ep         engineapi.Endpoint
	found      bool
	startErr   error
	startCalls int
}

func (f *fakeSupervisor) Locate(context.Context) (engineapi.Endpoint, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.ep, f.found
}

func (f *fakeSupervisor) Start() (engineapi.Endpoint, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.startCalls++
	if f.startErr != nil {
		return engineapi.Endpoint{}, f.startErr
	}
	return f.ep, nil
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

type engineTransport struct {
	version     string
	versionHits int
	streamBody  func(req *http.Request) io.ReadCloser
	mu          sync.Mutex
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
			tr.mu.Lock()
			tr.versionHits++
			tr.mu.Unlock()
			payload, _ := json.Marshal(map[string]string{"version": tr.version, "platform": "test"})
			return respond(http.StatusOK, io.NopCloser(strings.NewReader(string(payload))), "application/json")
		case req.Method == http.MethodGet && strings.HasPrefix(req.URL.Path, "/v1/workspaces/") && strings.HasSuffix(req.URL.Path, "/events"):
			return respond(http.StatusOK, tr.streamBody(req), "text/event-stream")
		default:
			return respond(http.StatusNotFound, io.NopCloser(strings.NewReader(`{"message":"not found"}`)), "")
		}
	})
}

func newTestLink(t *testing.T, sup *fakeSupervisor, tr *engineTransport) *Link {
	t.Helper()
	sup.ep = engineapi.Endpoint{Network: "pipe", Address: "fake-engine"}
	sup.found = true
	link := NewLink(sup)
	link.dial = func(engineapi.Endpoint) (*http.Client, error) {
		return &http.Client{Transport: tr.roundTrip()}, nil
	}

	link.handshakeTimeout = 2 * time.Second
	return link
}

func readyRecorder(calls *int, epOut *engineapi.Endpoint, versionOut *string) ReadyFunc {
	return func(_ context.Context, _ *engineapi.Client, ep engineapi.Endpoint, version string) error {
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

func TestLifecycleConnectCancelReconnect(t *testing.T) {
	sup := &fakeSupervisor{}
	tr := &engineTransport{version: "1.2.3", streamBody: blockingBody}
	link := newTestLink(t, sup, tr)

	var wg sync.WaitGroup
	for cycle := 0; cycle < 8; cycle++ {
		scope, started := link.BeginConnect(context.Background())
		if !started {
			link.Disconnect()
			scope, started = link.BeginConnect(context.Background())
			if !started {
				t.Fatalf("cycle %d: link refused to reconnect from a settled state", cycle)
			}
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			err := link.Connect(scope, func(ctx context.Context, _ *engineapi.Client, ep engineapi.Endpoint, version string) error {
				if !link.CommitAttach(ctx, ep, version) {
					return ErrAttachSuperseded
				}
				link.MarkRunning()
				return nil
			})
			if err != nil && !errors.Is(err, ErrAttachSuperseded) {
				link.Fail(err.Error())
			}
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			scope2 := link.ReplaceStreamScope(context.Background())
			link.TransportLost(scope2, "churn")
			link.CancelScope()
		}()

		link.Disconnect()
	}

	wg.Wait()

	link.Disconnect()
	scope, started := link.BeginConnect(context.Background())
	if !started {
		t.Fatal("settled link refused the final reconnect")
	}
	var calls int
	if err := link.Connect(scope, func(ctx context.Context, api *engineapi.Client, ep engineapi.Endpoint, version string) error {
		calls++
		if !link.CommitAttach(ctx, ep, version) {
			return ErrAttachSuperseded
		}
		link.MarkRunning()
		return nil
	}); err != nil {
		t.Fatalf("final reconnect failed: %v", err)
	}
	if calls != 1 {
		t.Fatalf("ready calls = %d, want 1", calls)
	}
	if got := link.Status(); got != StatusRunning {
		t.Fatalf("status = %q, want running after reconnect", got)
	}
}

func TestConnectScopeCancellationAbandons(t *testing.T) {
	sup := &fakeSupervisor{}
	tr := &engineTransport{version: "1.2.3", streamBody: blockingBody}
	link := newTestLink(t, sup, tr)

	scope, started := link.BeginConnect(context.Background())
	if !started {
		t.Fatal("stopped link must accept a connect attempt")
	}

	done := make(chan error, 1)
	go func() {
		done <- link.Connect(scope, func(ctx context.Context, _ *engineapi.Client, ep engineapi.Endpoint, version string) error {
			if !link.CommitAttach(ctx, ep, version) {
				return ErrAttachSuperseded
			}
			link.MarkRunning()
			return nil
		})
	}()

	link.Disconnect()

	select {
	case err := <-done:
		if err != nil && !errors.Is(err, ErrAttachSuperseded) {
			if link.Status() == StatusRunning {
				t.Fatalf("superseded attempt reached running (err = %v)", err)
			}
		}
	case <-time.After(5 * time.Second):
		t.Fatal("connect attempt did not finish after the scope was cancelled")
	}
	if got := link.Status(); got == StatusRunning {
		t.Fatal("a cancelled scope must never leave the link running")
	}
}

type recordingConsumer struct {
	mu     sync.Mutex
	count  int
	done   chan struct{}
	closed bool
}

func (r *recordingConsumer) Consume(events <-chan engineapi.StreamEvent) {
	for range events {
		r.mu.Lock()
		r.count++
		r.mu.Unlock()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.closed {
		r.closed = true
		close(r.done)
	}
}

func TestAttachStreamReportsUnexpectedClose(t *testing.T) {
	envelope := `data: {"type":"message","payload":{"type":"updated","payload":{}}}

`
	tr := &engineTransport{version: "1.2.3", streamBody: sseBody(envelope)}
	api := engineapi.NewClient(&http.Client{Transport: tr.roundTrip()})

	consumer := &recordingConsumer{done: make(chan struct{})}
	lost := make(chan string, 1)
	scope, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := AttachStream(scope, api, consumer, "ws-1", func(context.Context, string) {
		lost <- "engine event stream disconnected"
	}); err != nil {
		t.Fatalf("AttachStream() error = %v", err)
	}

	select {
	case <-consumer.done:
	case <-time.After(5 * time.Second):
		t.Fatal("consumer never saw the stream close")
	}
	if consumer.count < 1 {
		t.Fatalf("consumer events = %d, want the envelope delivered", consumer.count)
	}
	select {
	case reason := <-lost:
		if reason != "engine event stream disconnected" {
			t.Fatalf("loss reason = %q", reason)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("unexpected stream close was not reported")
	}
}

func TestAttachStreamSilentOnCancellation(t *testing.T) {
	tr := &engineTransport{version: "1.2.3", streamBody: blockingBody}
	api := engineapi.NewClient(&http.Client{Transport: tr.roundTrip()})

	consumer := &recordingConsumer{done: make(chan struct{})}
	var lostCalls int
	var mu sync.Mutex
	scope, cancel := context.WithCancel(context.Background())

	if err := AttachStream(scope, api, consumer, "ws-1", func(context.Context, string) {
		mu.Lock()
		lostCalls++
		mu.Unlock()
	}); err != nil {
		t.Fatalf("AttachStream() error = %v", err)
	}

	cancel()
	select {
	case <-consumer.done:
	case <-time.After(5 * time.Second):
		t.Fatal("consumer did not return after scope cancellation")
	}
	mu.Lock()
	defer mu.Unlock()
	if lostCalls != 0 {
		t.Fatalf("loss reports = %d, a cancelled scope must stay silent", lostCalls)
	}
}

func TestAttachStreamSurfacesServerError(t *testing.T) {
	api := engineapi.NewClient(&http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 500, Body: io.NopCloser(strings.NewReader(`{"message":"boom"}`)), Header: make(http.Header), Request: req}, nil
	})})

	err := AttachStream(context.Background(), api, &recordingConsumer{done: make(chan struct{})}, "ws-1", func(context.Context, string) {})
	if err == nil || !strings.Contains(err.Error(), "event stream attach failed") {
		t.Fatalf("AttachStream() error = %v, want the wrapped attach failure", err)
	}
}

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
	var ep engineapi.Endpoint
	var version string
	err := link.Connect(scope, func(ctx context.Context, api *engineapi.Client, gotEP engineapi.Endpoint, gotVersion string) error {
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
	tr.mu.Lock()
	versionHits := tr.versionHits
	tr.mu.Unlock()
	if versionHits != 1 {
		t.Fatalf("version requests = %d, want 1", versionHits)
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
	link.dial = func(engineapi.Endpoint) (*http.Client, error) {
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

	link.Fail(err.Error())
	if got := link.Status(); got != StatusError {
		t.Fatalf("status = %q, want error", got)
	}
	if !strings.Contains(link.LastError(), "dial") {
		t.Fatalf("last error = %q, want the dial reason", link.LastError())
	}

	link.dial = func(engineapi.Endpoint) (*http.Client, error) {
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
	if link.CommitAttach(scope, engineapi.Endpoint{Address: "late"}, "v1") {
		t.Fatal("commit attach accepted a cancelled scope")
	}
	if got := link.Endpoint(); got.Address == "late" {
		t.Fatalf("endpoint = %+v, dead scope must not clobber state", got)
	}
}

func TestTransportLost(t *testing.T) {
	link := NewLink(&fakeSupervisor{})

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
	link.CommitAttach(scope, engineapi.Endpoint{Address: "ep"}, "v9")
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

	link.CancelScope()
}
