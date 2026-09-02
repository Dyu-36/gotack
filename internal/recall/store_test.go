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
)

const (
	fixtureSessionsTable = `CREATE TABLE sessions (
id TEXT PRIMARY KEY, parent_session_id TEXT, title TEXT NOT NULL,
message_count INTEGER NOT NULL DEFAULT 0, prompt_tokens INTEGER NOT NULL DEFAULT 0,
completion_tokens INTEGER NOT NULL DEFAULT 0, cost REAL NOT NULL DEFAULT 0,
updated_at INTEGER NOT NULL, created_at INTEGER NOT NULL)`
	fixtureMessagesTable = `CREATE TABLE messages (
id TEXT PRIMARY KEY, session_id TEXT NOT NULL, role TEXT NOT NULL,
parts TEXT NOT NULL DEFAULT '[]', model TEXT, created_at INTEGER NOT NULL,
updated_at INTEGER NOT NULL, finished_at INTEGER, provider TEXT,
is_summary_message INTEGER DEFAULT 0 NOT NULL)`
)

func createFixture(t *testing.T, dir string, mutate func(*sql.DB)) string {
	t.Helper()
	db := openFixtureRW(t, dir)
	for _, statement := range []string{fixtureSessionsTable, fixtureMessagesTable} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("create fixture: %v", err)
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
	if _, err := db.Exec(
		"INSERT INTO sessions (id, title, updated_at, created_at) VALUES (?, ?, ?, ?)",
		id, title, updatedAt, updatedAt); err != nil {
		t.Fatalf("seed session %q: %v", id, err)
	}
}

func seedMessage(t *testing.T, db *sql.DB, id, sessionID, role, parts string, createdAt, updatedAt int64) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO messages
(id, session_id, role, parts, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		id, sessionID, role, parts, createdAt, updatedAt); err != nil {
		t.Fatalf("seed message %q: %v", id, err)
	}
}

func standardFixture(t *testing.T, dir string) string {
	t.Helper()
	return createFixture(t, dir, func(db *sql.DB) {
		seedSession(t, db, "sess-deploy", "Deploy fixes", 200)
		seedSession(t, db, "sess-stage", "Staging work", 300)
		seedMessage(t, db, "deploy-1", "sess-deploy", "user", textParts(t, "kubernetes deploy question"), 101, 101)
		seedMessage(t, db, "deploy-2", "sess-deploy", "assistant", textParts(t, "kubernetes rollout answer"), 102, 102)
		seedMessage(t, db, "deploy-3", "sess-deploy", "tool", toolParts(t, "pods healthy"), 103, 103)
		seedMessage(t, db, "stage-1", "sess-stage", "user", textParts(t, "kubernetes staging question"), 201, 201)
		seedMessage(t, db, "stage-2", "sess-stage", "assistant", textParts(t, "staging rollout answer"), 202, 202)
	})
}

func textParts(t *testing.T, text string) string {
	t.Helper()
	return partsArray(partFixture(t, "text", map[string]string{"text": text}))
}

func toolParts(t *testing.T, content string) string {
	t.Helper()
	return partsArray(partFixture(t, "tool_result", map[string]string{"name": "exec", "content": content}))
}

func newTestStore(t *testing.T, dataDir string) *Store {
	t.Helper()
	store := OpenStore(dataDir, t.TempDir(), nil)
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestDiscoveryDedupesAndHydratesActualMessages(t *testing.T) {
	store := newTestStore(t, standardFixture(t, t.TempDir()))
	results, err := store.SearchWithOptions(context.Background(), SearchOptions{
		Query: "kubernetes", Limit: 10, Detail: DetailAdaptive,
	})
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d distinct sessions: %+v", len(results), results)
	}
	if results[0].Detail != DetailFull || len(results[0].Messages) < 2 {
		t.Fatalf("top result was not fully hydrated: %+v", results[0])
	}
	if results[1].Detail != DetailAdaptive || len(results[1].Messages) != 1 {
		t.Fatalf("lower result was not compact: %+v", results[1])
	}
	for _, result := range results {
		if result.MatchMessageID == "" || len(result.Messages) == 0 {
			t.Fatalf("result lacks its actual anchor: %+v", result)
		}
		found := false
		for _, message := range result.Messages {
			found = found || (message.ID == result.MatchMessageID && message.Anchor && strings.Contains(message.Content, "kubernetes"))
		}
		if !found {
			t.Fatalf("anchor was not hydrated: %+v", result)
		}
	}
}

