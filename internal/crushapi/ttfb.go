package crushapi

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// pendingFirstByte is the per-response slot that captures the first byte
// timestamp before the SSE reader knows the runID. The reader migrates it
// into the runID-keyed registry entry as soon as it decodes a frame that
// carries a runID.
type pendingFirstByte struct {
	stamped atomic.Pointer[time.Time]
}

func (p *pendingFirstByte) stamp(t time.Time) { p.stamped.CompareAndSwap(nil, &t) }
func (p *pendingFirstByte) snapshot() *time.Time {
	return p.stamped.Load()
}

// firstByteBody wraps an http.Response body so the first successful Read
// stamps a monotonic timestamp into the response-scoped pending slot.
type firstByteBody struct {
	inner   io.ReadCloser
	slot    *pendingFirstByte
	stamped atomic.Bool
}

func (b *firstByteBody) Read(p []byte) (int, error) {
	n, err := b.inner.Read(p)
	if n > 0 && b.stamped.CompareAndSwap(false, true) {
		b.slot.stamp(time.Now())
	}
	return n, err
}

func (b *firstByteBody) Close() error { return b.inner.Close() }

// ttfbEntry carries the per-attempt observation state for a single model call.
// Both fields are atomic pointers so concurrent readers and writers do not
// block each other; map mutations alone need the registry mutex.
type ttfbEntry struct {
	firstByte atomic.Pointer[time.Time] // migrated from pending slot
	sseFirst  atomic.Pointer[time.Time] // stamped once by the SSE side
}

func (e *ttfbEntry) setFirstByte(t time.Time) {
	e.firstByte.CompareAndSwap(nil, &t)
}

func (e *ttfbEntry) setSSEFirst(t time.Time) {
	e.sseFirst.CompareAndSwap(nil, &t)
}

// TTFBRegistry is the per-Client registry that holds one entry per
// (runID, attempt, purpose) tuple. Map insertion is mutex-guarded; entry
// mutations are lock-free atomic stores. The type is exposed by name so
// the engineobserver package can consume the registry without depending on
// crushapi internals.
type TTFBRegistry struct {
	mu      sync.Mutex
	entries map[string]*ttfbEntry
	clock   func() time.Time // injectable for deterministic tests
}

// NewTTFBRegistry builds a registry with the wall clock as the default.
func NewTTFBRegistry() *TTFBRegistry {
	return &TTFBRegistry{
		entries: make(map[string]*ttfbEntry),
		clock:   func() time.Time { return time.Now() },
	}
}

func ttfbKey(runID string, attempt int, purpose string) string {
	var b bytes.Buffer
	b.Grow(len(runID) + 1 + 8 + 1 + len(purpose))
	b.WriteString(runID)
	b.WriteByte(':')
	b.WriteString(strconv.Itoa(attempt))
	b.WriteByte(':')
	b.WriteString(purpose)
	return b.String()
}

// ensure returns the entry for key, creating it if absent. The map mutex
// covers insertion only; entry mutations are lock-free atomic stores.
func (r *TTFBRegistry) ensure(runID string, attempt int, purpose string) *ttfbEntry {
	key := ttfbKey(runID, attempt, purpose)
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.entries[key]
	if !ok {
		e = &ttfbEntry{}
		r.entries[key] = e
	}
	return e
}

// BindFirstByte migrates the pending firstByte timestamp into the
// (runID, attempt, purpose) entry, creating it if absent. Returns the
// entry for downstream SSE-first stamping.
func (r *TTFBRegistry) BindFirstByte(runID string, attempt int, purpose string, t time.Time) *ttfbEntry {
	if runID == "" {
		return nil
	}
	e := r.ensure(runID, attempt, purpose)
	e.setFirstByte(t)
	return e
}

// MarkSSEFirst stamps the SSE-first-frame timestamp on the entry for the
// given runID/attempt/purpose, creating it if needed.
func (r *TTFBRegistry) MarkSSEFirst(runID string, attempt int, purpose string) {
	if runID == "" {
		return
	}
	e := r.ensure(runID, attempt, purpose)
	e.setSSEFirst(r.clock())
}

// Look returns a snapshot of the (firstByte, sseFirst) timestamps for the
// given tuple. Either or both may be nil.
func (r *TTFBRegistry) Look(runID string, attempt int, purpose string) (firstByte, sseFirst *time.Time) {
	key := ttfbKey(runID, attempt, purpose)
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.entries[key]
	if !ok {
		return nil, nil
	}
	return e.firstByte.Load(), e.sseFirst.Load()
}

// Evict removes the entry for the given tuple. Used by the host when the
// call ended in error or cancellation so a subsequent successful attempt
// does not inherit the prior entry's stamps.
func (r *TTFBRegistry) Evict(runID string, attempt int, purpose string) {
	if runID == "" {
		return
	}
	key := ttfbKey(runID, attempt, purpose)
	r.mu.Lock()
	delete(r.entries, key)
	r.mu.Unlock()
}

// ttfbTransport wraps a RoundTripper so every SSE response carries a body
// wrapper that stamps firstByte on first Read. Non-streaming responses are
// passed through unmodified.
type ttfbTransport struct {
	inner http.RoundTripper
}

func (t *ttfbTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.inner.RoundTrip(req)
	if err != nil || resp == nil {
		return resp, err
	}
	// Only SSE event streams need a first-byte timestamp; JSON round-trips
	// never inform first_byte_to_first_sse and would inflate the registry.
	if !isSSEStream(resp) {
		return resp, nil
	}
	resp.Body = &firstByteBody{inner: resp.Body, slot: &pendingFirstByte{}}
	return resp, nil
}

func isSSEStream(resp *http.Response) bool {
	return resp.Header.Get("Content-Type") == "text/event-stream"
}

// unwrapFirstByteBody returns the firstByteBody wrapper if resp.Body is one.
// The SSE reader uses this to recover the pending firstByte slot.
func unwrapFirstByteBody(resp *http.Response) (*firstByteBody, bool) {
	w, ok := resp.Body.(*firstByteBody)
	return w, ok
}
