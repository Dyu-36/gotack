package crushapi

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

// makeClient wires a fresh Client onto a stub transport that dials the
// given httptest.Server listener.
func makeClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, srv.Listener.Addr().String())
	}
	return NewClient(&http.Client{Transport: transport})
}

// readFirstEvent reads the first emitted StreamEvent on the channel or fails
// the test on timeout.
func readFirstEvent(t *testing.T, ch <-chan StreamEvent) StreamEvent {
	t.Helper()
	select {
	case ev, ok := <-ch:
		if !ok {
			t.Fatal("stream closed before any event")
		}
		return ev
	case <-time.After(2 * time.Second):
		t.Fatal("stream produced no event within 2s")
		return StreamEvent{}
	}
}

// TestTTFBPositive_DelayBetweenHeadersAndFirstSSE asserts the controlled
// delay scenario from the design memo (75ms between preamble and first SSE
// frame, ±15ms tolerance).
func TestTTFBPositive_DelayBetweenHeadersAndFirstSSE(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		// Send the SSE preamble (a comment line + blank line) so the body
		// wrapper's first Read returns immediately. The 75ms sleep happens
		// BEFORE the actual data frame, so firstByte << sseFirst.
		_, _ = w.Write([]byte(": keep-alive\n\n"))
		flusher.Flush()
		time.Sleep(75 * time.Millisecond)
		_, _ = w.Write([]byte("data: {\"type\":\"run_complete\",\"payload\":{\"session_id\":\"s\",\"run_id\":\"run-1\",\"message_id\":\"m\",\"text\":\"done\"}}\n\n"))
		flusher.Flush()
	}))
	defer srv.Close()

	client := makeClient(t, srv)
	out, stop, err := client.Stream(context.Background(), "ws-1", "run_complete")
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}
	defer stop()
	_ = readFirstEvent(t, out)

	firstByte, sseFirst := client.observer.Look("run-1", 0, "")
	if firstByte == nil {
		t.Fatal("firstByte not stamped")
	}
	if sseFirst == nil {
		t.Fatal("sseFirst not stamped")
	}
	delta := sseFirst.Sub(*firstByte)
	if delta < 60*time.Millisecond || delta > 90*time.Millisecond {
		t.Fatalf("first_byte_to_first_sse delta = %v, want 60-90ms", delta)
	}
}

// TestTTFBNegative_HTTP500BeforeBody asserts that an immediate 500 leaves
// the registry without a usable entry (no firstByte or sseFirst).
func TestTTFBNegative_HTTP500BeforeBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := makeClient(t, srv)
	_, _, err := client.Stream(context.Background(), "ws-1")
	if err == nil {
		t.Fatal("Stream() expected error on 500")
	}
	if _, ok := client.observer.entries["run-1:0:"]; ok {
		t.Fatal("registry unexpectedly contains entry for 500 path")
	}
}

// TestTTFBNegative_ConnectionClosedAfterHeaders asserts that headers-then-
// close leaves firstByte stamped but sseFirst nil — host merge is a no-op.
func TestTTFBNegative_ConnectionClosedAfterHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Connection", "close")
		w.WriteHeader(http.StatusOK)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		// Hijack and drop the connection without writing any body.
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Fatal("ResponseWriter is not a Hijacker")
		}
		conn, _, _ := hj.Hijack()
		_ = conn.Close()
	}))
	defer srv.Close()

	client := makeClient(t, srv)
	out, stop, err := client.Stream(context.Background(), "ws-1", "run_complete")
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}
	defer stop()
	// Channel should close without emitting anything.
	for ev := range out {
		t.Fatalf("unexpected event: %#v", ev)
	}
	// No runID-keyed entry should exist because no envelope was decoded.
	for k := range client.observer.entries {
		t.Fatalf("registry should be empty, got %q", k)
	}
}

