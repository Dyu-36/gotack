# Contract: gotack-recall MCP server (`cmd/recall`)

Status: implemented by WP5 (Phase 3 of `docs/plans/active/hermes-parity-harness.md`).

`cmd/recall` is a stdio MCP server (protocol `2024-11-05`, built on
`internal/mcp`, same pattern as `cmd/office`) that gives the assistant
cross-session recall over past Crush conversations. It reads the engine's
`crush.db` **strictly read-only** and answers searches from its own FTS5
index, `recall.db`, which gotack owns.

## Tools

Exactly one tool is exposed.

### `session_search`

Input schema:

```json
{
  "type": "object",
  "properties": {
    "query": {"type": "string", "description": "Words or phrases to search for in past conversations"},
    "limit": {"type": "integer", "description": "Maximum number of results (default 10, max 50)"}
  },
  "required": ["query"]
}
```

Result: a JSON text payload. Every hit cites its source so the assistant can
say where a memory came from:

```json
{
  "count": 1,
  "results": [
    {
      "session_id": "sess-deploy",
      "session_title": "Deploy fixes",
      "message_id": "msg-user-1",
      "role": "user",
      "created_at_ms": 1700000000100,
      "created_at": "2023-11-14T22:13:20Z",
      "is_summary": false,
      "snippet": "please fix the [kubernetes] deployment pipeline …"
    }
  ]
}
```

Guarantees:

- Results are ranked by FTS5 `bm25` rank; snippets are produced by
  SQLite's `snippet()` around the matched terms.
- Messages flagged `is_summary_message` in Crush are filtered out, per the
  Phase 3 plan, so summaries do not crowd out real content.
- An incremental sync runs before every search (see below), so new engine
  messages become recallable without any push wiring.
- Failures are returned as MCP `isError` tool results with a typed `recall:`
  message. A missing or drifted `crush.db` is **never** hidden behind an
  empty result set.
- Queries are sanitized: each whitespace token becomes a quoted FTS5 phrase,
  so FTS operators in user input cannot inject query syntax. A query with no
  searchable word returns `recall: search query contains no searchable
  words`.

## Database access guarantees

### crush.db is opened strictly read-only

- DSN: `file:<dataDir>/crush.db?_txlock=immediate&mode=ro`, mirroring
  Crush's own `openDBReadOnly` (`third_party/crush/internal/db/connect_modernc.go`).
- Columns are selected explicitly from a probed schema; the server never
  executes a write statement against the engine database.
- Proven by tests: a write attempt through the read-only handle fails, the
  `crush.db` bytes are byte-identical after a sync, and the data directory
  grows no new files (`internal/recall/store_test.go`,
  `TestSourceIsStrictlyReadOnly`).

### The engine's flock is respected, never held

Crush holds an exclusive advisory lock on `{dataDir}/crush.lock` for the
lifetime of a running engine (`third_party/crush/internal/db/datadirlock.go`
plus `internal/lock`). The recall server mirrors those lock primitives to
**probe** the lock but follows the Phase 3 plan's hard constraint: it never
acquires/holds the data-dir lock and never sets `CRUSH_SKIP_DATADIR_LOCK`.

Probe policy:

1. A missing `crush.lock` means no engine owns the directory (Crush creates
   the file on startup and never unlinks it): proceed immediately.
2. Otherwise the server attempts a non-blocking lock observation up to 5
   times with 200 ms exponential-free backoff, to ride out engine restart
   and migration windows. A successful observation releases the lock
   immediately.
3. Contention after the budget is **tolerated, not fatal**: the server logs
   it and proceeds with the strictly read-only open, which is safe against a
   WAL-mode database. The wait is therefore always bounded and never blocks
   forever.

Proven by tests in `internal/recall/flock_test.go`: retry-then-acquire,
bounded wait under a permanently held lock, and successful search while
another process holds `crush.lock`.

### No FTS5 objects inside crush.db

