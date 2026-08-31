# Contract: gotack-memory MCP server (`cmd/memory`)

Status: implemented by WP4 (Phase 2 of `docs/plans/completed/hermes-parity-harness.md`).

`cmd/memory` is a stdio MCP server (protocol `2024-11-05`, built on
`internal/mcp`, same pattern as `cmd/office` and `cmd/recall`) that gives the
assistant persistent self-editing memory. It curates two files inside the
seeded context directory; because Crush re-reads every context file on each
prompt construction, a write here lands in the system prompt on the next
turn with no engine restart.

This tool is the **only sanctioned writer** of the memory files, per
`docs/decisions/0003-memory-writes-constrained-by-construction.md`. The
safety is structural: hard caps with documented eviction, provenance on
every entry, atomic writes, and a guard that denies every other write path
into the context directory. No interactive approval is involved.

## Tools

Exactly one tool is exposed.

### `memory`

Input schema:

```json
{
  "type": "object",
  "properties": {
    "action": {"type": "string", "enum": ["view", "add", "replace", "remove"]},
    "target": {"type": "string", "enum": ["memory", "user"]},
    "section": {"type": "string"},
    "content": {"type": "string"}
  },
  "required": ["action"]
}
```

| Field | Meaning |
| --- | --- |
| `action` | `view` reads one file; `add` appends to a section (creating it when absent); `replace` swaps one existing section; `remove` drops one existing section. Required. |
| `target` | `memory` (MEMORY.md, durable facts) or `user` (USER.md, user preferences). Optional, defaults to `memory`. |
| `section` | `§`-delimited section heading. Required for `add`, `replace` and `remove`. |
| `content` | Text to store (may be multiline). Required and non-empty for `add` and `replace`. |

Result: a JSON payload so the model can self-manage its budget (plan 2.2):

```json
{
  "target": "memory",
  "file": "MEMORY.md",
  "size": 312,
  "cap": 2200,
  "remaining": 1888,
  "evicted": 0,
  "content": "…present for action=view only…"
}
```

Validation is strict by construction (decision 0003): unknown `action` or
`target` values, empty `content` on `add`/`replace`, and a missing `section`
heading are typed errors; `replace` or `remove` against a section that does
not exist is an explicit `memory: section not found` error, never a silent
no-op. All errors surface as MCP `isError` tool results.

## File layout

```text
<appconfig.Dir()>/context/          registered under options.global_context_paths
  TACK.md                            seeded persona, not touched by this tool
  memory/
    MEMORY.md                        agent-curated durable facts, cap 2200 bytes
    USER.md                          user preferences and profile, cap 1375 bytes
```

Both files sit under the registered context directory so injection is
automatic, and outside any workspace, so the agent's generic `write`/`edit`
tools cannot reach them (the guard also denies it, see below). The
`memory/` subdirectory is created by the first write; `internal/contextseed`
never deletes files it did not seed, so memory content survives re-seeding.
CLI flags: `-dir` (memory directory), `-session` (provenance writer).
Logs go to stderr; stdout carries only JSON-RPC.

## File format

Sections are delimited by the Hermes-compatible `§` marker; each section
holds one or more entries, each entry preceded by a provenance stamp:

```markdown
§ Project facts
<!-- gotack-memory: session=sess-1 at=2026-09-01T10:00:00Z -->
The build uses pnpm.
§ Preferences
<!-- gotack-memory: session=sess-2 at=2026-09-02T09:30:00Z -->
Prefers concise answers.
```

- A line starting with `§` opens a section; the heading is the trimmed
  remainder (a bare `§` is an unnamed section).
- A line matching exactly `<!-- gotack-memory: session=<id> at=<RFC3339> -->`
  opens a new entry; every other line is content of the open entry. Content
  before the first marker is preserved in an unnamed section, so hand-edited
  files lose nothing on the next write. CRLF input is normalized to LF.
- Parsing is total: every input round-trips byte for byte (proven by
  `TestSectionRoundTrip` in `internal/memory/sections_test.go`).

