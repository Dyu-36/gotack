package recall

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
)

var ansiEscapeRE = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)

const (
	discoveryWindow         = 5
	bookendMessageCount     = 3
	defaultAroundWindow     = 5
	maxAroundWindow         = 20
	readHeadMessages        = 20
	readTailMessages        = 10
	maxMessageContentBytes  = 4000
	maxBookendContentBytes  = 1200
	maxResponseContentBytes = 24 * 1024
)

type Message struct {
	ID                   string `json:"id"`
	Role                 string `json:"role"`
	Content              string `json:"content"`
	Timestamp            int64  `json:"timestamp"`
	Anchor               bool   `json:"anchor,omitempty"`
	ContentTruncated     bool   `json:"content_truncated,omitempty"`
	OriginalContentBytes int    `json:"original_content_bytes,omitempty"`
}

type DiscoveryResult struct {
	SessionID      string    `json:"session_id"`
	Title          string    `json:"title,omitempty"`
	MatchedRole    string    `json:"matched_role"`
	MatchMessageID string    `json:"match_message_id"`
	Snippet        string    `json:"snippet"`
	BookendStart   []Message `json:"bookend_start"`
	Messages       []Message `json:"messages"`
	BookendEnd     []Message `json:"bookend_end"`
	MessagesBefore int       `json:"messages_before"`
	MessagesAfter  int       `json:"messages_after"`
	Detail         Detail    `json:"detail"`
}

type SessionMeta struct {
	Title string `json:"title,omitempty"`
}

type ReadResult struct {
	SessionID    string      `json:"session_id"`
	SessionMeta  SessionMeta `json:"session_meta"`
	MessageCount int         `json:"message_count"`
	Truncated    bool        `json:"truncated"`
	Messages     []Message   `json:"messages"`
}

type AroundResult struct {
	SessionID       string      `json:"session_id"`
	AroundMessageID string      `json:"around_message_id"`
	SessionMeta     SessionMeta `json:"session_meta"`
	Window          int         `json:"window"`
	Messages        []Message   `json:"messages"`
	MessagesBefore  int         `json:"messages_before"`
	MessagesAfter   int         `json:"messages_after"`
}

type SessionSummary struct {
	SessionID    string `json:"session_id"`
	Title        string `json:"title,omitempty"`
	LastActive   int64  `json:"last_active"`
	MessageCount int    `json:"message_count"`
	Preview      string `json:"preview"`
}

type messageRow struct {
	id        string
	role      string
	content   string
	createdAt int64
}

type contentBudget struct {
	remaining int
}

func newContentBudget(limit int) *contentBudget {
	return &contentBudget{remaining: limit}
}

func (b *contentBudget) shape(row messageRow, anchorID string, perMessageLimit int) Message {
	content := ansiEscapeRE.ReplaceAllString(row.content, "")
	original := len(content)
	limit := perMessageLimit
	if b.remaining < limit {
		limit = b.remaining
	}
	if limit < 0 {
		limit = 0
	}
	content = clipContent(content, limit)
	b.remaining -= len(content)
	message := Message{
		ID:        row.id,
		Role:      row.role,
		Content:   content,
		Timestamp: row.createdAt,
		Anchor:    row.id == anchorID,
	}
	if len(content) < original {
		message.ContentTruncated = true
		message.OriginalContentBytes = original
	}
	return message
}

func clipContent(text string, limit int) string {
	if limit <= 0 {
		return ""
	}
	return truncateBytes(text, limit)
}

func shapeRows(rows []messageRow, anchorID string, perMessageLimit int, budget *contentBudget) []Message {
	messages := make([]Message, 0, len(rows))
	for _, row := range rows {
		messages = append(messages, budget.shape(row, anchorID, perMessageLimit))
	}
	return messages
}

func hydrateDiscovery(ctx context.Context, db *sql.DB, hit searchResult, full bool,
	budget *contentBudget,
) (DiscoveryResult, error) {
	result := DiscoveryResult{
		SessionID:      hit.sessionID,
		Title:          hit.title,
		MatchedRole:    hit.role,
		MatchMessageID: hit.messageID,
		Snippet:        hit.snippet,
		BookendStart:   []Message{},
		BookendEnd:     []Message{},
		Detail:         DetailAdaptive,
	}
	if !full {
		rows, err := queryMessageRows(ctx, db,
			"SELECT id, role, text, created_at FROM messages WHERE session_id = ? AND id = ?",
			hit.sessionID, hit.messageID)
		if err != nil {
			return result, err
		}
		result.Messages = shapeRows(rows, hit.messageID, maxMessageContentBytes, budget)
		return result, nil
	}

	rows, before, after, err := loadAroundRows(ctx, db, hit.sessionID, hit.messageID, discoveryWindow)
	if err != nil {
		return result, err
	}
	start, err := loadBookend(ctx, db, hit.sessionID, false)
	if err != nil {
		return result, err
	}
	end, err := loadBookend(ctx, db, hit.sessionID, true)
	if err != nil {
		return result, err
	}
	result.Detail = DetailFull
	result.BookendStart = shapeRows(withoutCompactionSummaries(start), "", maxBookendContentBytes, budget)
	result.Messages = shapeRows(rows, hit.messageID, maxMessageContentBytes, budget)
	result.BookendEnd = shapeRows(withoutCompactionSummaries(end), "", maxBookendContentBytes, budget)
	result.MessagesBefore = before
	result.MessagesAfter = after
	return result, nil
}

