# Execution Plan: Hermes learning loop parity

Date: 2026-09-02  
Completed: 2026-09-02  
Owner: implementation agent  
Branch: `agent/hermes-learning-loop-parity`  
Pull request: #11

## Status

Completed. The existing bounded learning loop was audited against pinned
`NousResearch/hermes-agent` revision
`ab9866bc64df48281a2d929dfb1dfd1001973d24`, the detached-session parity gaps
were fixed, contracts were reconciled with code, and clean-checkout Linux and
Windows validation passed.

## Outcome

Gotack now has one coherent Hermes-derived self-learning loop across five
bounded components:

- `internal/memory` and `cmd/memory` own capped, locked, atomic `MEMORY.md` and
  `USER.md` mutation without automatic eviction or echoed write payloads;
- `internal/skillmanage` and `cmd/skills` expose the pinned five-action
  `skill_manage` surface, same-process `skill_view` verification, a protected
  `.ownership.json` manifest, legacy-safe ownership migration, and recoverable
  background consolidation deletes;
- `internal/reflection` and `reflection_host.go` track 10 accepted foreground
  turns and 10 model iterations, then launch one cancellable detached review
  with a strict latest-24 context projection;
- `internal/guard` and `cmd/guard` enforce approval tiers, the destructive
  command floor, trusted review metadata, and the background-review allowlist;
- `internal/recall` and `cmd/recall` index the read-only Crush history source
  into a derived FTS5 database and cap hydrated response content at 24 KiB.

The host continues to communicate with Crush only through `internal/crushapi`,
REST/SSE, and stdio MCP. No daemon, prompt nudge, journey UI, three-strike
learning rule, or staged approval queue was added.

## Authority and parity decisions

This plan uses the pinned upstream revision as the behavior authority and
adapts only where the Gotack/Crush process boundary requires it.

Two prompt assumptions conflicted with that source and were resolved in favor
of the pin:

1. the advertised upstream action enum is `create`, `patch`, `delete`,
   `write_file`, and `remove_file`; `revert` and `manage` are not actions in the
   pinned tool schema;
2. a verified background consolidation delete is recoverable through the
   hidden `.archive` location, while a foreground delete remains a hard delete.
   This is recovery state inside the managed root, not a separate archive tool
   or autonomous curator service.

Hermes can retain a warm same-process model cache for its review fork. Gotack
creates a fresh detached REST session, so its adapter explicitly carries only
the newest 24 transcript items and bounds every carried preview.

## Scope completed

### Persistent memory

- Confirmed 2,200-rune `MEMORY.md` and 1,375-rune `USER.md` caps.
- Confirmed final-state overflow refusal with no eviction.
- Confirmed raw UTF-8 entry storage, cross-process Unix/Windows locking,
  temporary-file sync, and atomic replacement.
- Confirmed compact successful responses and bounded corrective content only
  for recoverable failures.

### Procedural skills

- Confirmed Crush remains the canonical skill catalog and viewer.
- Confirmed the same long-lived MCP process owns `skill_view` marks and
  `skill_manage` mutations.
- Confirmed 20-operation atomic batches, managed-root confinement, rollback,
  symlink/special-file refusal, and support-file bounds.
- Renamed the current provenance file to `.ownership.json` while reading the
  legacy `.gotack-agent-skills.json` only when the current file is absent.
  Corrupt or redirected current state fails closed.
- Confirmed background edits target only agent-owned skills and require a fresh
  exact-file read mark.
- Confirmed background delete requires an existing different agent-owned
  `absorbed_into` umbrella and moves the source to `.archive`; foreground
  delete remains permanent.

### Background reflection

- Confirmed memory cadence at 10 accepted foreground user prompts.
- Confirmed skill cadence at 10 unique model iterations, reset by
  `skill_manage`.
- Confirmed scheduled jobs suppress review launch.
- Confirmed one host-wide detached review, 16-iteration ceiling, cancellation
  when a new foreground prompt begins, and session/roster cleanup.
