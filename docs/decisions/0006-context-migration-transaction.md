# 0006 Context Migration Is a Transaction

Date: 2026-09-05

## Status

Accepted

## Context

ADR 0005 fixed the ownership model: `TACK_CORE.md` is product-managed and
`USER.md` is user-owned, replacing the single legacy `TACK.md`. Retiring that
legacy file is not a rename. It removes user-visible content from the prompt
snapshot, and the host seeds context on every startup. A naive
"write new files, delete old file" sequence leaves unrecoverable states:
a crash after deleting `TACK.md` but before the layered files are written
silently drops the user's context; a crash mid-write leaves a half-written
prompt; a reseed that reruns before the user has seen the change can destroy
the only copy of customized legacy content. ImplementPlan.md section 0.5
requires the migration to be a transaction, not a chain of independent
renames, and section 6 requires modified legacy content to migrate only after
explicit approval.

## Decision

The legacy-to-layered migration is one durable state machine in
`internal/contextseed` with five persisted modes — `legacy`, `pending`,
`staged`, `committed-layered`, `rolled-back` — recorded in a single
`context-migration.json` state file:

1. Legacy `TACK.md` whose hash matches a stock entry in the versioned
   `stock-manifest.json` is auto-migrated. Modified or unknown legacy content
   is never auto-migrated; the state becomes `pending` and the desktop UI
   presents a preview with a three-way candidate for explicit acceptance.
2. Every accept first stages the target core and user files in a
   token-addressed staging directory, persists the `staged` state (including
   the previous mode and expected hashes), and only then retires the legacy
   file into a content-addressed backup and commits.
3. Startup and preview recover an interrupted `staged` transaction: with a
   valid backup the migration completes; without one it fails loudly instead
   of guessing.
4. Every commit keeps a backup plus rollback token that survives reseeds.
   Rollback restores the exact backed-up bytes and marks `rolled-back`; the
   same content can be migrated again afterwards.
5. Accept and rollback are compare-and-swap on the monotonically increasing
   `generation` plus the previewed hashes. A concurrent edit between preview
   and confirmation fails with `context migration changed since preview`
   instead of overwriting unseen changes.

The desktop surface (Settings, "Ngữ cảnh" tab) mirrors this machine: it shows
the mode, offers the conflict-resolving preview for pending states, and offers
rollback only when a `backup_token` exists.

## Alternatives Considered

1. Rename `TACK.md` and append the managed core — rejected: two policy owners
   would enter the prompt snapshot, which ADR 0005 forbids, and generic rules
   would keep duplicating the engine template.
2. Migrate in place without a stage directory — rejected: a crash between the
   legacy deletion and the layered write loses the user's context with no
   recovery path.
3. Wipe unknown legacy content on first layered startup — rejected: destroys
   user customization; the plan requires explicit approval for modified
   legacy.

## Consequences

Positive:

- Every migration path has a durable backup, an atomic commit, and an explicit
  rollback (ImplementPlan.md section 10 PR4 invariant).
- Crash recovery is deterministic: staged transactions resume or fail loudly,
  never silently.
- The generation counter makes concurrent UI edits and second instances safe.

Tradeoffs:

- The state file, staging directory, and backups add on-disk bookkeeping that
  must stay out of the prompt snapshot.
- Migration state must be loaded under one lock per operation so the snapshot
  is consistent (ImplementPlan.md section 0.5).

## Follow-Up

- Keep the stock manifest versioned; new stock bases ship with the release
  that changes them.
- Portable-flow UI verification (agent-browser run over the packaged app)
  remains a release-gate item for the migration surface.