func TestSyncIngestsNewRowsAndReconcilesDeletes(t *testing.T) {
	dir := standardFixture(t, t.TempDir())
	store := newTestStore(t, dir)
	if _, err := store.Search(context.Background(), "kubernetes", 10); err != nil {
		t.Fatalf("initial sync: %v", err)
	}

	db := openFixtureRW(t, dir)
	seedMessage(t, db, "late", "sess-deploy", "assistant", textParts(t, "helm upgrade complete"), 400, 400)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	results, err := store.Search(context.Background(), "helm", 10)
	if err != nil || len(results) != 1 || results[0].MatchMessageID != "late" {
		t.Fatalf("incremental row not found: results=%+v err=%v", results, err)
	}

	db = openFixtureRW(t, dir)
	if _, err := db.Exec("DELETE FROM messages WHERE session_id = 'sess-deploy'"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("DELETE FROM sessions WHERE id = 'sess-deploy'"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	results, err = store.Search(context.Background(), "helm", 10)
	if err != nil || len(results) != 0 {
		t.Fatalf("deleted message remained searchable: results=%+v err=%v", results, err)
	}
	if _, err := store.ReadSession(context.Background(), "sess-deploy"); !errors.Is(err, ErrUnknownSession) {
		t.Fatalf("deleted session read err=%v", err)
	}
}

func TestReadAndAroundBounds(t *testing.T) {
	dir := createFixture(t, t.TempDir(), func(db *sql.DB) {
		seedSession(t, db, "long", "Long", 100)
		for i := 0; i < 40; i++ {
			id := fmtID(i)
			seedMessage(t, db, id, "long", "user", textParts(t, id), int64(i), int64(i))
		}
	})
	store := newTestStore(t, dir)
	read, err := store.ReadSession(context.Background(), "long")
	if err != nil {
		t.Fatal(err)
	}
	if !read.Truncated || read.MessageCount != 40 || len(read.Messages) != 30 ||
		read.Messages[0].ID != "m-00" || read.Messages[19].ID != "m-19" || read.Messages[20].ID != "m-30" {
		t.Fatalf("bounded read = %+v", read)
	}
	around, err := store.Around(context.Background(), "long", "m-20", 999)
	if err != nil {
		t.Fatal(err)
	}
	if around.Window != 20 || len(around.Messages) != 40 || around.MessagesBefore != 20 || around.MessagesAfter != 19 {
		t.Fatalf("around bounds = %+v", around)
	}
}

func fmtID(value int) string {
	return fmt.Sprintf("m-%02d", value)
}

func TestSourceIsReadOnlyAndSchemaDriftSurfaces(t *testing.T) {
	missing := OpenStore(t.TempDir(), t.TempDir(), nil)
	defer missing.Close()
	if err := missing.Sync(context.Background()); !errors.Is(err, ErrSourceMissing) {
		t.Fatalf("missing source err=%v", err)
	}

	dir := standardFixture(t, t.TempDir())
	before, err := os.ReadFile(filepath.Join(dir, engineDBFile))
	if err != nil {
		t.Fatal(err)
	}
	store := newTestStore(t, dir)
	if _, err := store.Search(context.Background(), "kubernetes", 3); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(filepath.Join(dir, engineDBFile))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("source changed: err=%v", err)
	}
	source, err := openSource(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	if _, err := source.db.Exec("DELETE FROM sessions"); err == nil {
		t.Fatal("read-only source accepted a write")
	}

	drift := createFixture(t, t.TempDir(), func(db *sql.DB) {
		if _, err := db.Exec("ALTER TABLE messages DROP COLUMN parts"); err != nil {
			t.Fatal(err)
		}
	})
	if err := OpenStore(drift, t.TempDir(), nil).Sync(context.Background()); !errors.Is(err, ErrSchemaMismatch) {
		t.Fatalf("schema drift err=%v", err)
	}
}

func TestMissingOptionalColumnsStillSearches(t *testing.T) {
	dir := createFixture(t, t.TempDir(), func(db *sql.DB) {
		seedSession(t, db, "s", "Drift", 10)
		seedMessage(t, db, "m", "s", "user", textParts(t, "remember credentials"), 10, 10)
		for _, statement := range []string{
			"ALTER TABLE sessions DROP COLUMN title",
			"ALTER TABLE messages DROP COLUMN role",
			"ALTER TABLE messages DROP COLUMN created_at",
			"ALTER TABLE messages DROP COLUMN is_summary_message",
		} {
			if _, err := db.Exec(statement); err != nil {
				t.Fatal(err)
			}
		}
	})
	var logs bytes.Buffer
	store := OpenStore(dir, t.TempDir(), slog.New(slog.NewTextHandler(&logs, nil)))
	defer store.Close()
	results, err := store.Search(context.Background(), "credentials", 3)
	if err != nil || len(results) != 1 || results[0].Title != "" {
		t.Fatalf("degraded search: results=%+v err=%v", results, err)
	}
	if !strings.Contains(logs.String(), "schema drift") {
		t.Fatalf("degradation not logged: %s", logs.String())
	}
}

func TestRebuildDropsDerivedState(t *testing.T) {
	store := newTestStore(t, standardFixture(t, t.TempDir()))
	if _, err := store.Search(context.Background(), "kubernetes", 3); err != nil {
		t.Fatal(err)
	}
	if err := store.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	results, err := store.Search(context.Background(), "kubernetes", 3)
	if err != nil || len(results) != 2 {
		t.Fatalf("post-rebuild results=%+v err=%v", results, err)
	}
}
