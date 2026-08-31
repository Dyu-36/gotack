package recall

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"
)

// defaultSearchLimit is applied when the caller omits limit; maxSearchLimit
// bounds it so one query cannot dump the index.
const (
	defaultSearchLimit = 10
	maxSearchLimit     = 50
)

// Result is one search hit with enough provenance for the assistant to cite
// where a memory came from: session identity plus the message timestamp.
type Result struct {
	SessionID    string `json:"session_id"`
	SessionTitle string `json:"session_title"`
	MessageID    string `json:"message_id"`
	Role         string `json:"role"`
	CreatedAtMS  int64  `json:"created_at_ms"`
	CreatedAt    string `json:"created_at"`
	IsSummary    bool   `json:"is_summary"`
	Snippet      string `json:"snippet"`
}

// Store couples the read-only engine source with the gotack-owned FTS index.
// One Store serves one data directory; all operations are serialized because
// the index connection is single-connection by construction.
type Store struct {
	dataDir  string
	indexDir string
	log      *slog.Logger

	// lockAttempts and lockBackoff override the probe policy; zero selects
	// the defaults. Tests shorten them.
	lockAttempts int
	lockBackoff  time.Duration

	mu    sync.Mutex
	index *sql.DB
}

// OpenStore creates a Store for {dataDir}/crush.db with its index at
// {indexDir}/recall.db. The source is opened lazily on each sync so a
// missing or migrated database is observed fresh every time.
func OpenStore(dataDir, indexDir string, log *slog.Logger) *Store {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Store{dataDir: dataDir, indexDir: indexDir, log: log}
}

// Close releases the index database handle.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.index == nil {
		return nil
	}
	err := s.index.Close()
	s.index = nil
	return err
}

// Rebuild drops the index database and resyncs from scratch. It is the
// documented recovery for a stale or corrupted recall.db.
func (s *Store) Rebuild(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.index != nil {
		if err := s.index.Close(); err != nil {
			return fmt.Errorf("close recall index before rebuild: %w", err)
		}
		s.index = nil
	}
	if err := removeIndexFiles(s.indexDir); err != nil {
		return err
	}
	return s.syncLocked(ctx)
}

// Sync ingests new sessions and messages from crush.db into the index.
// Lock contention with the running engine is logged and tolerated; schema
// drift on required objects is returned as an error, never swallowed.
func (s *Store) Sync(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.syncLocked(ctx)
}

func (s *Store) syncLocked(ctx context.Context) error {
	probe, err := probeDataDirLock(ctx, s.dataDir, s.lockAttempts, s.lockBackoff)
	switch {
	case err != nil:
		// A probe failure (permissions, vanished file mid-probe) does not
		// prove the database is unreadable; log and let the open decide.
		s.log.Warn("recall: data-dir lock probe failed; proceeding read-only", "err", err)
	case probe.Contended:
		s.log.Info("recall: crush.lock is held by the engine; reading crush.db read-only anyway",
			"lock", probe.LockPath, "attempts", probe.Attempts)
	default:
		s.log.Debug("recall: crush.lock free", "attempts", probe.Attempts)
	}

	source, err := openSource(ctx, s.dataDir)
	if err != nil {
		return err
	}
	defer func() { _ = source.Close() }()
	for _, note := range source.Degraded() {
		s.log.Warn("recall: schema drift in crush.db", "detail", note)
	}

	index, err := s.ensureIndexLocked()
	if err != nil {
		return err
	}

	sessions, err := source.SessionsSince(ctx, s.watermark(ctx, index, metaSessionWatermark))
	if err != nil {
		return err
	}
	messages, err := source.MessagesSince(ctx, s.watermark(ctx, index, metaMessageWatermark))
	if err != nil {
		return err
	}
	if err := ingestSessions(ctx, index, sessions); err != nil {
		return err
	}
	if err := ingestMessages(ctx, index, messages); err != nil {
		return err
	}
	return s.advanceWatermarks(ctx, index, sessions, messages)
}

// watermark reads a stored watermark; parse failures degrade to a full
// rescan rather than aborting the sync.
func (s *Store) watermark(ctx context.Context, index *sql.DB, key string) int64 {
	raw, err := indexMeta(ctx, index, key)
	if err != nil {
		s.log.Warn("recall: watermark unreadable, rescanning", "key", key, "err", err)
		return 0
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if raw != "" && err != nil {
		s.log.Warn("recall: watermark unparseable, rescanning", "key", key, "value", raw)
		return 0
	}
	return value
}

// advanceWatermarks stores the highest updated_at observed in this sync. The
// next sync uses an inclusive >= bound, so rows sharing the watermark are
// re-checked idempotently.
func (s *Store) advanceWatermarks(ctx context.Context, index *sql.DB,
	sessions []SourceSession, messages []SourceMessage) error {
	sessionWM := s.watermark(ctx, index, metaSessionWatermark)
	for _, session := range sessions {
		if session.UpdatedAt > sessionWM {
			sessionWM = session.UpdatedAt
		}
	}
	messageWM := s.watermark(ctx, index, metaMessageWatermark)
	for _, message := range messages {
		if message.UpdatedAt > messageWM {
			messageWM = message.UpdatedAt
		}
	}
	if err := setMeta(ctx, index, metaSessionWatermark, strconv.FormatInt(sessionWM, 10)); err != nil {
		return err
	}
	return setMeta(ctx, index, metaMessageWatermark, strconv.FormatInt(messageWM, 10))
}

// ensureIndexLocked opens the index on first use.
func (s *Store) ensureIndexLocked() (*sql.DB, error) {
	if s.index != nil {
		return s.index, nil
	}
	index, err := openIndex(s.indexDir)
	if err != nil {
		return nil, err
	}
	s.index = index
	return index, nil
}

// Search runs an incremental sync and then queries the FTS index. A sync
// failure (missing crush.db, schema drift) is returned instead of hiding
// behind empty results.
func (s *Store) Search(ctx context.Context, query string, limit int) ([]Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.syncLocked(ctx); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = defaultSearchLimit
	}
	if limit > maxSearchLimit {
		limit = maxSearchLimit
	}
	hits, err := search(ctx, s.index, query, limit)
	if err != nil {
		return nil, err
	}
	results := make([]Result, 0, len(hits))
	for _, hit := range hits {
		results = append(results, Result{
			SessionID:    hit.sessionID,
			SessionTitle: hit.title,
			MessageID:    hit.messageID,
			Role:         hit.role,
			CreatedAtMS:  hit.createdAtMS,
			CreatedAt:    time.UnixMilli(hit.createdAtMS).UTC().Format(time.RFC3339),
			IsSummary:    hit.isSummary,
			Snippet:      hit.snippet,
		})
	}
	return results, nil
}
