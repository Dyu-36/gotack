package permission

import (
	"sync"
	"time"

	"github.com/Dyu-36/gotack/internal/crushapi"
)

// relay.go -- role: pair an inbound request with the answer coming from the UI.
//
// Requests are emitted as events, answers arrive as bound calls in
// bind_permission.go. Every request is answered or expires: never auto-approve.

// Relay pairs inbound Crush permission requests with the answers that arrive
// from the UI through the bound call. The lifecycle is one-shot: a request is
// stored, a TTL timer is armed, and the next Take pops the entry. The default
// TTL is short because the agent is blocked waiting for the user; expiring
// without an answer effectively means "deny".
type Relay struct {
	ttl time.Duration

	mu      sync.Mutex
	pending map[string]entry
}

// entry is one waiting request together with the timer that will drop it.
// Keeping the pair in a single map means there is exactly one place to insert
// and one place to delete. The previous parallel pending/timers maps had to be
// kept in sync by hand at three call sites, which is a bug class, not a style
// preference.
type entry struct {
	req   crushapi.PermissionRequest
	timer *time.Timer
}

// NewRelay returns a Relay with the given default TTL. A zero TTL falls back
// to five minutes, which matches the spec's "ttl ~5m" recommendation.
func NewRelay(ttl time.Duration) *Relay {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &Relay{
		ttl:     ttl,
		pending: make(map[string]entry),
	}
}

// Pending stores req and arms the expiry timer. If a request with the same ID
// already exists it is replaced (the second copy wins) and the previous timer
// is stopped. This matches the agent's behaviour of re-issuing a permission
// request when the user disconnects mid-flight.
func (r *Relay) Pending(req crushapi.PermissionRequest) {
	if req.ID == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	// Replace any prior entry; an old timer is no longer relevant.
	if prev, ok := r.pending[req.ID]; ok {
		prev.timer.Stop()
	}
	r.pending[req.ID] = entry{
		req:   req,
		timer: time.AfterFunc(r.ttl, func() { r.drop(req.ID) }),
	}
}

// drop is the timer callback. The bind layer treats a missing entry as "deny",
// so expiry is nothing more than a delete.
func (r *Relay) drop(id string) {
	r.mu.Lock()
	delete(r.pending, id)
	r.mu.Unlock()
}

// Take pops the request with the given id. The bool is true when the request
// existed; the caller forwards the action upstream via
// crushapi.GrantPermission. Expiry and tests use the same entry point. An
// empty id needs no special case: Pending never stores one.
func (r *Relay) Take(id string) (crushapi.PermissionRequest, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.pending[id]
	if !ok {
		return crushapi.PermissionRequest{}, false
	}
	e.timer.Stop()
	delete(r.pending, id)
	return e.req, true
}

// PendingCount reports how many requests are still waiting. Test-only helper.
func (r *Relay) PendingCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.pending)
}
