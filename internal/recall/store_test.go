package recall

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The fixture schema mirrors third_party/crush/internal/db/migrations
// 20250424200609_initial.sql plus the later ALTER migrations, so drift tests
// can drop pieces of it.
const (
	fixtureSessionsTable = `
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    parent_session_id TEXT,
    title TEXT NOT NULL,
    message_count INTEGER NOT NULL DEFAULT 0,
    prompt_tokens INTEGER NOT NULL DEFAULT 0,
    completion_tokens INTEGER NOT NULL DEFAULT 0,
    cost REAL NOT NULL DEFAULT 0.0,
    updated_at INTEGER NOT NULL,
    created_at INTEGER NOT NULL
)`
	fixtureMessagesTable = `
CREATE TABLE messages (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    role TEXT NOT NULL,
    parts TEXT NOT NULL DEFAULT '[]',
    model TEXT,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    finished_at INTEGER,
    provider TEXT,
    is_summary_message INTEGER DEFAULT 0 NOT NULL
)`
)

// createFixture builds a crush-shaped crush.db in dir and returns dir.
// mutate, when non-nil, may reshape the schema for drift tests.
func createFixture(t *testing.T, dir string, mutate func(db *sql.DB)) string {
	t.Helper()
	db := openFixtureRW(t, dir)
	for _, statement := range []string{fixtureSessionsTable, fixtureMessagesTable} {
		if _, err := db.Exec(statement); err != nil {
			db.Close()
			t.Fatalf("create fixture table: %v", err)
		}
	}
	if mutate != nil {
		mutate(db)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close fixture: %v", err)
	}
	return dir
}

// openFixtureRW opens the fixture database read-write for seeding.
func openFixtureRW(t *testing.T, dir string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+filepath.Join(dir, engineDBFile))
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	return db
}

func seedSession(t *testing.T, db *sql.DB, id, title string, updatedAt int64) {
	t.Helper()
	_, err := db.Exec(
		"INSERT INTO sessions (id, title, updated_at, created_at) VALUES (?, ?, ?, ?)",
		id, title, updatedAt, updatedAt)
	if err != nil {
		t.Fatalf("seed session %s: %v", id, err)
	}
}

func seedMessage(t *testing.T, db *sql.DB, id, sessionID, role, parts string,
	createdAt, updatedAt int64,
) {
	t.Helper()
	_, err := db.Exec(
		"INSERT INTO messages (id, session_id, role, parts, created_at, updated_at)"+
			" VALUES (?, ?, ?, ?, ?, ?)",
		id, sessionID, role, parts, createdAt, updatedAt)
	if err != nil {
		t.Fatalf("seed message %s: %v", id, err)
	}
}

// standardFixture seeds two sessions: deploy work and small talk.
func standardFixture(t *testing.T, dir string) string {
	t.Helper()
	db := openFixtureRW(t, dir)
	defer db.Close()
	for _, statement := range []string{fixtureSessionsTable, fixtureMessagesTable} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("create fixture table: %v", err)
		}
	}
	seedSession(t, db, "sess-deploy", "Deploy fixes", 1_700_000_001_000)
	seedSession(t, db, "sess-chat", "Small talk", 1_700_000_002_000)
	seedMessage(t, db, "msg-user-1", "sess-deploy", "user",
		partsArray(partFixture(t, "text", map[string]string{"text": "please fix the kubernetes deployment pipeline"})),
		1_700_000_000_100, 1_700_000_000_100)
	seedMessage(t, db, "msg-asst-1", "sess-deploy", "assistant",
		partsArray(partFixture(t, "text", map[string]string{"text": "restarting the kubernetes pods fixed the rollout"})),
		1_700_000_000_200, 1_700_000_000_200)
	seedMessage(t, db, "msg-tool-1", "sess-deploy", "tool",
		partsArray(partFixture(t, "tool_result", map[string]string{"name": "exec", "content": "all pods healthy"})),
		1_700_000_000_300, 1_700_000_000_300)
	seedMessage(t, db, "msg-chat-1", "sess-chat", "user",
		partsArray(partFixture(t, "text", map[string]string{"text": "what did we order for lunch yesterday"})),
		1_700_000_001_500, 1_700_000_001_500)
	return dir
}

