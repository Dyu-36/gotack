package attachments

import (
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/Dyu-36/gotack/internal/appconfig"
)

// cache_gc.go -- role: keep the attachment cache from growing forever.
//
// Every send used to copy the uploaded bytes under %APPDATA%/gotack/attachments
// and nothing ever removed them. Startup calls PruneCache exactly once; there is
// deliberately no background loop watching the directory (AGENTS.md rule 5).

// PruneCache trims the attachment cache: entries older than
// appconfig.AttachmentCacheTTL go first, then the oldest remaining entries until
// the total fits appconfig.AttachmentCacheBudget.
func PruneCache() {
	pruneCache(CacheDir(), appconfig.AttachmentCacheTTL, appconfig.AttachmentCacheBudget)
}

type cacheEntry struct {
	path     string
	size     int64
	modified time.Time
}

func pruneCache(dir string, ttl time.Duration, budget int64) {
	items, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-ttl)
	kept := make([]cacheEntry, 0, len(items))
	var total int64
	for _, item := range items {
		if !item.IsDir() {
			continue
		}
		entry := statCacheDir(filepath.Join(dir, item.Name()))
		if entry.modified.Before(cutoff) {
			_ = os.RemoveAll(entry.path)
			continue
		}
		kept = append(kept, entry)
		total += entry.size
	}
	if total <= budget {
		return
	}
	// Oldest first: every cache entry is only a copy of a file the user still
	// has, and the prompt keeps the path, so deleting one is never data loss.
	sort.Slice(kept, func(i, j int) bool { return kept[i].modified.Before(kept[j].modified) })
	for _, entry := range kept {
		if total <= budget {
			return
		}
		if err := os.RemoveAll(entry.path); err == nil {
			total -= entry.size
		}
	}
}

// statCacheDir sums one cache entry and reports its newest modification time.
func statCacheDir(path string) cacheEntry {
	entry := cacheEntry{path: path}
	_ = filepath.WalkDir(path, func(_ string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		info, statErr := d.Info()
		if statErr != nil {
			return nil
		}
		entry.size += info.Size()
		if info.ModTime().After(entry.modified) {
			entry.modified = info.ModTime()
		}
		return nil
	})
	return entry
}
