package permission

import (
	"testing"
	"time"

	"github.com/Dyu-36/gotack/internal/crushapi"
)

func testReq(id string) crushapi.PermissionRequest {
	return crushapi.PermissionRequest{ID: id, SessionID: "s1", ToolName: "bash"}
}

func pendingCount(r *Relay) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.pending)
}

func TestRelayTake(t *testing.T) {
	r := NewRelay(time.Minute)
	r.Pending(testReq("p1"))

	req, ok := r.Take("p1")
	if !ok || req.ID != "p1" {
		t.Fatalf("Take = %v, %v; want p1, true", req, ok)
	}
	if _, ok := r.Take("p1"); ok {
		t.Fatal("second Take must not find the popped request")
	}
	if n := pendingCount(r); n != 0 {
		t.Fatalf("pending count = %d, want 0", n)
	}
}

func TestRelayExpiry(t *testing.T) {
	r := NewRelay(20 * time.Millisecond)
	before := time.Now().Add(15 * time.Millisecond).UnixMilli()
	expiresAt := r.Pending(testReq("p2"))
	after := time.Now().Add(25 * time.Millisecond).UnixMilli()
	if expiresAt < before || expiresAt > after {
		t.Fatalf("expiry = %d, want relay TTL deadline in [%d, %d]", expiresAt, before, after)
	}

	time.Sleep(60 * time.Millisecond)
	if _, ok := r.Take("p2"); ok {
		t.Fatal("expired request must be dropped")
	}
}

func TestRelayUnknownAndZeroTTL(t *testing.T) {
	r := NewRelay(0) // falls back to the 5m default
	if _, ok := r.Take("missing"); ok {
		t.Fatal("unknown id must not resolve")
	}
	r.Pending(testReq("p3"))
	if n := pendingCount(r); n != 1 {
		t.Fatalf("pending count = %d, want 1", n)
	}
}

func TestRelayReplacementIgnoresStaleTimer(t *testing.T) {
	r := NewRelay(time.Minute)
	r.Pending(testReq("replace"))
	r.mu.Lock()
	oldGeneration := r.pending["replace"].generation
	r.mu.Unlock()

	r.Pending(testReq("replace"))
	// Simulate an old callback that was already queued when Stop returned
	// false. It must not expire the replacement entry.
	r.drop("replace", oldGeneration)
	if _, ok := r.Take("replace"); !ok {
		t.Fatal("stale timer callback removed replacement request")
	}
}