## Caps and eviction policy

Caps are measured in UTF-8 bytes: `MEMORY.md` 2200, `USER.md` 1375 (the
Hermes values; bytes are the quantity that actually costs prompt space).
Exact policy, pinned by `TestCapEnforcementEvictsOldest`,
`TestEvictionOrdersByTimestamp`, `TestEvictionKeepsUnstampedContentLast`
and `TestCapExceededRejectsSingleOversizedEntry`:

1. After an `add` or `replace`, if the serialized file exceeds the cap,
   entries are evicted oldest-first until it fits. Order: unstamped entries
   (they predate recorded history) first, then stamped entries in ascending
   provenance timestamp, ties broken by file position (top of file first).
2. The just-written entry always survives eviction; a write is never
   silently discarded.
3. If the file still exceeds the cap with only the new entry left (the
   entry alone is too large), the write is **rejected** with
   `memory: … exceeds the file size cap …` naming the cap, forcing the
   model to consolidate; the file on disk is untouched.
4. Sections left empty by eviction are dropped. Surviving entries keep
   their provenance stamps intact.

`add` appends, so absent eviction the oldest entries settle at the top of
the file; the timestamp order above keeps eviction correct even when stamps
and positions disagree.

## Provenance

Every entry written by the tool carries `session=<writer> at=<RFC3339 UTC>`
so a poisoned entry is traceable and removable (decision 0003). The writer
id resolves `-session` flag > `GOTACK_MEMORY_SESSION` > `CRUSH_SESSION_ID`
> `unknown`. Honest limitation: the engine exports the session id only to
hooks today, not to MCP server processes, so in a stock install the session
field reads `unknown` while the timestamp is always accurate; the env seams
are in place for when the engine exports it.

## Atomicity guarantee

All persistence goes through temp-file-plus-rename in the target directory
(`writeFileAtomic` in `internal/memory/store.go`): write, sync, close, then
rename over the target; on any failure the temp file is removed. A crash at
any point therefore leaves either the old or the new content on disk, never
a half-written file inside the system prompt. Proven by
`TestAtomicWriteSemantics`: success replaces content with no temp residue;
a failed rename leaves the target intact and removes the temp file; a
failed persist never touches the existing file.

## Registration

Written by `memory_seed.go` `registerMemoryTools` from the shared workspace
rebind path (`rebindWorkspaceRuntime` in `bind_workspace.go`), at workspace
scope, exactly like `gotack-office`:

```json
"mcp_servers": {
  "gotack-memory": {
    "command": "<absolute path to memory.exe>",
    "type": "stdio",
    "timeout": 30
  }
}
```

Config key written: `mcp_servers.gotack-memory`. Removal path: when the
binary cannot be resolved (bundled-then-PATH, same strategy as
`resolveOfficeCommand`), the host calls `RemoveConfigField` on the same key
so the workspace is never left pointing at a missing binary (hard rule 8).
Both directions are pinned by `memory_seed_test.go`.

## Interaction with the guard

`internal/guard` rule `memory-context-write` denies every generic
`write`/`edit` into the context directory, independent of posture or
write-safe root (`TestEvaluateTierMatrix`). That closes the side door: the
caps above cannot be bypassed by writing the memory files directly, so this
tool stays the only writer by construction. Deleting or editing the files
by hand remains the documented recovery path — the tool re-parses whatever
it finds.

## Removing the feature

1. `RemoveConfigField mcp_servers.gotack-memory` (workspace scope), or let
   the host do it by removing the binary.
2. Delete `cmd/memory/`, `internal/memory/`, `memory_seed.go` and this
   contract; drop the row from `docs/contracts/crush-rest-sse.md`.
3. Optionally delete `<appconfig.Dir()>/context/memory/` — plain text, no
   other state exists.

## Windows-only caveat

Developed and tested on Windows. `os.Rename` over an existing file uses
`MoveFileEx` with replace semantics, which this contract relies on; non-
Windows behavior compiles but is untested, as elsewhere in gotack.