func withoutCompactionSummaries(rows []messageRow) []messageRow {
	filtered := rows[:0]
	for _, row := range rows {
		content := strings.TrimSpace(row.content)
		if strings.HasPrefix(content, "[CONTEXT COMPACTION") ||
			strings.HasPrefix(content, "[CONTEXT SUMMARY]:") {
			continue
		}
		filtered = append(filtered, row)
	}
	return filtered
}

func (s *Store) Around(ctx context.Context, sessionID, anchorID string, window int) (AroundResult, error) {
	window = clampWindow(window)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.syncLocked(ctx); err != nil {
		return AroundResult{}, err
	}
	title, err := sessionTitle(ctx, s.index, sessionID)
	if err != nil {
		return AroundResult{}, err
	}
	rows, before, after, err := loadAroundRows(ctx, s.index, sessionID, anchorID, window)
	if err != nil {
		return AroundResult{}, err
	}
	if len(rows) == 0 {
		return AroundResult{}, fmt.Errorf("%w: %q is not in session %q", ErrUnknownMessage, anchorID, sessionID)
	}
	return AroundResult{
		SessionID:       sessionID,
		AroundMessageID: anchorID,
		SessionMeta:     SessionMeta{Title: title},
		Window:          window,
		Messages:        shapeRows(rows, anchorID, maxMessageContentBytes, newContentBudget(maxResponseContentBytes)),
		MessagesBefore:  before,
		MessagesAfter:   after,
	}, nil
}

func clampWindow(window int) int {
	if window <= 0 {
		return defaultAroundWindow
	}
	if window > maxAroundWindow {
		return maxAroundWindow
	}
	return window
}

func (s *Store) ReadSession(ctx context.Context, sessionID string) (ReadResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.syncLocked(ctx); err != nil {
		return ReadResult{}, err
	}
	title, err := sessionTitle(ctx, s.index, sessionID)
	if err != nil {
		return ReadResult{}, err
	}
	var count int
	if err := s.index.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM messages WHERE session_id = ?", sessionID).Scan(&count); err != nil {
		return ReadResult{}, fmt.Errorf("count session messages: %w", err)
	}
	rows, err := loadReadRows(ctx, s.index, sessionID, count)
	if err != nil {
		return ReadResult{}, err
	}
	return ReadResult{
		SessionID:    sessionID,
		SessionMeta:  SessionMeta{Title: title},
		MessageCount: count,
		Truncated:    count > readHeadMessages+readTailMessages,
		Messages:     shapeRows(rows, "", maxMessageContentBytes, newContentBudget(maxResponseContentBytes)),
	}, nil
}