- Replaced the cache-dependent digest with exactly the newest 24 items,
  300-rune user/assistant previews, and 200-rune tool text/results using
  rune-safe explicit truncation.

### Guard and recall

- Confirmed destructive-command checks precede review allowlisting.
- Confirmed review sessions can use only memory, skill verification/mutation,
  and bounded local read/search tools.
- Confirmed `crush.db` opens with SQLite `mode=ro`, the FTS5 index is derived
  and disposable, source deletions reconcile, and hydrated response content is
  bounded to 24 KiB.

### Host and package wiring

- Confirmed the single workspace activation sequence registers `memory`,
  `skills`, `recall`, and `guard` together with office/context wiring.
- Confirmed Crush skill paths merge existing configured roots, bundled skills,
  the learned per-user root, and workspace `.agents/skills` with stable
  deduplication; the mutation server receives only the learned root.
- Fixed portable attachment basename normalization so Windows-origin paths are
  handled correctly by Linux tests and remote clients.
- Updated CI so a clean checkout creates the minimal Wails embed directory,
  generates `frontend/wailsjs`, builds `frontend/dist`, and only then executes
  Go and frontend validation.

## Validation evidence

The source publication run (`33582777220`) and permanent PR CI run
(`33583000790`) completed the requested commands successfully:

```bash
go test ./internal/memory ./internal/skillmanage ./internal/recall ./internal/reflection ./internal/guard
go test ./cmd/memory ./cmd/skills ./cmd/recall ./cmd/guard
go test ./...
go vet ./...
node scripts/check-repository-invariants.mjs
```

Additional passing gates:

```bash
pnpm --dir frontend check
pnpm --dir frontend test
pnpm --dir frontend build
gofmt repository check
generated UI-event drift check
actual-import scan for third_party/crush/internal
```

Permanent PR CI also completed the Windows Wails portable build, compiled the
pinned Crush revision and all packaged helpers, assembled the portable tree,
and uploaded artifact `gotack-windows-amd64`.

The downloaded artifact was inspected independently. Its SHA-256 was
`fb14e41534bde7bddc383424ab059892ad4ba9814262be986660e5674663a10c`, matching
GitHub's recorded digest, and it contained non-empty copies of:

```text
gotack.exe
resources/crush.exe
resources/guard.exe
resources/memory.exe
resources/office.exe
resources/recall.exe
resources/skills.exe
README.txt
```

Deterministic host/runtime tests cover accepted-turn cadence, iteration
counting, cancellation, the final-response gate, the 16-iteration boundary,
review roster cleanup, atomic mutations, ownership safety, and recall bounds.
A live third-party-model turn is intentionally not part of credential-free CI;
the deterministic integration tests and inspected packaged artifact provide
the reproducible completion proof without external credentials or model spend.

## Documentation result

The following contracts were rechecked against implementation:

- `docs/contracts/gotack-memory-mcp.md` — already matched code;
- `docs/contracts/gotack-skills-mcp.md` — updated for `.ownership.json`, legacy
  migration, and recoverable background delete semantics;
- `docs/contracts/gotack-recall-mcp.md` — already matched code;
- `docs/contracts/gotack-reflection.md` — updated for strict latest-24,
  300/200-rune detached-session bounds.

## Recovery

- Memory recovery remains manual editing or deletion of the plain-text memory
  files.
- Skills remain plain files. A background consolidation can be restored
  manually from `.archive`; deleting `.ownership.json` deliberately removes
  agent-owned provenance and therefore disables future background mutation of
  those existing skills.
- Recall recovery remains deletion and rebuild of the derived index; the Crush
  history database is untouched.
- Disabling reflection consists of removing its tracker/host wiring and review
  roster branch; memory, skills, guard, and recall remain independently usable.

## Result

The bounded Hermes learning loop is complete on PR #11, documented, packaged,
and ready for review. The PR is not merged by this plan; repository maintainers
retain normal review and merge control.
