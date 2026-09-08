# Bounded personal assistant context

Gotack is a Windows personal assistant. Coding is optional, not the default persona.

## Permanent prompt

Only three sources may enter a persistent snapshot:

- Embedded `internal/assistant/core.md`: at most 4,096 bytes.
- `<app-data>/assistant/PROFILE.md`: at most 1,375 body characters.
- `<app-data>/assistant/MEMORY.md`: at most 2,200 body characters.

The reader as well as the writer enforces limits. Rendering includes whole sanitized
entries, not partial facts. File reads are bounded to 64 KiB. Invalid or oversized
sources never grow the prompt; originals are left unchanged. The combined snapshot
has a 24 KiB ceiling including headers. Byte/character counts are not model-specific
token counts. Files added beside these sources are not automatically included.

HMAC identities, staged validation, transactional registration and cross-process
leases remain. Unchanged content reuses its generation. Foreground turn preparation
checks the small source set and avoids a config refresh when the acknowledged
snapshot matches. Memory changes become visible at the next turn boundary, not
in the middle of a tool loop. This is not a guarantee of provider cache hits.

## Existing data

A one-time additive import copies legacy `context/memory/USER.md` to
`assistant/PROFILE.md` and `context/memory/MEMORY.md` to `assistant/MEMORY.md`.
Existing new-format files are never overwritten. Original files are never deleted.
Backups are in `<app-data>/assistant-import-backups`, with an import report at
`assistant/.legacy-import-v1.json`.

Legacy root USER/TACK custom instructions are preserved for explicit review, not
blindly promoted into product policy. Conflicting, invalid and omitted content
remains in originals/backups. The Context settings panel shows review paths.
The shipped `resources/context` bundle and its legacy accept/rollback state machine
are retired; there is no automatic loss of user data to simplify the repository.

## Personal memory learning

Review is memory-only, every 15 accepted user turns, after a short idle delay.
A foreground turn cancels pending review. The digest contains at most 16 recent
user/assistant items, with an aggregate 4,000-character budget. Tool dumps are not
included. Review has a 3-iteration limit and a cumulative 32,000-input-token run
budget, not a 32K context allocation per call. No budget-unaware send fallback is
used. The host guard permits only the memory tool in these review sessions.
Automatic skill creation/modification is disabled; foreground skill use remains.
Learning cadence restarts after reconnect rather than fetching all prior messages
before the next user turn. The current engine transcript API still reads a complete
transcript for a background review; only the tail is extracted and sent for review.

## Recall and observation

The existing SQLite/FTS5 recall archive remains. No vector service, embeddings,
reranker or new memory database is introduced without evidence that it is needed.
`session_search` defaults to brief snippets and IDs. Explicit adaptive/full/around
reads remain available, with a 12 KiB serialized-result ceiling. Narrow subsequent
reads use session/message IDs. Content clipping affects results, not stored history.
Session/time and lineage indexes are additive and do not require a database rebuild.
Deletion reconciliation remains intact, even though its cost can grow with history.

Context settings expose payload size, caps, omitted entries and snapshot timing.
Recall logs record response sizes and elapsed time, not query or conversation text.
The existing engine run telemetry remains the source for provider/cache timing.

These changes bound permanent context and individual recall results. They do not
bound an entire active conversation, attachment data, tool schemas, or a sequence
of tool calls. Those budgets and conversation compaction belong to the engine and
must be measured separately. No fixed total prompt size or latency gain is claimed.

## Validation

No unit/integration tests, Windows build, or runtime benchmark were executed for
this refactor. Review is static. Existing test fixtures need to follow the new
explicit-source layout; obsolete migration coverage is replaced by additive-import
coverage rather than removing safety invariants.
