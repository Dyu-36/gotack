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

const engineDBFile = "crush.db"

var (
	requiredSessionColumns = []string{"id", "updated_at"}
	requiredMessageColumns = []string{"id", "session_id", "parts", "updated_at"}
)

var (
	optionalSessionColumns = []string{"title", "parent_session_id"}
	optionalMessageColumns = []string{"role", "created_at", "is_summary_message"}
)

type SourceSession struct {
	ID              string
	ParentSessionID string
	Title           string
	UpdatedAt       int64
}

type SourceMessage struct {
	ID        string
	SessionID string
	Role      string
	Parts     string
	IsSummary bool
	CreatedAt int64
	UpdatedAt int64
}

type sourceSchema struct {
	sessionColumns map[string]bool
	messageColumns map[string]bool

	degraded []string
}

func (s sourceSchema) sessionHas(column string) bool { return s.sessionColumns[column] }
func (s sourceSchema) messageHas(column string) bool { return s.messageColumns[column] }

type Source struct {
	db     *sql.DB
	schema sourceSchema
}

func (s *Source) Degraded() []string { return s.schema.degraded }

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

func (s *Source) Close() error {
	return s.db.Close()
}

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

func tableColumns(ctx context.Context, db *sql.DB, table string) (map[string]bool, error) {

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

func (s *Source) SessionsSince(ctx context.Context, since int64) ([]SourceSession, error) {
	titleExpr := "''"
	if s.schema.sessionHas("title") {
		titleExpr = "title"
	}
	parentExpr := "''"
	if s.schema.sessionHas("parent_session_id") {
		parentExpr = "COALESCE(parent_session_id, '')"
	}
	query := fmt.Sprintf(
		"SELECT id, %s AS parent_session_id, %s AS title, updated_at FROM sessions WHERE updated_at >= ? ORDER BY updated_at",
		parentExpr, titleExpr,
	)
	rows, err := s.db.QueryContext(ctx, query, since)
	if err != nil {
		return nil, fmt.Errorf("query sessions: %w", err)
	}
	defer rows.Close()

	var sessions []SourceSession
	for rows.Next() {
		var session SourceSession
		if err := rows.Scan(&session.ID, &session.ParentSessionID, &session.Title, &session.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan session row: %w", err)
		}
		sessions = append(sessions, session)
	}
	return sessions, rows.Err()
}

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

func (s *Source) SessionIDs(ctx context.Context) ([]string, error) {
	return sourceIDs(ctx, s.db, "sessions")
}

func (s *Source) MessageIDs(ctx context.Context) ([]string, error) {
	return sourceIDs(ctx, s.db, "messages")
}

func sourceIDs(ctx context.Context, db *sql.DB, table string) ([]string, error) {
	rows, err := db.QueryContext(ctx, "SELECT id FROM "+table+" ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("query %s ids: %w", table, err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan %s id: %w", table, err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read %s ids: %w", table, err)
	}
	return ids, nil
}
