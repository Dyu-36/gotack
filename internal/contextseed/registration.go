package contextseed

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Dyu-36/gotack/internal/crushapi"
)

const globalContextPathsKey = "options.global_context_paths"

// Registrar owns the transactional registration state for prompt-context
// snapshots. Keeping leases here prevents the desktop host from having to know
// how generations are pinned, rolled back, or released.
type Registrar struct {
	seeder *Seeder
	log    *slog.Logger

	mu      sync.Mutex
	leases  map[string]*SnapshotLease
	pending map[string][]*SnapshotLease
}

func NewRegistrar(seeder *Seeder, log *slog.Logger) *Registrar {
	return &Registrar{seeder: seeder, log: log}
}

func (r *Registrar) Seeder() *Seeder {
	if r == nil {
		return nil
	}
	return r.seeder
}

func (r *Registrar) Register(base context.Context, api *crushapi.Client, workspaceID string) {
	if r == nil || r.seeder == nil || api == nil || workspaceID == "" {
		return
	}
	if base == nil {
		base = context.Background()
	}
	ctx, cancel := context.WithTimeout(base, 10*time.Second)
	defer cancel()

	if info, err := os.Stat(r.seeder.ContextDir()); err != nil || !info.IsDir() {
		r.Clear(ctx, api, workspaceID)
		return
	}
	previousPaths, transactionLease, previousErr := r.capture(ctx, api, workspaceID)
	if previousErr != nil {
		r.warn("context: failed to capture current registration; refusing mutation", "err", previousErr)
		return
	}
	dir, lease, snapshotErr := r.seeder.BuildPromptSnapshotLease()
	if snapshotErr != nil {
		if transactionLease != nil {
			r.restoreAcknowledged(workspaceID, transactionLease)
		}
		r.warn("context prompt snapshot failed; keeping committed revision", "err", snapshotErr)
		return
	}
	if err := api.SetConfigField(ctx, workspaceID, crushapi.ConfigScopeWorkspace, globalContextPathsKey, []string{dir}); err != nil {
		_ = lease.Release()
		if transactionLease != nil {
			r.restoreAcknowledged(workspaceID, transactionLease)
		}
		r.warn("context path registration failed", "err", err)
		return
	}

	if err := api.RefreshPromptContext(ctx, workspaceID); err != nil {
		rollbackErr := r.restoreRegistration(ctx, api, workspaceID, previousPaths)
		if rollbackErr != nil {
			r.holdPending(workspaceID, lease)
			if transactionLease != nil {
				r.restoreAcknowledged(workspaceID, transactionLease)
			}
		} else {
			_ = lease.Release()
			if transactionLease != nil {
				r.replace(workspaceID, transactionLease)
			}
		}
		r.warn("context prompt refresh failed; restored prior registration", "err", err, "rollback_err", rollbackErr)
		return
	}
	if transactionLease != nil {
		_ = transactionLease.Release()
	}
	r.replace(workspaceID, lease)
	if err := r.seeder.PrunePromptSnapshotsChecked(dir); err != nil {
		r.warn("context snapshot prune failed", "err", err)
	}
}

