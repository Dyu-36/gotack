package enginelink

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Dyu-36/gotack/internal/crushapi"
)

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
			err := link.Connect(scope, func(ctx context.Context, _ *crushapi.Client, ep crushapi.Endpoint, version string) error {
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
	if err := link.Connect(scope, func(ctx context.Context, api *crushapi.Client, ep crushapi.Endpoint, version string) error {
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
		done <- link.Connect(scope, func(ctx context.Context, _ *crushapi.Client, ep crushapi.Endpoint, version string) error {
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

func (r *recordingConsumer) Consume(events <-chan crushapi.StreamEvent) {
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
	api := crushapi.NewClient(&http.Client{Transport: tr.roundTrip()})

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
	api := crushapi.NewClient(&http.Client{Transport: tr.roundTrip()})

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

	api := crushapi.NewClient(&http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 500,
			Body:       io.NopCloser(strings.NewReader(`{"message":"boom"}`)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})})

	err := AttachStream(context.Background(), api, &recordingConsumer{done: make(chan struct{})}, "ws-1", func(context.Context, string) {})
	if err == nil || !strings.Contains(err.Error(), "event stream attach failed") {
		t.Fatalf("AttachStream() error = %v, want the wrapped attach failure", err)
	}
}
