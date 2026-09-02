package permission

import (
	"sync"
	"time"

	"github.com/Dyu-36/gotack/internal/crushapi"
)

// DefaultTTL is the single permission-expiry policy used by both the relay
// timer and the UI countdown.
const DefaultTTL = 5 * time.Minute

// Relay pairs inbound Crush permission requests with the answers that arrive
// from the UI through the bound call. The lifecycle is one-shot: a request is
// stored, a TTL timer is armed, and the next Take pops the entry. Expiry is a
// deny, never an auto-approval.
type Relay struct {
	ttl            time.Duration
	nextGeneration uint64

	mu      sync.Mutex
	pending map[string]entry
}

// entry is one waiting request together with the timer that will drop it.
// Keeping the pair in a single map gives insertion and deletion one owner.
type entry struct {
	req        crushapi.PermissionRequest
	timer      *time.Timer
	generation uint64
}

// NewRelay returns a Relay with the given default TTL. A zero TTL falls back
// to five minutes, matching the permission contract.
func NewRelay(ttl time.Duration) *Relay {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	return &Relay{
		ttl:     ttl,
		pending: make(map[string]entry),
	}
}

// Pending stores req and arms the expiry timer. A repeated id replaces the
// previous request. Generation tagging prevents an already-queued old timer
// callback from deleting the replacement.
func (r *Relay) Pending(req crushapi.PermissionRequest) int64 {
	if req.ID == "" {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextGeneration++
	generation := r.nextGeneration
	expiresAt := time.Now().Add(r.ttl)
	if previous, ok := r.pending[req.ID]; ok {
		previous.timer.Stop()
	}
	entry := entry{req: req, generation: generation}
	entry.timer = time.AfterFunc(time.Until(expiresAt), func() {
		r.drop(req.ID, generation)
	})
	r.pending[req.ID] = entry
	return expiresAt.UnixMilli()
}

// drop is the timer callback. It only removes the entry if it still belongs
// to the timer that fired; a stale callback cannot expire a replacement.
func (r *Relay) drop(id string, generation uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if current, ok := r.pending[id]; !ok || current.generation != generation {
		return
	}
	delete(r.pending, id)
}

// Take pops the request with the given id. The bool is true when the request
// existed; callers forward the action upstream via crushapi.GrantPermission.
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