func newTestStore(t *testing.T, dataDir string) *Store {
	t.Helper()
	store := OpenStore(dataDir, t.TempDir(), nil)
	t.Cleanup(func() { _ = store.Close() })
	// Short lock policy keeps tests fast; retry semantics stay exercised.
	store.lockAttempts = 3
	store.lockBackoff = 5 * time.Millisecond
	return store
}

func TestSearchReturnsHitsWithSourceCitation(t *testing.T) {
	dataDir := standardFixture(t, t.TempDir())
	store := newTestStore(t, dataDir)

	results, err := store.Search(context.Background(), "kubernetes", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d hits, want 2: %+v", len(results), results)
	}
	first := results[0]
	if first.SessionID != "sess-deploy" || first.SessionTitle != "Deploy fixes" {
		t.Fatalf("citation missing session identity: %+v", first)
	}
	if first.Role != "user" && first.Role != "assistant" {
		t.Fatalf("citation missing role: %+v", first)
	}
	if first.CreatedAtMS == 0 || !strings.HasPrefix(first.CreatedAt, "2023-11-14T") {
		t.Fatalf("citation missing timestamp: %+v", first)
	}
	if !strings.Contains(first.Snippet, "kubernetes") {
		t.Fatalf("snippet does not cite the match: %+v", first)
	}

	// A term unique to the other session must resolve there instead.
	other, err := store.Search(context.Background(), "lunch", 10)
	if err != nil {
		t.Fatalf("Search lunch: %v", err)
	}
	if len(other) != 1 || other[0].SessionID != "sess-chat" {
		t.Fatalf("lunch hits = %+v", other)
	}

	// No match is an empty result, not an error.
	none, err := store.Search(context.Background(), "zebra", 10)
	if err != nil {
		t.Fatalf("Search zebra: %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("zebra hits = %+v", none)
	}
}

func TestSearchFiltersSummaryMessages(t *testing.T) {
	dataDir := standardFixture(t, t.TempDir())
	db := openFixtureRW(t, dataDir)
	_, err := db.Exec(
		"INSERT INTO messages (id, session_id, role, parts, created_at, updated_at, is_summary_message)"+
			" VALUES ('msg-summary', 'sess-chat', 'assistant', ?, 1700000001600, 1700000001600, 1)",
		partsArray(partFixture(t, "text", map[string]string{"text": "lunch summary of the conversation"})))
	db.Close()
	if err != nil {
		t.Fatalf("seed summary: %v", err)
	}

	store := newTestStore(t, dataDir)
	results, err := store.Search(context.Background(), "lunch", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	for _, result := range results {
		if result.MessageID == "msg-summary" || result.IsSummary {
			t.Fatalf("summary message leaked into results: %+v", result)
		}
	}
	if len(results) != 1 {
		t.Fatalf("got %d hits, want the 1 non-summary: %+v", len(results), results)
	}
}

func TestIncrementalSyncByWatermark(t *testing.T) {
	dataDir := standardFixture(t, t.TempDir())
	store := newTestStore(t, dataDir)

	if _, err := store.Search(context.Background(), "kubernetes", 10); err != nil {
		t.Fatalf("first sync: %v", err)
	}

	// A new message arrives after the first sync.
	db := openFixtureRW(t, dataDir)
	seedMessage(t, db, "msg-late", "sess-deploy", "assistant",
		partsArray(partFixture(t, "text", map[string]string{"text": "the helm chart upgrade is scheduled"})),
		1_700_000_003_000, 1_700_000_003_000)
	if err := db.Close(); err != nil {
		t.Fatalf("close reseed: %v", err)
	}

	results, err := store.Search(context.Background(), "helm", 10)
	if err != nil {
		t.Fatalf("second sync: %v", err)
	}
	if len(results) != 1 || results[0].MessageID != "msg-late" {
		t.Fatalf("incremental sync missed the new message: %+v", results)
	}

	// Re-syncing must not duplicate rows: the inclusive watermark re-reads
	// the boundary row and upserts it.
	if _, err := store.Search(context.Background(), "helm", 10); err != nil {
		t.Fatalf("third sync: %v", err)
	}
	results, err = store.Search(context.Background(), "helm", 10)
	if err != nil {
		t.Fatalf("fourth search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("watermark re-ingest duplicated rows: %+v", results)
	}
}

func TestMissingOptionalColumnsDegradeGracefully(t *testing.T) {
	dir := t.TempDir()
	createFixture(t, dir, func(db *sql.DB) {
		seedSession(t, db, "sess-a", "Drift session", 1_700_000_000_000)
		seedMessage(t, db, "msg-a", "sess-a", "user",
			partsArray(partFixture(t, "text", map[string]string{"text": "remember the staging credentials"})),
			1_700_000_000_000, 1_700_000_000_000)
		for _, statement := range []string{
			"ALTER TABLE sessions DROP COLUMN title",
			"ALTER TABLE messages DROP COLUMN role",
			"ALTER TABLE messages DROP COLUMN created_at",
			"ALTER TABLE messages DROP COLUMN is_summary_message",
		} {
			if _, err := db.Exec(statement); err != nil {
				t.Fatalf("drift mutation %q: %v", statement, err)
			}
		}
	})

	var logBuf bytes.Buffer
	store := OpenStore(dir, t.TempDir(), slog.New(slog.NewTextHandler(&logBuf, nil)))
	defer store.Close()

	results, err := store.Search(context.Background(), "staging credentials", 10)
	if err != nil {
		t.Fatalf("degraded search must not fail: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("degraded search hits = %+v", results)
	}
	if results[0].SessionTitle != "" || results[0].Role != "" {
		t.Fatalf("degraded fields must be neutral: %+v", results[0])
	}
	if results[0].CreatedAtMS != 1_700_000_000_000 {
		t.Fatalf("created_at must fall back to updated_at: %+v", results[0])
	}
	if !strings.Contains(logBuf.String(), "schema drift") {
		t.Fatalf("degradation was not logged: %q", logBuf.String())
	}
}

func TestSchemaMismatchSurfacesAsError(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(db *sql.DB)
	}{
		{"messages table missing", func(db *sql.DB) {
			if _, err := db.Exec("DROP TABLE messages"); err != nil {
				t.Fatalf("drop messages: %v", err)
			}
		}},
		{"sessions table missing", func(db *sql.DB) {
			if _, err := db.Exec("DROP TABLE sessions"); err != nil {
				t.Fatalf("drop sessions: %v", err)
			}
		}},
		{"required column parts missing", func(db *sql.DB) {
			if _, err := db.Exec("ALTER TABLE messages DROP COLUMN parts"); err != nil {
				t.Fatalf("drop parts: %v", err)
			}
		}},
		{"required column updated_at missing", func(db *sql.DB) {
			if _, err := db.Exec("ALTER TABLE messages DROP COLUMN updated_at"); err != nil {
				t.Fatalf("drop updated_at: %v", err)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			createFixture(t, dir, func(db *sql.DB) {
				seedSession(t, db, "sess-a", "S", 1)
				seedMessage(t, db, "msg-a", "sess-a", "user", "[]", 1, 1)
				tc.mutate(db)
			})
			store := newTestStore(t, dir)
			_, err := store.Search(context.Background(), "anything", 10)
			if !errors.Is(err, ErrSchemaMismatch) {
				t.Fatalf("err = %v, want ErrSchemaMismatch", err)
			}
		})
	}
}

func TestMissingDatabaseSurfacesAsError(t *testing.T) {
	store := newTestStore(t, t.TempDir())
	_, err := store.Search(context.Background(), "anything", 10)
	if !errors.Is(err, ErrSourceMissing) {
		t.Fatalf("err = %v, want ErrSourceMissing", err)
	}
}

func TestSourceIsStrictlyReadOnly(t *testing.T) {
	dataDir := standardFixture(t, t.TempDir())
	before, err := os.ReadFile(filepath.Join(dataDir, engineDBFile))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	entriesBefore := dirNames(t, dataDir)

	store := newTestStore(t, dataDir)
	if _, err := store.Search(context.Background(), "kubernetes", 10); err != nil {
		t.Fatalf("Search: %v", err)
	}

	source, err := openSource(context.Background(), dataDir)
	if err != nil {
		t.Fatalf("openSource: %v", err)
	}
	defer source.Close()
	if _, err := source.db.Exec(
		"INSERT INTO sessions (id, title, updated_at, created_at) VALUES ('x', 'x', 1, 1)"); err == nil {
		t.Fatal("write through the read-only DSN must fail")
	}

	after, err := os.ReadFile(filepath.Join(dataDir, engineDBFile))
	if err != nil {
		t.Fatalf("re-read fixture: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("crush.db bytes changed after recall reads")
	}
	if got := dirNames(t, dataDir); fmt.Sprint(got) != fmt.Sprint(entriesBefore) {
		t.Fatalf("data dir grew during recall reads: before=%v after=%v", entriesBefore, got)
	}
}

func dirNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir %q: %v", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

func TestSearchLimitClamping(t *testing.T) {
	dir := t.TempDir()
	createFixture(t, dir, func(db *sql.DB) {
		seedSession(t, db, "sess-many", "Many", 1)
		for i := 0; i < 15; i++ {
			seedMessage(t, db, fmt.Sprintf("msg-%02d", i), "sess-many", "user",
				partsArray(partFixture(t, "text", map[string]string{"text": "repeated needle term"})),
				int64(100+i), int64(100+i))
		}
	})
	store := newTestStore(t, dir)

	cases := []struct {
		name  string
		limit int
		want  int
	}{
		{"zero becomes default", 0, defaultSearchLimit},
		{"explicit limit honored", 4, 4},
		{"over-cap clamps", 999, 15},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			results, err := store.Search(context.Background(), "needle", tc.limit)
			if err != nil {
				t.Fatalf("Search: %v", err)
			}
			if len(results) != tc.want {
				t.Fatalf("got %d results, want %d", len(results), tc.want)
			}
		})
	}
}

func TestSearchQuerySafety(t *testing.T) {
	dataDir := standardFixture(t, t.TempDir())
	store := newTestStore(t, dataDir)

	// FTS operators and punctuation must never produce a SQL error.
	for _, query := range []string{
		`AND OR NOT`,
		`"unterminated`,
		`(parenthesis`,
		`kuber*netes "quoted"`,
		`column:title`,
		`NEAR/3`,
		`lunch AND kubernetes`,
	} {
		if _, err := store.Search(context.Background(), query, 10); err != nil {
			t.Fatalf("Search(%q) errored: %v", query, err)
		}
	}

	// Queries with no searchable word are a typed user error.
	for _, query := range []string{"", "   ", `"`, "...", "-"} {
		if _, err := store.Search(context.Background(), query, 10); !errors.Is(err, ErrInvalidQuery) {
			t.Fatalf("Search(%q) err = %v, want ErrInvalidQuery", query, err)
		}
	}
}

func TestRebuildDropsAndResyncs(t *testing.T) {
	dataDir := standardFixture(t, t.TempDir())
	store := newTestStore(t, dataDir)

	if _, err := store.Search(context.Background(), "kubernetes", 10); err != nil {
		t.Fatalf("initial search: %v", err)
	}
	if err := store.Rebuild(context.Background()); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	results, err := store.Search(context.Background(), "kubernetes", 10)
	if err != nil {
		t.Fatalf("post-rebuild search: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("post-rebuild hits = %+v", results)
	}
}
