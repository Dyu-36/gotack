package recall

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
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
		"INSERT INTO sessions (id, title, updated_at) VALUES (?, ?, ?)"+
			" ON CONFLICT (id) DO UPDATE SET title = excluded.title, updated_at = excluded.updated_at")
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("prepare session upsert: %w", err)
	}
	defer stmt.Close()
	for _, session := range sessions {
		if _, err := stmt.ExecContext(ctx, session.ID, session.Title, session.UpdatedAt); err != nil {
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
		text := extractPartsText(message.Parts)
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

// ftsMatch converts free-form user input into a safe FTS5 MATCH expression:
// every whitespace-separated token becomes a quoted phrase, so FTS operators
// and punctuation in the query cannot inject query syntax. Tokens without a
// letter or digit are dropped; nil is returned when nothing searchable
// remains.
func ftsMatch(query string) *string {
	var terms []string
	for _, token := range strings.Fields(query) {
		if !strings.ContainsFunc(token, func(r rune) bool {
			return unicode.IsLetter(r) || unicode.IsDigit(r)
		}) {
			continue
		}
		terms = append(terms, `"`+strings.ReplaceAll(token, `"`, `""`)+`"`)
	}
	if len(terms) == 0 {
		return nil
	}
	match := strings.Join(terms, " ")
	return &match
}

// searchResult is one FTS hit joined back to its message and session rows.
type searchResult struct {
	messageID   string
	sessionID   string
	title       string
	role        string
	isSummary   bool
	createdAtMS int64
	snippet     string
}

// search runs an FTS5 query ranked by bm25 and filters summary messages out,
// per the Phase 3 plan: summaries must not crowd out real content.
func search(ctx context.Context, db *sql.DB, query string, limit int) ([]searchResult, error) {
	match := ftsMatch(query)
	if match == nil {
		return nil, ErrInvalidQuery
	}
	rows, err := db.QueryContext(ctx, `
SELECT m.id, m.session_id, COALESCE(s.title, ''), m.role, m.is_summary, m.created_at,
       snippet(messages_fts, 0, '[', ']', ' … ', 24)
FROM messages_fts
JOIN messages m ON m.id = messages_fts.message_id
LEFT JOIN sessions s ON s.id = m.session_id
WHERE messages_fts MATCH ? AND m.is_summary = 0
ORDER BY messages_fts.rank
LIMIT ?`, *match, limit)
	if err != nil {
		return nil, fmt.Errorf("run recall search: %w", err)
	}
	defer rows.Close()

	var results []searchResult
	for rows.Next() {
		var result searchResult
		var isSummary int64
		if err := rows.Scan(&result.messageID, &result.sessionID, &result.title,
			&result.role, &isSummary, &result.createdAtMS, &result.snippet); err != nil {
			return nil, fmt.Errorf("scan recall hit: %w", err)
		}
		result.isSummary = isSummary != 0
		results = append(results, result)
	}
	return results, rows.Err()
}
