# Contract: gotack-recall MCP server (`cmd/recall`)

Status: implemented by `internal/recall`, `cmd/recall`, and `recall_seed.go`.

`gotack-recall` exposes read-only cross-session recall through one stdio MCP
tool, `session_search`. It opens the active workspace's Crush `crush.db` in
SQLite read-only mode and maintains a separate, disposable FTS5 index. Tool
results contain actual stored messages; no LLM summarization step is used.

## Four request shapes

The handler selects a mode in this precedence order:

| Arguments | Mode | Result |
| --- | --- | --- |
| `session_id` + `around_message_id` | `scroll` | A window centered on one exact message id. |
| `session_id` | `read` | A bounded read of one session. |
| non-blank `query` | `discover` | Distinct matching sessions hydrated around their hits. |
| `{}` or blank `query` | `browse` | Recently active sessions with a short preview. |

An absent MCP `arguments` value is malformed; callers use `{}` to browse.
When fields for more than one shape are supplied, the precedence above is
authoritative rather than an inferred merge.

Common fields:

| Field | Contract |
| --- | --- |
| `limit` | Session count for discover/browse; default 3, clamped to 1–10. |
| `sort` | `newest` or `oldest` for discovery; absent or unrecognized values use relevance. |
| `detail` | `adaptive` (default) or `full`. |
| `role_filter` | Comma-separated discovery roles: `user`, `assistant`, or `tool`; absent/invalid input falls back to user, assistant, and the neutral empty role used for older schemas. |
| `window` | Messages on each side of an anchor; default 5, clamped to 1–20. |

### Discover

Discovery searches FTS5 hits, keeps at most one result per session, and
returns `session_id`, title, matched role/message id, snippet, messages, and
counts before/after the match. In adaptive mode the first result is fully
hydrated with a ±5-message window and three-message start/end bookends; later
results carry only the matched message. `detail: "full"` hydrates every
result.

The safe query parser supports adjacent terms as implicit AND, quoted
phrases, uppercase `AND`/`OR`/`NOT`, and a trailing `*` prefix. Terms are
re-emitted as quoted FTS expressions, so unmatched punctuation, quotes, or
SQLite column syntax cannot escape into raw `MATCH` SQL. Malformed operator
streams degrade to literal phrases; punctuation-only queries with no term or
operator word are refused.

### Read, scroll, and browse

- A small session read returns every message. When a session has more than 30
  messages it returns the first 20 and last 10 and sets `truncated`.
- Anchored scroll returns up to `window` older messages, the exact anchor, and
  up to `window` newer messages, with `messages_before` and `messages_after`.
  Message ids remain strings.
- Browse orders sessions by last activity and returns id, title, timestamp,
  message count, and the first non-summary message as a clipped preview.

## Token and content bounds

Model-facing content is stripped of ANSI escapes and clipped before JSON
encoding:

| Surface | Limit |
| --- | ---: |
| Ordinary message content | 4,000 bytes per message |
| Discovery bookend / browse preview | 1,200 bytes each |
| Combined content in one response | 24 KiB |

Truncated messages carry `content_truncated` and
`original_content_bytes`. Compaction-summary rows are excluded from discovery
bookends. Fields that Crush does not persist—Hermes profile/source/model,
lineage, and links—are not fabricated.

## Source and derived index

Before every browse, discovery, read, or scroll response, the store performs
an incremental sync under one process mutex:

1. open `<data-dir>/crush.db` with `mode=ro`;
2. ingest sessions/messages newer than stored watermarks;
3. reconcile source ids so deleted messages and sessions disappear from the
   index; and
4. advance the watermarks.

The source must contain `sessions(id, updated_at)` and
`messages(id, session_id, parts, updated_at)`. Missing required tables/columns
is a schema error. Missing optional title, role, creation timestamp, or
summary marker is logged and replaced with a neutral value.

The derived database is `<index-dir>/recall.db`, uses WAL mode, and can be
removed or rebuilt without touching engine history. `recall.exe --rebuild`
deletes only the index and its journal sidecars, then resyncs from `crush.db`.

## Registration and removal

On workspace rebind, `recall_seed.go` resolves the active workspace data
directory and registers:

```json
{
  "mcp_servers": {
    "gotack-recall": {
      "command": "<absolute path to recall.exe>",
      "args": [
        "--data-dir", "<workspace Crush data dir>",
        "--index-dir", "<appconfig.Dir()>/recall/<workspace-id>"
      ],
      "type": "stdio",
      "timeout": 30
    }
  }
}
```

If the binary, active workspace, or its data directory is unavailable, the
host removes `mcp_servers.gotack-recall`. The server also accepts
`GOTACK_CRUSH_DATA_DIR`; its standalone index default is
`<appconfig.Dir()>/recall`.

Removing the registration disables recall immediately. Deleting the matching
index directory is safe because `crush.db` remains the source of truth.
