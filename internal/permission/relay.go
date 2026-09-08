package permission

import (
	"sync"
	"time"

	"github.com/Dyu-36/gotack/internal/engineapi"
)

const DefaultTTL = 5 * time.Minute

type Relay struct {
	ttl            time.Duration
	nextGeneration uint64

	mu      sync.Mutex
	pending map[string]entry
}

type entry struct {
	req        engineapi.PermissionRequest
	timer      *time.Timer
	generation uint64
}

func NewRelay(ttl time.Duration) *Relay {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	return &Relay{
		ttl:     ttl,
		pending: make(map[string]entry),
	}
}

func (r *Relay) Pending(req engineapi.PermissionRequest) int64 {
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

func (r *Relay) drop(id string, generation uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if current, ok := r.pending[id]; !ok || current.generation != generation {
		return
	}
	delete(r.pending, id)
}

func (r *Relay) Take(id string) (engineapi.PermissionRequest, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.pending[id]
	if !ok {
		return engineapi.PermissionRequest{}, false
	}
	e.timer.Stop()
	delete(r.pending, id)
	return e.req, true
}
