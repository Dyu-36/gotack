package recall

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
)

const (
	defaultResultLimit = 3
	maxResultLimit     = 10
	discoverScanLimit  = 300
)

type Store struct {
	dataDir       string
	indexDir      string
	log           *slog.Logger
	mu            sync.Mutex
	index         *sql.DB
	roleAvailable bool
}

func OpenStore(dataDir, indexDir string, log *slog.Logger) *Store {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Store{dataDir: dataDir, indexDir: indexDir, log: log}
}

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

func (s *Store) Sync(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.syncLocked(ctx)
}

func (s *Store) syncLocked(ctx context.Context) error {
	source, err := openSource(ctx, s.dataDir)
	if err != nil {
		return err
	}
	defer func() { _ = source.Close() }()
	s.roleAvailable = source.schema.messageHas("role")
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
	// Keep deletion reconciliation; speed must not resurrect deleted history.
	sessionIDs, err := source.SessionIDs(ctx)
	if err != nil {
		return err
	}
	messageIDs, err := source.MessageIDs(ctx)
	if err != nil {
		return err
	}
	if err := ingestSessions(ctx, index, sessions); err != nil {
		return err
	}
	if err := ingestMessages(ctx, index, messages); err != nil {
		return err
	}
	if err := reconcileIndex(ctx, index, sessionIDs, messageIDs); err != nil {
		return err
	}
	return s.advanceWatermarks(ctx, index, sessions, messages)
}

func (s *Store) watermark(ctx context.Context, index *sql.DB, key string) int64 {
	raw, err := indexMeta(ctx, index, key)
	if err != nil {
		s.log.Warn("recall: watermark unreadable, rescanning", "key", key, "err", err)
		return 0
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if raw != "" && err != nil {
		s.log.Warn("recall: watermark unparseable, rescanning", "key", key)
		return 0
	}
	return value
}

func (s *Store) advanceWatermarks(ctx context.Context, index *sql.DB, sessions []SourceSession, messages []SourceMessage) error {
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

func (s *Store) ensureIndexLocked() (*sql.DB, error) {
	if s.index != nil {
		return s.index, nil
	}
	index, err := openIndex(s.indexDir)
	if err != nil {
		return nil, err
	}
	// Apply after openIndex has upgraded legacy lineage columns. No rebuild.
	for _, statement := range []string{
		"CREATE INDEX IF NOT EXISTS idx_recall_session_time ON messages(session_id, created_at, id)",
		"CREATE INDEX IF NOT EXISTS idx_recall_session_parent ON sessions(parent_session_id)",
	} {
		if _, err := index.Exec(statement); err != nil {
			_ = index.Close()
			return nil, fmt.Errorf("create recall lookup index: %w", err)
		}
	}
	s.index = index
	return index, nil
}

type SortOrder string

const (
	SortRelevance SortOrder = ""
	SortNewest    SortOrder = "newest"
	SortOldest    SortOrder = "oldest"
)

func parseSortOrder(value string) SortOrder {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(SortNewest):
		return SortNewest
	case string(SortOldest):
		return SortOldest
	default:
		return SortRelevance
	}
}

func orderClause(order SortOrder) string {
	switch order {
	case SortNewest:
		return "m.created_at DESC, m.id DESC"
	case SortOldest:
		return "m.created_at ASC, m.id ASC"
	default:
		return "messages_fts.rank"
	}
}

type Detail string

const (
	DetailBrief    Detail = "brief"
	DetailAdaptive Detail = "adaptive"
	DetailFull     Detail = "full"
)

func parseDetail(value string) Detail {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(DetailFull):
		return DetailFull
	case string(DetailAdaptive):
		return DetailAdaptive
	default:
		return DetailBrief
	}
}

type SearchOptions struct {
	Query            string
	Roles            []string
	Limit            int
	Sort             SortOrder
	Detail           Detail
	ExcludeSessionID string
}

func (s *Store) SearchWithOptions(ctx context.Context, opts SearchOptions) ([]DiscoveryResult, error) {
	if len(opts.Query) > 2048 {
		return nil, fmt.Errorf("recall query is too long; use a few names or keywords")
	}
	match, err := buildMatch(opts.Query)
	if err != nil {
		return nil, err
	}
	limit := clampResultLimit(opts.Limit)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.syncLocked(ctx); err != nil {
		return nil, err
	}
	roles := normalizeRoles(opts.Roles)
	if len(opts.Roles) == 0 && !s.roleAvailable {
		roles = nil
	}
	hits, err := search(ctx, s.index, match, orderClause(opts.Sort), roles, discoverScanLimit, opts.ExcludeSessionID)
	if err != nil {
		return nil, err
	}
	results := make([]DiscoveryResult, 0, limit)
	seen := make(map[string]struct{}, limit)
	budget := newContentBudget(maxResponseContentBytes)
	for _, hit := range hits {
		if _, exists := seen[hit.sessionID]; exists {
			continue
		}
		seen[hit.sessionID] = struct{}{}
		var result DiscoveryResult
		if opts.Detail == DetailBrief {
			result = DiscoveryResult{
				SessionID: hit.sessionID, Title: clipUTF8(hit.title, 256), MatchedRole: hit.role,
				MatchMessageID: hit.messageID, Snippet: clipUTF8(hit.snippet, 768), Detail: DetailBrief,
				Messages: []Message{}, BookendStart: []Message{}, BookendEnd: []Message{},
			}
		} else {
			full := opts.Detail == DetailFull || len(results) == 0
			result, err = hydrateDiscovery(ctx, s.index, hit, full, budget)
			if err != nil {
				return nil, err
			}
		}
		results = append(results, result)
		if len(results) == limit {
			break
		}
	}
	return results, nil
}

// Preserve the programmatic API; the MCP tool explicitly selects brief mode.
func (s *Store) Search(ctx context.Context, query string, limit int) ([]DiscoveryResult, error) {
	return s.SearchWithOptions(ctx, SearchOptions{Query: query, Limit: limit, Detail: DetailAdaptive})
}

func clampResultLimit(limit int) int {
	if limit <= 0 {
		return defaultResultLimit
	}
	if limit > maxResultLimit {
		return maxResultLimit
	}
	return limit
}

func normalizeRoles(values []string) []string {
	if len(values) == 0 {
		return []string{"user", "assistant"}
	}
	seen := make(map[string]struct{}, len(values))
	roles := make([]string, 0, len(values))
	for _, value := range values {
		role := strings.ToLower(strings.TrimSpace(value))
		switch role {
		case "user", "assistant", "tool":
			if _, exists := seen[role]; !exists {
				seen[role] = struct{}{}
				roles = append(roles, role)
			}
		}
	}
	if len(roles) == 0 {
		return []string{"user", "assistant"}
	}
	return roles
}