The index lives in a separate gotack-owned database. An unmanaged FTS table
inside the goose-migrated engine database would collide with a future
upstream migration (Phase 3 decision, recorded in the plan's Decisions).

## recall.db: location, schema, rebuild

- Location: `<appconfig.Dir()>/recall/recall.db` (`%AppData%\gotack\recall\recall.db`
  on Windows), overridable with `-index-dir`. The plan's Phase 3 text says
  `<appconfig.Dir()>/recall.db`; the WP5 work order directed a `recall/`
  subdirectory, which this contract fixes as the canonical path.
- Contents: `sessions` (id, title, updated_at), `messages` (id, session_id,
  role, is_summary, created_at, updated_at, extracted text), `messages_fts`
  (FTS5, unicode61 tokenizer), and `recall_meta` (watermarks). Journal mode
  is WAL.
- Incremental sync: watermarks on `sessions.updated_at` / `messages.updated_at`
  (Unix milliseconds) stored in `recall_meta`; each sync selects rows with
  `updated_at >= watermark` and upserts them, so re-reads are idempotent.
  Deleted engine messages are not purged; rebuild to reconcile.
- Text extraction from the `messages.parts` JSON is defensive: known part
  types (`text`, `reasoning`, `tool_call`, `tool_result`, `shell_command`,
  `finish`) contribute their human-readable fields; unknown or malformed
  parts are skipped, never fatal. Per-message text is capped at 64 KB.
- **Rebuild procedure**: run `recall.exe -rebuild` (drops `recall.db` and its
  WAL/SHM sidecars, then resyncs), or delete `%AppData%\gotack\recall\` and
  restart. The index is a pure cache; rebuilding loses nothing.

## Schema-drift tolerance

At open time the server probes `PRAGMA table_info` for `sessions` and
`messages`:

- Missing **table** or missing **required column** (`sessions.id`,
  `sessions.updated_at`, `messages.id`, `messages.session_id`,
  `messages.parts`, `messages.updated_at`) returns
  `recall: crush.db schema does not match the recall contract` — surfaced as
  a tool error, never an empty result, and never a panic.
- Missing **optional column** (`sessions.title`, `messages.role`,
  `messages.created_at`, `messages.is_summary_message`) degrades
  gracefully: the query substitutes a neutral expression, logs a
  `schema drift` warning, and keeps working.

Guarded upstream: `scripts/update-crush.ps1` now fails any pin bump whose
`internal/db/migrations/20250424200609_initial.sql` or
`internal/db/models.go` lacks the depended-on table/column markers.

## Registration (host wiring is out of scope for WP5)

The binary is designed so host registration is a one-line config entry, in
the same shape `office_seed.go` writes for `gotack-office`:

```json
"mcp_servers": {
  "gotack-recall": {
    "command": "<absolute path to recall.exe>",
    "type": "stdio",
    "timeout": 30,
    "env": {"GOTACK_CRUSH_DATA_DIR": "<workspace Crush data dir>"}
  }
}
```

Per Phase 3.3, pass the workspace's known data directory instead of
re-deriving it: the host opens workspaces with `OpenWithDataDir`; the
default workspace uses `<appconfig.Dir()>/default-workspace-data`, which is
also the binary's built-in default when neither `-data-dir` nor
`GOTACK_CRUSH_DATA_DIR` is set. CLI flags: `-data-dir`, `-index-dir`,
`-rebuild`. Logs go to stderr; stdout carries only JSON-RPC.

WP5 deliberately does not write this config key from the desktop host: the
work order assigns that wiring elsewhere, and Phase 3 of the plan does not
list a registration step (unlike Phase 2's `mcp_servers.gotack-memory`).

## Coupling declaration

This feature reads Crush's private SQLite schema, a coupling the REST
boundary otherwise avoids. It is justified because no REST endpoint exposes
historical message search (plan Phase 3.4). The coupling is named here,
checked by `scripts/update-crush.ps1` markers, and exercised by tests that
build fixture databases with the real SQLite driver.

## Dependency exception (WP5)

Reading SQLite requires a driver. Per the work order's authorized exception,
the module adds exactly the driver Crush itself uses on this platform:
`modernc.org/sqlite v1.56.0` — the exact requirement line from
`third_party/crush/go.mod` (pure Go, registered as driver `sqlite`, used by
Crush's `connect_modernc.go` on windows/amd64). No other module was added;
`golang.org/x/sys` only moved from indirect to direct because the lock probe
mirrors Crush's Windows `LockFileEx` code.

## Removing the feature

1. Remove the `mcp_servers.gotack-recall` config entry from any workspace
   (or never add it).
2. Delete `cmd/recall/` and `internal/recall/`, and the
   `docs/contracts/gotack-recall-mcp.md` contract.
3. Delete the recall schema marker block in `scripts/update-crush.ps1`.
4. If no other feature needs SQLite, `go mod tidy` drops
   `modernc.org/sqlite` and its transitive requirements.
5. Optionally delete `%AppData%\gotack\recall\` — it is a pure cache.

## Windows-only caveat

Developed and tested on Windows (LockFileEx-based lock probe, NTFS paths in
SQLite URI DSNs). The unix path (`flock_unix.go`) mirrors Crush's unix lock
code and compiles, but gotack ships for Windows; treat non-Windows behavior
as untested.