func (s *Store) BrowseWithOptions(ctx context.Context, limit int, excludeSessionID string) ([]SessionSummary, error) {
	limit = clampResultLimit(limit)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.syncLocked(ctx); err != nil {
		return nil, err
	}
	where := ""
	args := make([]any, 0, 2)
	if strings.TrimSpace(excludeSessionID) != "" {
		where = `WHERE s.id NOT IN (
			WITH RECURSIVE lineage(id) AS (
				SELECT ?
				UNION
				SELECT ps.parent_session_id FROM sessions ps JOIN lineage l ON ps.id = l.id
				WHERE ps.parent_session_id <> ''
				UNION
				SELECT cs.id FROM sessions cs JOIN lineage l ON cs.parent_session_id = l.id
				WHERE cs.id <> ''
			)
			SELECT id FROM lineage
		)`
		args = append(args, strings.TrimSpace(excludeSessionID))
	}
	args = append(args, limit)
	rows, err := s.index.QueryContext(ctx, `
SELECT s.id, s.title, COALESCE(MAX(m.created_at), s.updated_at), COUNT(m.id),
       COALESCE((SELECT p.text FROM messages p
                 WHERE p.session_id = s.id AND p.is_summary = 0 AND p.text <> ''
                 ORDER BY p.created_at ASC, p.id ASC LIMIT 1), '')
FROM sessions s
LEFT JOIN messages m ON m.session_id = s.id AND m.is_summary = 0
`+where+`
GROUP BY s.id, s.title, s.updated_at
ORDER BY 3 DESC, s.id ASC
LIMIT ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("browse recall sessions: %w", err)
	}
	defer rows.Close()
	budget := newContentBudget(maxResponseContentBytes)
	results := make([]SessionSummary, 0, limit)
	for rows.Next() {
		var result SessionSummary
		var preview string
		if err := rows.Scan(&result.SessionID, &result.Title, &result.LastActive,
			&result.MessageCount, &preview); err != nil {
			return nil, fmt.Errorf("scan recall session: %w", err)
		}
		preview = strings.Join(strings.Fields(ansiEscapeRE.ReplaceAllString(preview, "")), " ")
		result.Preview = budget.shape(messageRow{content: preview}, "", maxBookendContentBytes).Content
		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read recall sessions: %w", err)
	}
	return results, nil
}

func loadAroundRows(ctx context.Context, db *sql.DB, sessionID, anchorID string,
	window int,
) ([]messageRow, int, int, error) {
	var anchor messageRow
	err := db.QueryRowContext(ctx,
		"SELECT id, role, text, created_at FROM messages WHERE session_id = ? AND id = ?",
		sessionID, anchorID).Scan(&anchor.id, &anchor.role, &anchor.content, &anchor.createdAt)
	if err == sql.ErrNoRows {
		return nil, 0, 0, nil
	}
	if err != nil {
		return nil, 0, 0, fmt.Errorf("load recall anchor: %w", err)
	}
	var before, after int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM messages WHERE session_id = ?
AND (created_at < ? OR (created_at = ? AND id < ?))`, sessionID, anchor.createdAt, anchor.createdAt, anchor.id).Scan(&before); err != nil {
		return nil, 0, 0, fmt.Errorf("count messages before anchor: %w", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM messages WHERE session_id = ?
AND (created_at > ? OR (created_at = ? AND id > ?))`, sessionID, anchor.createdAt, anchor.createdAt, anchor.id).Scan(&after); err != nil {
		return nil, 0, 0, fmt.Errorf("count messages after anchor: %w", err)
	}
	older, err := queryMessageRows(ctx, db, `SELECT id, role, text, created_at FROM messages
WHERE session_id = ? AND (created_at < ? OR (created_at = ? AND id < ?))
ORDER BY created_at DESC, id DESC LIMIT ?`, sessionID, anchor.createdAt, anchor.createdAt, anchor.id, window)
	if err != nil {
		return nil, 0, 0, err
	}
	newer, err := queryMessageRows(ctx, db, `SELECT id, role, text, created_at FROM messages
WHERE session_id = ? AND (created_at > ? OR (created_at = ? AND id > ?))
ORDER BY created_at ASC, id ASC LIMIT ?`, sessionID, anchor.createdAt, anchor.createdAt, anchor.id, window)
	if err != nil {
		return nil, 0, 0, err
	}
	reverseRows(older)
	rows := append(older, anchor)
	rows = append(rows, newer...)
	return rows, before, after, nil
}

func loadBookend(ctx context.Context, db *sql.DB, sessionID string, tail bool) ([]messageRow, error) {
	order := "ASC"
	if tail {
		order = "DESC"
	}
	rows, err := queryMessageRows(ctx, db,
		"SELECT id, role, text, created_at FROM messages WHERE session_id = ? ORDER BY created_at "+order+", id "+order+" LIMIT ?",
		sessionID, bookendMessageCount)
	if err == nil && tail {
		reverseRows(rows)
	}
	return rows, err
}

func loadReadRows(ctx context.Context, db *sql.DB, sessionID string, count int) ([]messageRow, error) {
	if count <= readHeadMessages+readTailMessages {
		return queryMessageRows(ctx, db,
			"SELECT id, role, text, created_at FROM messages WHERE session_id = ? ORDER BY created_at ASC, id ASC",
			sessionID)
	}
	head, err := queryMessageRows(ctx, db,
		"SELECT id, role, text, created_at FROM messages WHERE session_id = ? ORDER BY created_at ASC, id ASC LIMIT ?",
		sessionID, readHeadMessages)
	if err != nil {
		return nil, err
	}
	tail, err := queryMessageRows(ctx, db,
		"SELECT id, role, text, created_at FROM messages WHERE session_id = ? ORDER BY created_at DESC, id DESC LIMIT ?",
		sessionID, readTailMessages)
	if err != nil {
		return nil, err
	}
	reverseRows(tail)
	return append(head, tail...), nil
}

func queryMessageRows(ctx context.Context, db *sql.DB, query string, args ...any) ([]messageRow, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query recall messages: %w", err)
	}
	defer rows.Close()
	var messages []messageRow
	for rows.Next() {
		var message messageRow
		if err := rows.Scan(&message.id, &message.role, &message.content, &message.createdAt); err != nil {
			return nil, fmt.Errorf("scan recall message: %w", err)
		}
		messages = append(messages, message)
	}
	return messages, rows.Err()
}

func reverseRows(rows []messageRow) {
	for left, right := 0, len(rows)-1; left < right; left, right = left+1, right-1 {
		rows[left], rows[right] = rows[right], rows[left]
	}
}

func sessionTitle(ctx context.Context, db *sql.DB, sessionID string) (string, error) {
	var title string
	err := db.QueryRowContext(ctx, "SELECT title FROM sessions WHERE id = ?", sessionID).Scan(&title)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("%w: %q", ErrUnknownSession, sessionID)
	}
	if err != nil {
		return "", fmt.Errorf("look up session %q: %w", sessionID, err)
	}
	return title, nil
}
