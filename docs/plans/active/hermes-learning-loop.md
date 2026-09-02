# Execution Plan: Hermes learning loop parity

Date: 2026-09-02

## Status

Active — implementation is present; final repository and packaged-app
validation is still being completed before this record moves to `completed/`.

## Outcome

Gotack learns through the same bounded primitives as current Hermes Agent:
persistent `MEMORY.md`/`USER.md`, progressive-disclosure skills, and a
post-turn background reviewer on separate memory/skill cadences. The feature
must stay token-bounded and must not introduce a second agent loop beside
Crush.

## Authority

Behavior was compared against the official NousResearch Hermes Agent source at
the current main commit
[`ab9866bc64df48281a2d929dfb1dfd1001973d24`](https://github.com/NousResearch/hermes-agent/commit/ab9866bc64df48281a2d929dfb1dfd1001973d24)
(2026-09-02). The skill-management reference files are byte-identical to the
reviewed `622883bad7f55f56a6393cd994e36c65fbdff253` snapshot. Current gotack
code and tests are the implementation authority; older Hermes design notes are
not.

Relevant code truth:

- `internal/memory`, `cmd/memory`, `memory_seed.go`;
- `internal/skillmanage`, `cmd/skills`, `skills_seed.go`;
- `internal/reflection`, `reflection_host.go`, `internal/uievents`;
- `internal/guard` and `cmd/guard`;
- `internal/recall`, `cmd/recall`, `recall_seed.go`.

## Scope

In scope:

- bounded, atomic memory mutations with compact results;
- Crush-owned catalog/view disclosure plus the `skill_view` safety handshake and
  `skill_manage` mutations with background ownership protection;
- memory review every 10 accepted user turns and skill review every 10 model
  iterations, with `skill_manage` resetting skill cadence;
- one cancellable detached review, a bounded transcript digest, strict tool
  allowlist, and a 16-iteration ceiling;
- read-only, token-bounded `session_search` over past Crush sessions;
- workspace registration and packaged `memory`, `skills`, `recall`, and
  `guard` binaries.

Explicitly out of scope because these are not the reviewed core mechanism:

- injecting literal memory/skill nudge text into user prompts;
- a three-success pattern counter or automatic `skillpattern` distillation;
- a Journey/timeline UI;
- a separate staging or approval queue for learned writes;
- an autonomous curator, archival scheduler, or consolidation daemon;
- session-end/hourly reflection gates or review-after-every-turn behavior.

No placeholder package or file should remain for an excluded mechanism.

## Implementation record

- [x] Memory: two rune-capped stores (2,200/1,375), atomic batches,
  no eviction, content checks, cross-process locking, atomic replacement, and
  success responses that do not echo stored text.
- [x] Skills: Crush-owned catalog/view, the two-tool `skill_view` safety
  handshake plus five-action `skill_manage`; max-20 rollback batches;
  managed-root path validation; compact results; background-only ownership
  manifest and fresh-read enforcement.
- [x] Reflection: exact 10/10 counters; transcript hydration for memory only;
  post-final-response gate; one detached review; latest-24 plus 300/200-rune
  digest with recent tool results; live-turn cancellation; 16-iteration
  ceiling; scheduled-run suppression; detached-session cleanup.
- [x] Guard: separate review roster, destructive-command floor, review tool
  allowlist, and host-owned skill metadata injection.
- [x] Recall: strictly read-only Crush source, derived/rebuildable FTS5 index,
  discover/read/anchor-window/browse shapes, delete reconciliation, and a
  24-KiB hydrated-content budget.
- [x] Host registration: missing binaries remove their own workspace MCP keys;
  learned skills root is merged into `options.skills_paths`.
- [ ] Final packaging, repository-wide checks, and packaged-app smoke proof.

## Adaptations to the Crush boundary

- Review uses a detached session because the REST API exposes create/send,
  not a Hermes-style in-process fork.
- The review uses the configured model because the session create/send path
  has no per-review model argument.
- Review safety is enforced by a separate host-written roster plus PreToolUse
  policy; hidden skill ownership fields are never model-controlled. Crush's
  canonical view remains the catalog/read source; `skill_view` is retained only
  as a same-process read mark for the separate `skills.exe` writer.
- Recall indexes the engine's SQLite data read-only in a gotack-owned database
  and returns actual rows without an LLM summarization pass.

These are transport/platform adaptations. They do not add a new learning
policy.

## Risks and recovery

- A learned memory entry affects later prompts. Mitigation: bounded content
  checks and a guard that blocks generic context writes. Recovery: edit/remove
  the plain-text entry or disable `gotack-memory`.
- A background skill edit could overwrite user guidance. Mitigation: the
  ownership manifest and same-review `skill_view` check. Recovery: restore the
  plain skill files from source control/backup or remove the MCP registration.
- A review could consume excessive context or keep calling tools. Mitigation:
  bounded digest, one in flight, strict tool allowlist, live-turn cancellation,
  and 16-iteration ceiling.
- `recall.db` is derived state. Recovery: delete/rebuild it from the untouched
  read-only `crush.db` source.

## Validation

Focused proof to retain:

```powershell
go test ./internal/memory ./internal/skillmanage ./internal/recall ./internal/reflection ./internal/guard
go test ./cmd/memory ./cmd/skills ./cmd/recall ./cmd/guard
```

Final proof before completion:

```powershell
rg --files -g '*.go' | ForEach-Object { gofmt -l $_ }
go test ./...
go vet ./...
go build ./...
node scripts/check-repository-invariants.mjs
pnpm --dir frontend check
pnpm --dir frontend test
pnpm --dir frontend build
```

Also verify the packaged resources contain all four helper executables and run
one real-app smoke path: accepted foreground turn, due review creation, guarded
tool use, cleanup, learned skill discovery, and bounded recall output.

## Decisions

- 2026-09-02: Pin parity review to official commit
  `ab9866bc64df48281a2d929dfb1dfd1001973d24`; the skill-management files are
  byte-identical to the `622883bad7f55f56a6393cd994e36c65fbdff253` snapshot;
  do not extrapolate from older local prose.
- 2026-09-02: Keep `skill_view` as the writer-process safety handshake while
  removing the duplicate `skills_list` surface. Crush's injected catalog and
  canonical `view` remain authoritative for discovery and ordinary reads.
- 2026-09-02: Remove the old literal nudge and the proposed pattern/Journey/
  staging/curator layers because current reviewed Hermes core does not require
  them.
- 2026-09-02: Keep gotack-specific work only where the Crush boundary requires
  an adapter: detached session, routed digest, review roster, MCP registration,
  and read-only recall index.

## Result

Pending final validation. Record exact commands, packaged-app evidence, and
any remaining limitation here before moving this plan to
`docs/plans/completed/`.
