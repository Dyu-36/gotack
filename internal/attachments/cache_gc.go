package attachments

import (
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/Dyu-36/gotack/internal/appconfig"
)

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
