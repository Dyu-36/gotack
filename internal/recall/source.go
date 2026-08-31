package recall

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// engineDBFile is the engine's SQLite database name inside its data dir.
const engineDBFile = "crush.db"

// Required columns per table. A missing table or required column is a schema
// mismatch that must surface as an error, per the Phase 3 plan: an empty
// result set would hide a broken pin bump.
var (
	requiredSessionColumns = []string{"id", "updated_at"}
	requiredMessageColumns = []string{"id", "session_id", "parts", "updated_at"}
)

// Optional columns: when one disappears in a Crush upgrade the ingestion
// degrades (logs and substitutes a neutral value) instead of failing.
var (
	optionalSessionColumns = []string{"title"}
	optionalMessageColumns = []string{"role", "created_at", "is_summary_message"}
)

// SourceSession is one session row read from crush.db.
type SourceSession struct {
	ID        string
	Title     string
	UpdatedAt int64
}

// SourceMessage is one message row read from crush.db.
type SourceMessage struct {
	ID        string
	SessionID string
	Role      string
	Parts     string
	IsSummary bool
	CreatedAt int64
	UpdatedAt int64
}

// sourceSchema records which columns actually exist so queries select
// explicitly present columns only. Crush migrations add columns over time
// (provider, is_summary_message, todos), and a future one may rename or drop
// others; SELECT * would break at runtime instead of degrading.
type sourceSchema struct {
	sessionColumns map[string]bool
	messageColumns map[string]bool
	// degraded holds human-readable notes about missing optional columns.
	degraded []string
}

func (s sourceSchema) sessionHas(column string) bool { return s.sessionColumns[column] }
func (s sourceSchema) messageHas(column string) bool { return s.messageColumns[column] }

// Source is a strictly read-only view of the engine's crush.db.
type Source struct {
	db     *sql.DB
	schema sourceSchema
}

// Degraded returns notes about optional columns missing from the engine
// schema. Callers log these so drift is visible instead of silent.
func (s *Source) Degraded() []string { return s.schema.degraded }

// openSource opens {dataDir}/crush.db strictly read-only. The DSN mirrors
// Crush's own openDBReadOnly (internal/db/connect_modernc.go): mode=ro plus
// _txlock=immediate, which upstream proves safe against a running WAL-mode
// engine. Columns are probed before the Source is returned, so schema drift
// is detected at open time rather than mid-query.
func openSource(ctx context.Context, dataDir string) (*Source, error) {
	dbPath := filepath.Join(dataDir, engineDBFile)
	if _, err := os.Stat(dbPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s", ErrSourceMissing, dbPath)
		}
		return nil, fmt.Errorf("stat engine database %q: %w", dbPath, err)
	}

	params := url.Values{}
	params.Set("_txlock", "immediate")
	params.Set("mode", "ro")
	dsn := fmt.Sprintf("file:%s?%s", dbPath, params.Encode())

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open engine database read-only: %w", err)
	}
	// A single connection keeps the read-only probe cheap and avoids pooling
	// handles against a database another process owns.
	db.SetMaxOpenConns(1)

	schema, err := probeSchema(ctx, db)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("read engine database %q: %w", dbPath, err)
	}
	return &Source{db: db, schema: schema}, nil
}

// Close releases the read-only database handle.
func (s *Source) Close() error {
	return s.db.Close()
}

// probeSchema reads PRAGMA table_info for the two depended-on tables. A
// missing table or required column wraps ErrSchemaMismatch; a missing
// optional column appends a degradation note.
func probeSchema(ctx context.Context, db *sql.DB) (sourceSchema, error) {
	var schema sourceSchema

	sessions, err := tableColumns(ctx, db, "sessions")
	if err != nil {
		return schema, err
	}
	if sessions == nil {
		return schema, fmt.Errorf("%w: table %q is missing from crush.db", ErrSchemaMismatch, "sessions")
	}
	messages, err := tableColumns(ctx, db, "messages")
	if err != nil {
		return schema, err
	}
	if messages == nil {
		return schema, fmt.Errorf("%w: table %q is missing from crush.db", ErrSchemaMismatch, "messages")
	}

	for _, column := range requiredSessionColumns {
		if !sessions[column] {
			return schema, fmt.Errorf("%w: sessions.%s is missing from crush.db", ErrSchemaMismatch, column)
		}
	}
	for _, column := range requiredMessageColumns {
		if !messages[column] {
			return schema, fmt.Errorf("%w: messages.%s is missing from crush.db", ErrSchemaMismatch, column)
		}
	}
	for _, column := range optionalSessionColumns {
		if !sessions[column] {
			schema.degraded = append(schema.degraded,
				fmt.Sprintf("sessions.%s is missing; recall continues without it", column))
		}
	}
	for _, column := range optionalMessageColumns {
		if !messages[column] {
			schema.degraded = append(schema.degraded,
				fmt.Sprintf("messages.%s is missing; recall continues without it", column))
		}
	}

	schema.sessionColumns = sessions
	schema.messageColumns = messages
	return schema, nil
}

