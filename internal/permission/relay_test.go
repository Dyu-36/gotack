package permission

import (
	"testing"
	"time"

	"github.com/Dyu-36/gotack/internal/crushapi"
)

func testReq(id string) crushapi.PermissionRequest {
	return crushapi.PermissionRequest{ID: id, SessionID: "s1", ToolName: "bash"}
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
	if n := r.PendingCount(); n != 0 {
		t.Fatalf("PendingCount = %d, want 0", n)
	}
}

func TestRelayExpiry(t *testing.T) {
	r := NewRelay(20 * time.Millisecond)
	r.Pending(testReq("p2"))

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
	if n := r.PendingCount(); n != 1 {
		t.Fatalf("PendingCount = %d, want 1", n)
	}
}
