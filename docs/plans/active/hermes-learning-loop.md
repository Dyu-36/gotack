# Execution Plan: Hermes learning loop parity

Date: 2026-09-02

## Status

Ready for credentialed smoke — implementation, repository validation, and the
Windows portable package are complete. One credentialed, interactive
packaged-app smoke path remains before this record can move to `completed/`.

## Outcome

Gotack learns through the same bounded primitives as current Hermes Agent:
persistent `MEMORY.md`/`USER.md`, progressive-disclosure skills, and a
post-turn background reviewer on separate memory/skill cadences. The feature
stays token-bounded and does not introduce a second agent loop beside Crush.

## Authority

Behavior was compared against the official NousResearch Hermes Agent source at
commit
[`ab9866bc64df48281a2d929dfb1dfd1001973d24`](https://github.com/NousResearch/hermes-agent/commit/ab9866bc64df48281a2d929dfb1dfd1001973d24)
(2026-09-02). Current Gotack code and tests are the implementation authority;
older Hermes design notes are not.

The pinned upstream source resolves two naming/behavior ambiguities:

- the five advertised `skill_manage` actions are `create`, `patch`, `delete`,
  `write_file`, and `remove_file`; `revert` and `manage` are not actions in the
  pinned schema;
- a verified background consolidation delete is recoverable through the hidden
  `.archive` location, while a foreground delete remains a hard delete. This is
  recovery state, not a separate archive/restore tool surface or daemon.

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

No placeholder package or file remains for an excluded mechanism.

## Implementation record

- [x] Memory: two rune-capped stores (2,200/1,375), atomic batches, no
  eviction, content checks, cross-process locking, atomic replacement, and
  success responses that do not echo stored text.
- [x] Skills: Crush-owned catalog/view, the two-tool `skill_view` safety
  handshake plus the pinned five-action `skill_manage`; max-20 rollback
  batches; managed-root path validation; compact results; `.ownership.json`
  background provenance; legacy `.gotack-agent-skills.json` migration; and
  fresh-read enforcement that fails closed.
- [x] Background consolidation delete: requires a different existing
  agent-owned `absorbed_into` umbrella, consumes a fresh source read, and moves
  the source tree into hidden recovery `.archive`; foreground delete remains
  permanent.
- [x] Reflection: exact 10/10 counters; transcript hydration for memory only;
  post-final-response gate; one detached review; exactly the latest 24 items
  with 300-rune user/assistant and 200-rune tool previews; live-turn
  cancellation; 16-iteration ceiling; scheduled-run suppression; and detached
  session cleanup.
- [x] Guard: separate review roster, destructive-command floor, review tool
  allowlist, and host-owned skill metadata injection.
- [x] Recall: strictly read-only Crush source, derived/rebuildable FTS5 index,
  discover/read/anchor-window/browse shapes, delete reconciliation, and a
  24-KiB hydrated-content budget.
- [x] Host registration: missing binaries remove their own workspace MCP keys;
  learned skills merge with existing configured, bundled, and workspace roots
  in `options.skills_paths`, while `skills.exe` receives only the learned root.
- [x] Automated packaging and repository-wide checks, including the Windows
  portable artifact.
- [ ] Credentialed packaged-app smoke proof: accepted foreground turn, due
  review, guarded tool use, cleanup, learned-skill discovery, and bounded recall
  output.

## Adaptations to the Crush boundary

- Review uses a detached session because the REST API exposes create/send, not
  a Hermes-style in-process fork.
- A fresh detached REST session cannot reuse Hermes' same-process warm prompt
  cache, so the adapter explicitly carries only the latest 24 transcript items
  and bounds every carried preview.
- The review uses the configured model because the session create/send path has
  no per-review model argument.
- Review safety is enforced by a separate host-written roster plus PreToolUse
  policy; hidden skill ownership fields are never model-controlled. Crush's
  canonical view remains the catalog/read source; `skill_view` is retained only
  as a same-process read mark for the separate `skills.exe` writer.
- Recall indexes the engine's SQLite data read-only in a Gotack-owned database
  and returns actual rows without an LLM summarization pass.

These are transport/platform adaptations. They do not add a new learning
policy.

## Risks and recovery

- A learned memory entry affects later prompts. Mitigation: bounded content
  checks and a guard that blocks generic context writes. Recovery: edit/remove
  the plain-text entry or disable `gotack-memory`.
- A background skill edit could overwrite user guidance. Mitigation: the
  ownership manifest and same-review `skill_view` check. Recovery: restore the
  plain skill files from source control/backup or restore a verified
  consolidation from `.archive`.
- A review could consume excessive context or keep calling tools. Mitigation:
  bounded digest, one in flight, strict tool allowlist, live-turn cancellation,
  and 16-iteration ceiling.
- `recall.db` is derived state. Recovery: delete/rebuild it from the untouched
  read-only `crush.db` source.

## Validation

The final merged-head CI run was `33584052326` on code head
`b4e8bd1c3f77d943c96b631e456f555345270c00`, based on current `main` commit
`92e1c5e815571ea0609c5d4ef3acf32714305336`. It passed all three jobs.

Requested focused and repository-wide proof:

```powershell
go test ./internal/memory ./internal/skillmanage ./internal/recall ./internal/reflection ./internal/guard
go test ./cmd/memory ./cmd/skills ./cmd/recall ./cmd/guard
go test ./...
go vet ./...
node scripts/check-repository-invariants.mjs
```

Additional passing gates:

```powershell
staticcheck ./...
deadcode -test ./...
pnpm --dir frontend check
pnpm --dir frontend test
pnpm --dir frontend build
```

CI also passed gofmt, generated UI-event drift, Wails binding generation, the
Go-parser invariant blocking imports from `third_party/crush/internal/...`, and
the Windows Wails portable build. Existing run `33582827394` passed
`go test -race -count=1 ./...`.

Run `33584052326` built Gotack, pinned Crush, and the `memory`, `skills`,
`recall`, `guard`, and `office` helpers, then assembled artifact
`gotack-windows-amd64`. The downloaded artifact's SHA-256 was
`cb0dd28b54cc754e144a059012a3db6da1cea472ab1a2bc9a575e790e9efc474`,
matching GitHub's recorded digest. Independent ZIP inspection found exactly
these non-empty files:

```text
gotack.exe
README.txt
resources/crush.exe
resources/guard.exe
resources/memory.exe
resources/office.exe
resources/recall.exe
resources/skills.exe
```

The remaining proof is deliberately manual and credentialed: launch the
packaged application and exercise an accepted foreground turn, due review
creation, guarded tool use, cleanup, learned-skill discovery, and bounded recall
output.

## Decisions

- 2026-09-02: Pin parity review to official commit
  `ab9866bc64df48281a2d929dfb1dfd1001973d24`; do not extrapolate from older
  local prose.
- 2026-09-02: Preserve the pinned action surface—`create`, `patch`, `delete`,
  `write_file`, `remove_file`—rather than inventing `revert` or `manage`.
- 2026-09-02: Keep `.archive` only as hidden recovery for verified background
  consolidation; do not expose archive/restore actions or an autonomous
  archival mechanism.
- 2026-09-02: Keep `skill_view` as the writer-process safety handshake while
  Crush's injected catalog and canonical `view` remain authoritative for
  discovery and ordinary reads.
- 2026-09-02: Keep Gotack-specific work only where the Crush boundary requires
  an adapter: detached session, routed digest, review roster, MCP registration,
  and read-only recall index.
- 2026-09-02: Resolve PR #11 on top of backend-hardening commits
  `b2cc47b7c0644c83dfa25ee5783e5ab67ae46291` and
  `92e1c5e815571ea0609c5d4ef3acf32714305336`, retaining their centralized
  attachment handling, build-tagged assets, static analysis, and plan history.

## Result

Implementation and automated proof are complete on PR #11. The plan is ready
for the credentialed packaged-app smoke path and remains active solely because
this environment has no external model credentials or interactive packaged-app
session. No automated result is substituted for that live proof.