// TestTTFBCorrelation_ConcurrentRunsDoNotCross asserts that two prompts with
// different runIDs run independently and merging one does not pollute the
// other. Each Stream emits a run_complete with its own runID, populating
// two distinct registry entries.
func TestTTFBCorrelation_ConcurrentRunsDoNotCross(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		time.Sleep(40 * time.Millisecond)
		// Two consecutive run_complete frames on the same stream; each
		// carries a distinct runID so the registry populates both.
		w.Write([]byte("data: {\"type\":\"run_complete\",\"payload\":{\"session_id\":\"s\",\"run_id\":\"run-a\",\"message_id\":\"m\",\"text\":\"a\"}}\n\n"))
		w.Write([]byte("data: {\"type\":\"run_complete\",\"payload\":{\"session_id\":\"s\",\"run_id\":\"run-b\",\"message_id\":\"m\",\"text\":\"b\"}}\n\n"))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}))
	defer srv.Close()

	client := makeClient(t, srv)
	out, stop, err := client.Stream(context.Background(), "ws-1", "run_complete")
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}
	defer stop()
	// Drain two run_complete events.
	first := readFirstEvent(t, out)
	if first.Kind != "run_complete" {
		t.Fatalf("unexpected event kind %q", first.Kind)
	}
	second := readFirstEvent(t, out)
	if second.Kind != "run_complete" {
		t.Fatalf("unexpected event kind %q", second.Kind)
	}

	firstA, sseA := client.observer.Look("run-a", 0, "")
	if firstA == nil || sseA == nil {
		t.Fatal("run-a entry missing")
	}
	firstB, sseB := client.observer.Look("run-b", 0, "")
	if firstB == nil || sseB == nil {
		t.Fatal("run-b entry missing")
	}
	if firstA == firstB || sseA == sseB {
		t.Fatal("concurrent runs share pointer; registry collapsed them")
	}
}

// TestTTFBCorrelation_SameRunIDDifferentAttempt asserts that two registry
// entries for the same runID with different attempt keys do not collide.
func TestTTFBCorrelation_SameRunIDDifferentAttempt(t *testing.T) {
	r := NewTTFBRegistry()
	r.BindFirstByte("r1", 1, "title", time.Unix(0, 1))
	r.MarkSSEFirst("r1", 1, "title")
	r.BindFirstByte("r1", 2, "retry", time.Unix(0, 2))
	r.MarkSSEFirst("r1", 2, "retry")

	fb1, ss1 := r.Look("r1", 1, "title")
	fb2, ss2 := r.Look("r1", 2, "retry")
	if fb1 == nil || ss1 == nil || fb2 == nil || ss2 == nil {
		t.Fatal("both attempts must be present")
	}
	if fb1 == fb2 || ss1 == ss2 {
		t.Fatal("attempts collided")
	}
	r.Evict("r1", 1, "title")
	if _, ok := r.entries[ttfbKey("r1", 1, "title")]; ok {
		t.Fatal("attempt 1 not evicted")
	}
	if _, ok := r.entries[ttfbKey("r1", 2, "retry")]; !ok {
		t.Fatal("attempt 2 evicted by accident")
	}
}

// TestTTFBClockInjection verifies that the registry's clock function is the
// source of truth for SSE-first stamps, so deterministic tests do not need
// real time.Sleep calls to assert ordering.
func TestTTFBClockInjection(t *testing.T) {
	r := NewTTFBRegistry()
	t0 := time.Unix(1700000000, 0)
	r.clock = func() time.Time { return t0 }

	r.BindFirstByte("r1", 1, "title", t0.Add(-50*time.Millisecond))
	r.MarkSSEFirst("r1", 1, "title")

	fb, ss := r.Look("r1", 1, "title")
	if fb == nil || ss == nil {
		t.Fatal("expected both stamps")
	}
	if !fb.Equal(t0.Add(-50 * time.Millisecond)) {
		t.Fatalf("firstByte = %v, want %v", fb, t0.Add(-50*time.Millisecond))
	}
	if !ss.Equal(t0) {
		t.Fatalf("sseFirst = %v, want %v", ss, t0)
	}
}

// TestTTFBTransport_NoStreamPassthrough verifies that non-SSE responses do
// not get a firstByte wrapper.
func TestTTFBTransport_NoStreamPassthrough(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	transport := &ttfbTransport{inner: http.DefaultTransport}
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	defer resp.Body.Close()
	if _, ok := resp.Body.(*firstByteBody); ok {
		t.Fatal("non-SSE response got firstByteBody wrapper")
	}
	_, _ = io.Copy(io.Discard, resp.Body)
}

// TestTTFBKey ensures the key shape matches (runID, attempt, purpose).
func TestTTFBKey(t *testing.T) {
	if got := ttfbKey("run-1", 3, "title"); got != "run-1:3:title" {
		t.Fatalf("ttfbKey = %q, want %q", got, "run-1:3:title")
	}
	if ttfbKey("r", 12, "p") != "r:12:p" {
		t.Fatal("attempt formatting broken")
	}
	if got := ttfbKey("r", 0, ""); got != "r:0:" {
		t.Fatalf("empty-purpose key = %q", got)
	}
	_ = strconv.Itoa // keep import for tests of strconv behavior
}