func (r *Registrar) Clear(ctx context.Context, api *crushapi.Client, workspaceID string) {
	if r == nil || r.seeder == nil || api == nil || workspaceID == "" {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	previousPaths, transactionLease, previousErr := r.capture(ctx, api, workspaceID)
	if previousErr != nil {
		r.warn("context: failed to capture current registration before removal", "err", previousErr)
		return
	}
	if err := api.RemoveConfigField(ctx, workspaceID, crushapi.ConfigScopeWorkspace, globalContextPathsKey); err != nil {
		if transactionLease != nil {
			r.restoreAcknowledged(workspaceID, transactionLease)
		}
		r.warn("context path removal failed", "err", err)
		return
	}
	if err := api.RefreshPromptContext(ctx, workspaceID); err != nil {
		rollbackErr := r.restoreRegistration(ctx, api, workspaceID, previousPaths)
		if transactionLease != nil {
			if rollbackErr == nil {
				r.replace(workspaceID, transactionLease)
			} else {
				r.restoreAcknowledged(workspaceID, transactionLease)
			}
		}
		r.warn("context prompt refresh after removal failed; restored prior registration", "err", err, "rollback_err", rollbackErr)
		return
	}
	if transactionLease != nil {
		_ = transactionLease.Release()
	}
	r.replace(workspaceID, nil)
}

func (r *Registrar) capture(ctx context.Context, api *crushapi.Client, workspaceID string) ([]string, *SnapshotLease, error) {
	cfg, err := api.GetWorkspaceConfig(ctx, workspaceID)
	if err != nil {
		return nil, nil, err
	}
	var paths []string
	if cfg.Options != nil {
		paths = append(paths, cfg.Options.GlobalContextPaths...)
	}
	if len(paths) != 1 || r.seeder == nil {
		return paths, nil, nil
	}
	path := filepath.Clean(paths[0])
	if current := r.LeaseGeneration(workspaceID); current != "" && filepath.Clean(current) == path {
		return paths, nil, nil
	}
	root, err := filepath.Abs(r.seeder.PromptContextRoot())
	if err != nil {
		return nil, nil, err
	}
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return nil, nil, err
	}
	relative, err := filepath.Rel(root, absolutePath)
	if err != nil || filepath.Dir(relative) != "." {
		return paths, nil, nil
	}
	lease, err := r.seeder.AcquireSnapshotLease(path)
	if err != nil {
		return nil, nil, err
	}
	return paths, lease, nil
}

func (r *Registrar) restoreRegistration(ctx context.Context, api *crushapi.Client, workspaceID string, paths []string) error {
	if len(paths) == 0 {
		return api.RemoveConfigField(ctx, workspaceID, crushapi.ConfigScopeWorkspace, globalContextPathsKey)
	}
	return api.SetConfigField(ctx, workspaceID, crushapi.ConfigScopeWorkspace, globalContextPathsKey, append([]string(nil), paths...))
}

func (r *Registrar) LeaseGeneration(workspaceID string) string {
	if r == nil {
		return ""
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if lease := r.leases[workspaceID]; lease != nil {
		return lease.Generation()
	}
	return ""
}

func (r *Registrar) holdPending(workspaceID string, lease *SnapshotLease) {
	if lease == nil {
		return
	}
	r.mu.Lock()
	if r.pending == nil {
		r.pending = make(map[string][]*SnapshotLease)
	}
	r.pending[workspaceID] = append(r.pending[workspaceID], lease)
	r.mu.Unlock()
}

func (r *Registrar) restoreAcknowledged(workspaceID string, lease *SnapshotLease) {
	if lease == nil {
		return
	}
	r.mu.Lock()
	if r.leases == nil {
		r.leases = make(map[string]*SnapshotLease)
	}
	old := r.leases[workspaceID]
	r.leases[workspaceID] = lease
	r.mu.Unlock()
	if old != nil && old != lease {
		_ = old.Release()
	}
}

func (r *Registrar) replace(workspaceID string, lease *SnapshotLease) {
	r.mu.Lock()
	if r.leases == nil {
		r.leases = make(map[string]*SnapshotLease)
	}
	old := r.leases[workspaceID]
	pending := r.pending[workspaceID]
	delete(r.pending, workspaceID)
	if lease == nil {
		delete(r.leases, workspaceID)
	} else {
		r.leases[workspaceID] = lease
	}
	r.mu.Unlock()

	if old != nil && old != lease {
		_ = old.Release()
	}
	for _, pendingLease := range pending {
		if pendingLease != nil && pendingLease != lease && pendingLease != old {
			_ = pendingLease.Release()
		}
	}
}

func (r *Registrar) ReleaseAll() {
	if r == nil {
		return
	}
	r.mu.Lock()
	leases := r.leases
	pending := r.pending
	r.leases = nil
	r.pending = nil
	r.mu.Unlock()

	for _, lease := range leases {
		if lease != nil {
			_ = lease.Release()
		}
	}
	for _, group := range pending {
		for _, lease := range group {
			if lease != nil {
				_ = lease.Release()
			}
		}
	}
}

func (r *Registrar) warn(message string, args ...any) {
	if r != nil && r.log != nil {
		r.log.Warn(message, args...)
	}
}
