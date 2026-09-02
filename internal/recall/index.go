package recall

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// indexFileName is the gotack-owned FTS5 database inside the index dir. It
// must never live inside crush.db's data dir: the engine schema is
// goose-managed and an unmanaged FTS table there would collide with a future
// upstream migration.
const indexFileName = "recall.db"

// Watermark keys in recall_meta. Values are Unix milliseconds, mirroring the
// engine's updated_at columns.
const (
	metaMessageWatermark = "watermark.messages"
	metaSessionWatermark = "watermark.sessions"
)

// indexSchema creates the recall index. messages_fts is a plain FTS5 table
// with unindexed id columns so re-ingestion can replace one message's row.
const indexSchema = `
CREATE TABLE IF NOT EXISTS recall_meta (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    parent_session_id TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL DEFAULT '',
    updated_at INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS messages (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT '',
    is_summary INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL DEFAULT 0,
    updated_at INTEGER NOT NULL DEFAULT 0,
    text TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_recall_messages_updated ON messages (updated_at);
CREATE VIRTUAL TABLE IF NOT EXISTS messages_fts USING fts5 (
    text,
    message_id UNINDEXED,
    session_id UNINDEXED,
    tokenize = 'unicode61'
);
`

// openIndex opens (creating if needed) the recall index database at
// {dir}/recall.db. One pooled connection keeps writes serialized.
func openIndex(dir string) (*sql.DB, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create recall index dir %q: %w", dir, err)
	}
	path := filepath.Join(dir, indexFileName)
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?_txlock=immediate", path))
	if err != nil {
		return nil, fmt.Errorf("open recall index %q: %w", path, err)
	}
	db.SetMaxOpenConns(1)
	// WAL survives an interrupted ingestion better than the rollback
	// journal and lets a future reader overlap a sync.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set recall index journal mode: %w", err)
	}
	if _, err := db.Exec(indexSchema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initialize recall index schema: %w", err)
	}
	// Existing derived indexes predate lineage metadata. Add the one optional
	// column in place; the index remains rebuildable and this migration never
	// touches the read-only Crush source. The CREATE TABLE above handles new
	// indexes; older ones need this additive migration.
	if _, err := db.Exec("ALTER TABLE sessions ADD COLUMN parent_session_id TEXT NOT NULL DEFAULT ''"); err != nil &&
		!strings.Contains(strings.ToLower(err.Error()), "duplicate column name") {
		_ = db.Close()
		return nil, fmt.Errorf("migrate recall lineage column: %w", err)
	}
	return db, nil
}

// removeIndexFiles deletes the index database and its journal sidecars so a
// rebuild starts from nothing. Missing files are not an error.
func removeIndexFiles(dir string) error {
	for _, name := range []string{
		indexFileName,
		indexFileName + "-wal",
		indexFileName + "-shm",
		indexFileName + "-journal",
	} {
		err := os.Remove(filepath.Join(dir, name))
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove recall index file %q: %w", name, err)
		}
	}
	return nil
}

// meta returns the stored value for key, "" when unset.
func indexMeta(ctx context.Context, db *sql.DB, key string) (string, error) {
	var value string
	err := db.QueryRowContext(ctx, "SELECT value FROM recall_meta WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read recall meta %q: %w", key, err)
	}
	return value, nil
}

// setMeta upserts one meta row.
func setMeta(ctx context.Context, db *sql.DB, key, value string) error {
	_, err := db.ExecContext(ctx,
		"INSERT INTO recall_meta (key, value) VALUES (?, ?)"+
			" ON CONFLICT (key) DO UPDATE SET value = excluded.value",
		key, value)
	if err != nil {
		return fmt.Errorf("write recall meta %q: %w", key, err)
	}
	return nil
}

// ingestSessions upserts session rows (id, title, updated_at) in one
// transaction.
func ingestSessions(ctx context.Context, db *sql.DB, sessions []SourceSession) error {
	if len(sessions) == 0 {
		return nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin session ingest: %w", err)
	}
	stmt, err := tx.PrepareContext(ctx,
		"INSERT INTO sessions (id, parent_session_id, title, updated_at) VALUES (?, ?, ?, ?)"+
			" ON CONFLICT (id) DO UPDATE SET parent_session_id = excluded.parent_session_id,"+
			" title = excluded.title, updated_at = excluded.updated_at")
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("prepare session upsert: %w", err)
	}
	defer stmt.Close()
	for _, session := range sessions {
		if _, err := stmt.ExecContext(ctx, session.ID, session.ParentSessionID, session.Title, session.UpdatedAt); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("upsert session %q: %w", session.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit session ingest: %w", err)
	}
	return nil
}