// tableColumns returns the column names of table, or nil when the table does
// not exist (PRAGMA table_info yields zero rows for unknown tables).
func tableColumns(ctx context.Context, db *sql.DB, table string) (map[string]bool, error) {
	// The table argument is a package constant, never user input, so it is
	// safe to interpolate; PRAGMA does not accept bound identifiers.
	rows, err := db.QueryContext(ctx, "PRAGMA table_info("+table+")")
	if err != nil {
		return nil, fmt.Errorf("probe table %q: %w", table, err)
	}
	defer rows.Close()

	columns := map[string]bool{}
	for rows.Next() {
		var (
			cid       int
			name      string
			columnTyp string
			notNull   int
		)
		var dfltValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &columnTyp, &notNull, &dfltValue, &pk); err != nil {
			return nil, fmt.Errorf("scan table_info for %q: %w", table, err)
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate table_info for %q: %w", table, err)
	}
	if len(columns) == 0 {
		return nil, nil
	}
	return columns, nil
}

// SessionsSince returns sessions with updated_at >= since. The inclusive
// bound makes a repeated sync idempotent for rows sharing a watermark value;
// re-ingestion is an upsert.
func (s *Source) SessionsSince(ctx context.Context, since int64) ([]SourceSession, error) {
	titleExpr := "''"
	if s.schema.sessionHas("title") {
		titleExpr = "title"
	}
	query := fmt.Sprintf(
		"SELECT id, %s AS title, updated_at FROM sessions WHERE updated_at >= ? ORDER BY updated_at",
		titleExpr,
	)
	rows, err := s.db.QueryContext(ctx, query, since)
	if err != nil {
		return nil, fmt.Errorf("query sessions: %w", err)
	}
	defer rows.Close()

	var sessions []SourceSession
	for rows.Next() {
		var session SourceSession
		if err := rows.Scan(&session.ID, &session.Title, &session.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan session row: %w", err)
		}
		sessions = append(sessions, session)
	}
	return sessions, rows.Err()
}

// MessagesSince returns messages with updated_at >= since, selecting only
// columns proven present by probeSchema. Missing optional columns fall back
// to neutral expressions so the row shape stays stable.
func (s *Source) MessagesSince(ctx context.Context, since int64) ([]SourceMessage, error) {
	roleExpr := "''"
	if s.schema.messageHas("role") {
		roleExpr = "role"
	}
	createdExpr := "updated_at"
	if s.schema.messageHas("created_at") {
		createdExpr = "created_at"
	}
	summaryExpr := "0"
	if s.schema.messageHas("is_summary_message") {
		summaryExpr = "is_summary_message"
	}
	query := fmt.Sprintf(
		"SELECT id, session_id, %s AS role, parts, %s AS is_summary_message, %s AS created_at, updated_at"+
			" FROM messages WHERE updated_at >= ? ORDER BY updated_at",
		roleExpr, summaryExpr, createdExpr,
	)
	rows, err := s.db.QueryContext(ctx, query, since)
	if err != nil {
		return nil, fmt.Errorf("query messages: %w", err)
	}
	defer rows.Close()

	var messages []SourceMessage
	for rows.Next() {
		var message SourceMessage
		var isSummary int64
		if err := rows.Scan(&message.ID, &message.SessionID, &message.Role, &message.Parts,
			&isSummary, &message.CreatedAt, &message.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan message row: %w", err)
		}
		message.IsSummary = isSummary != 0
		messages = append(messages, message)
	}
	return messages, rows.Err()
}
