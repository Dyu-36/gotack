# Execution Plan: Thermo-Nuclear Maintainability Hardening

Date: 2026-09-02

## Status

Active

## Outcome

The current main worktree builds and runs from one coherent source snapshot;
accepted security and persistence contracts match the implementation; confirmed
dead code and stale narration are removed; high-confidence structural findings
are simplified without changing accepted behavior; all repository gates pass;
the result is pushed to `origin/main`; and merged auxiliary branches/worktrees
are removed so `main` is the only remaining branch.

## Context

- `AGENTS.md` and `docs/WORKFLOW.md` define ownership, contract, planning, and
  validation requirements.
- `docs/decisions/0002-approval-posture-pretooluse-hook.md` requires interactive
  approval by default and makes auto-approval an explicit opt-in.
- `docs/decisions/0003-memory-writes-constrained-by-construction.md` requires
  bounded, scanned, cross-process-safe atomic writes. The memory contract
  explicitly migrates legacy provenance wrappers away and stores new entries
  as plain text.
- `docs/contracts/` is the external-boundary authority and must move with its
  implementation.
- The review snapshot contained concurrent staged, unstaged, and untracked work
  for the Hermes learning loop; filesystem state, not the index alone, is the
  implementation source of truth.

## Scope

In scope:

- Reconcile the current H1-H4/H6 work already present in the worktree into a
  buildable, documented, behavior-coherent change; do not leave staged-only
  halves of a feature.
- Fix confirmed blockers: approval default, Zalo attachment-only panic, memory
  representation/API drift, recall API/contract drift, and Gotack-owned Codex
  OAuth provider/refresh projection.
- Integrate or remove unreachable executables according to accepted product
  intent; package and register shipped MCP binaries consistently.
- Simplify high-confidence duplication/branching in provider transitions,
  OOXML parsing, bundle seeding, UI event observers, and timetable runtime where
  behavior-level proof exists.
- Remove confirmed internal dead code and stale/historical comments; correct
  layout, contracts, README, and documentation-map drift.
- Validate, commit, push `main`, then remove only clean, fully merged auxiliary
  worktrees and local/remote branches.

Out of scope:

- `frontend/`, `build/`, and a general audit/refactor of `scripts/`, as requested.
  Canonical Crush patch files or build wiring under `scripts/` may change only
  when required to make an in-scope Gotack-owned patch reproducible.
- Unmodified upstream Crush debt outside Gotack's tracked patch, including
  upstream files already over 1,000 lines.
- New product work for untouched Hermes phases H3/H5/H7/H8.
- Live Zalo credentials, branch-protection configuration, or upstream protocol
  freshness that cannot be proven locally.

## Approach

1. Wait for concurrent writes to settle, capture one status/diff snapshot, and
   preserve all user-owned changes.
2. Land independent safety/dead-code fixes with focused tests.
3. Reconcile the dirty Hermes feature set and its contracts as one graph.
4. Fix the Gotack-owned Crush patch and resource/runtime structural findings.
5. Apply cross-cutting simplification and documentation cleanup, adding only
   the smallest repository-native guards backed by accepted authority.
6. Run focused, whole-module, invariant, static, packaging, and application
   build checks; repair failures rather than weakening validation.
7. Commit and push to `origin/main`; after verifying every auxiliary ref is
   merged and every worktree is clean, remove those worktrees and delete the
   merged local/remote branches.

## Risks And Recovery

- Concurrent writers can overwrite or invalidate a review finding. Mitigation:
  do not edit a moving file; re-read status and source immediately before each
  workstream and re-run the full diff review afterward.
- The nested Crush checkout is ignored by the root repository. The tracked
  patch is canonical; regenerate/verify it and prove a fresh apply against the
  pinned commit. Never rely on an untracked nested edit alone.
- Crush now provides atomic config-field batches and typed model-pair
  mutations. Use those for related engine state, validate before the first
  write, and keep cross-store transitions dependency-ordered because Crush,
  Gotack's settings file, and the credential store do not share a transaction.
- Branch cleanup is destructive. Record ref SHAs, require clean worktrees and
  `--merged main`, push `main` first, and never use force deletion.
- Recovery before push is the working-tree diff and Git index; after push the
  final commit on `main` is the recovery point. No hard reset is permitted.

## Progress

- [x] Complete parallel read-only thermo-nuclear review.
- [ ] Capture a stable implementation snapshot.
- [ ] Fix safety, correctness, and confirmed dead-code findings.
- [ ] Reconcile Hermes feature implementation and contracts.
- [ ] Fix Gotack-owned Crush patch and resource findings.
- [ ] Complete structural simplification and comment/doc cleanup.
- [ ] Run focused and repository-wide validation.
- [ ] Commit and push `origin/main`.
- [ ] Remove clean merged worktrees and auxiliary local/remote branches.

## Decisions

- 2026-09-02: `main` is the sole retained branch because `origin/HEAD` points to
  `origin/main` and the user explicitly requested a single main/master branch.
- 2026-09-02: Review findings are actionable only when backed by code, graph,
  tests, or accepted docs; staged-only and concurrent dirty states are labeled
  and revalidated before edits.
- 2026-09-02: Accepted ADR/contract behavior wins over contradictory comments
  or defaults. Product behavior is not inferred from an unfinished helper.
- 2026-09-02: The latest-doc audit uses upstream Hermes `main` at
  [`ab9866bc64df48281a2d929dfb1dfd1001973d24`](https://github.com/NousResearch/hermes-agent/commit/ab9866bc64df48281a2d929dfb1dfd1001973d24).
  A direct Git diff confirms that its memory, session-search, skill-management,
  and background-review implementations are unchanged from the earlier audited
  commit `622883bad7f55f56a6393cd994e36c65fbdff253`. They define
  cadence-triggered background review, atomic memory batches with `new_text`
  compatibility, anchored session scroll, separate skill read tools, and an
  operations-array `skill_manage` schema with five advertised actions (the
  upstream flat `edit` shape is legacy compatibility). Gotack must reuse
  Crush's canonical skill catalog/read path where that preserves the local
  safety boundary; any safety-specific wrapper that remains must earn its
  additional surface explicitly.
- 2026-09-02: Cleanup candidates are the nine clean worktrees `wp1`–`wp9`
  owning `feat/phase1-persona-context`, `fix/skills-paths-merge`,
  `refactor/enginelink-activation`, `feat/phase2-memory`,
  `feat/phase3-recall`, `feat/phase4-approvals`, `feat/phase5-schedule`,
  `feat/phase6-learning`, and `chore/hygiene-close-plan`. All are currently
  merged into `main`; this must be reverified after the final push before any
  worktree or branch is removed.

## Validation

- Focused proof: package tests for every touched boundary, including negative
  cases for approval default, attachment-only Zalo input, malformed OOXML,
  memory legacy-representation migration/cap rollback, recall request
  conflicts, and Codex refresh plus provider projection.
- Integration proof: MCP tool contract tests, fresh application configuration
  wiring, Crush patch apply/reverse-apply checks, timetable solver fixture, and
  a Windows Wails build when local prerequisites permit.
- Repository-required checks: `gofmt`, `go test ./...`, `go vet ./...`,
  `staticcheck ./...`, `deadcode -test ./...`, generated event drift,
  repository invariants, frontend checks/build (validation only), and root
  `git diff --check`.
- Negative invariant proof observed before the timetable refactor:
  `node scripts/check-repository-invariants.mjs` rejected
  `resources/skills/timetable/runtime/solver.py` at 1,490 lines with the
  AGENTS.md hard-rule-6 diagnostic. Re-run for positive proof after splitting.

## Result

Pending implementation and validation.