// ingestMessages upserts message rows and rebuilds their FTS entries in one
// transaction. Text is extracted defensively from the parts JSON column.
func ingestMessages(ctx context.Context, db *sql.DB, messages []SourceMessage) error {
	if len(messages) == 0 {
		return nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin message ingest: %w", err)
	}
	upsert, err := tx.PrepareContext(ctx,
		"INSERT INTO messages (id, session_id, role, is_summary, created_at, updated_at, text)"+
			" VALUES (?, ?, ?, ?, ?, ?, ?)"+
			" ON CONFLICT (id) DO UPDATE SET"+
			" session_id = excluded.session_id, role = excluded.role,"+
			" is_summary = excluded.is_summary, created_at = excluded.created_at,"+
			" updated_at = excluded.updated_at, text = excluded.text")
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("prepare message upsert: %w", err)
	}
	defer upsert.Close()
	deleteFTS, err := tx.PrepareContext(ctx, "DELETE FROM messages_fts WHERE message_id = ?")
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("prepare fts delete: %w", err)
	}
	defer deleteFTS.Close()
	insertFTS, err := tx.PrepareContext(ctx,
		"INSERT INTO messages_fts (text, message_id, session_id) VALUES (?, ?, ?)")
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("prepare fts insert: %w", err)
	}
	defer insertFTS.Close()

	for _, message := range messages {
		text := extractPartsText(message.Parts, message.Role)
		isSummary := 0
		if message.IsSummary {
			isSummary = 1
		}
		if _, err := upsert.ExecContext(ctx, message.ID, message.SessionID, message.Role,
			isSummary, message.CreatedAt, message.UpdatedAt, text); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("upsert message %q: %w", message.ID, err)
		}
		if _, err := deleteFTS.ExecContext(ctx, message.ID); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("delete fts for message %q: %w", message.ID, err)
		}
		if text == "" {
			continue
		}
		if _, err := insertFTS.ExecContext(ctx, text, message.ID, message.SessionID); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("insert fts for message %q: %w", message.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit message ingest: %w", err)
	}
	return nil
}

// reconcileIndex removes rows whose source objects were deleted from
// crush.db. The identity snapshot and all derived deletes commit together.
func reconcileIndex(ctx context.Context, db *sql.DB, sessionIDs, messageIDs []string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin recall reconciliation: %w", err)
	}
	rollback := func(err error) error {
		_ = tx.Rollback()
		return err
	}
	for _, statement := range []string{
		"CREATE TEMP TABLE IF NOT EXISTS seen_session_ids (id TEXT PRIMARY KEY)",
		"CREATE TEMP TABLE IF NOT EXISTS seen_message_ids (id TEXT PRIMARY KEY)",
		"DELETE FROM seen_session_ids",
		"DELETE FROM seen_message_ids",
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return rollback(fmt.Errorf("prepare recall reconciliation: %w", err))
		}
	}
	if err := insertSeenIDs(ctx, tx, "seen_session_ids", sessionIDs); err != nil {
		return rollback(err)
	}
	if err := insertSeenIDs(ctx, tx, "seen_message_ids", messageIDs); err != nil {
		return rollback(err)
	}
	for _, statement := range []string{
		"DELETE FROM messages_fts WHERE message_id NOT IN (SELECT id FROM seen_message_ids)",
		"DELETE FROM messages WHERE id NOT IN (SELECT id FROM seen_message_ids)",
		"DELETE FROM sessions WHERE id NOT IN (SELECT id FROM seen_session_ids)",
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return rollback(fmt.Errorf("reconcile recall index: %w", err))
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit recall reconciliation: %w", err)
	}
	return nil
}

func insertSeenIDs(ctx context.Context, tx *sql.Tx, table string, ids []string) error {
	stmt, err := tx.PrepareContext(ctx, "INSERT INTO "+table+" (id) VALUES (?)")
	if err != nil {
		return fmt.Errorf("prepare %s insert: %w", table, err)
	}
	defer stmt.Close()
	for _, id := range ids {
		if _, err := stmt.ExecContext(ctx, id); err != nil {
			return fmt.Errorf("insert %s id %q: %w", table, id, err)
		}
	}
	return nil
}

// searchResult is one FTS hit joined back to its message and session rows.
type searchResult struct {
	messageID string
	sessionID string
	title     string
	role      string
	snippet   string
}

// search runs an FTS5 query and filters summary messages out so generated
// summaries cannot crowd out real content. The MATCH expression
// comes from buildMatch and the ORDER BY body from orderClause, so neither
// the query nor the sort shape ever reaches SQL as raw user text.
func search(ctx context.Context, db *sql.DB, match, order string, roles []string, limit int, excludeSessionID string) ([]searchResult, error) {
	where := "messages_fts MATCH ? AND m.is_summary = 0"
	args := []any{match}
	if strings.TrimSpace(excludeSessionID) != "" {
		// Exclude the active session and every parent/child in its lineage. The
		// recursive CTE is evaluated only when a trusted id is supplied.
		where += ` AND m.session_id NOT IN (
			WITH RECURSIVE lineage(id) AS (
				SELECT ?
				UNION
				SELECT s.parent_session_id FROM sessions s JOIN lineage l ON s.id = l.id
				WHERE s.parent_session_id <> ''
				UNION
				SELECT s.id FROM sessions s JOIN lineage l ON s.parent_session_id = l.id
				WHERE s.id <> ''
			)
			SELECT id FROM lineage
		)`
		args = append(args, strings.TrimSpace(excludeSessionID))
	}
	if len(roles) > 0 {
		where += " AND m.role IN (" + strings.TrimSuffix(strings.Repeat("?,", len(roles)), ",") + ")"
		for _, role := range roles {
			args = append(args, role)
		}
	}
	args = append(args, limit)
	rows, err := db.QueryContext(ctx, `
SELECT m.id, m.session_id, COALESCE(s.title, ''), m.role,
       snippet(messages_fts, 0, '[', ']', ' … ', 24)
FROM messages_fts
JOIN messages m ON m.id = messages_fts.message_id
LEFT JOIN sessions s ON s.id = m.session_id
WHERE `+where+`
ORDER BY `+order+`
LIMIT ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("run recall search: %w", err)
	}
	defer rows.Close()

	var results []searchResult
	for rows.Next() {
		var result searchResult
		if err := rows.Scan(&result.messageID, &result.sessionID, &result.title,
			&result.role, &result.snippet); err != nil {
			return nil, fmt.Errorf("scan recall hit: %w", err)
		}
		results = append(results, result)
	}
	return results, rows.Err()
}
