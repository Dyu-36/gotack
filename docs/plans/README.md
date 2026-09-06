# Execution Plans

Execution plans are Git-native working memory for complex tasks. They preserve
enough context for another agent or human to resume work without reconstructing
intent from chat history or a partial diff.

## When To Create A Plan

Use an ephemeral plan for bounded, single-session work.

Create one durable plan when work spans sessions, coordinates contributors, has
meaningful dependencies or ordering, requires recovery steps, or would be unsafe
to resume from the diff alone.

Use `docs/templates/exec-plan.md` and place the file under `active/`.
For an explicitly authorized baseline-to-rerun Harness experiment, use
`docs/templates/harness-improvement.md` instead.

## Lifecycle

```text
docs/plans/active/<slug>.md
  -> update progress and decisions during implementation
  -> record final validation and result
  -> move to docs/plans/completed/<slug>.md
```

The plan is the primary task artifact. Promote a lasting product or architecture
decision into `docs/decisions/`; keep task-local choices in the plan.

## Active Plans

- [`windows-release-completion.md`](active/windows-release-completion.md)
  — canonical remaining-work plan: CI, provider telemetry, snapshot integrity
  and reader lifecycle, Fantasy dependency integration, Windows/live/Hermes
  acceptance, real baseline with cache OFF, and consolidated release gates.
- [`hermes-learning-loop.md`](active/hermes-learning-loop.md),
  [`input-pipeline-upgrade.md`](active/input-pipeline-upgrade.md),
  [`ip-01-first-byte-to-first-sse.md`](active/ip-01-first-byte-to-first-sse.md),
  [`ip-02-snapshot-reader-lifecycle.md`](active/ip-02-snapshot-reader-lifecycle.md)
  — supporting implementation/evidence history. Remaining work and closure
  are owned by the canonical plan above, not by independent checklists.

## Recently Completed

- [`thermo-nuclear-maintainability-hardening.md`](completed/thermo-nuclear-maintainability-hardening.md)
  — hardened backend boundaries, removed confirmed dead code and stale artifacts,
  made clean-checkout validation reproducible, expanded CI/static analysis, and
  consolidated the repository to the single `main` branch. Completed 2026-09-02.
- [`hermes-parity-harness.md`](completed/hermes-parity-harness.md)
  — bring Hermes-class memory, cross-session recall, graduated approvals,
  scheduling and a learning loop onto the Crush core, and relocate the
  Tack/Sage persona out of the vendored checkout into a tracked, released
  seam. Completed 2026-09-01 with all seven phases and the carried-over
  hygiene items landed on `main`.
- [`cleanup-dead-code-and-doc-drift.md`](completed/cleanup-dead-code-and-doc-drift.md)
  — dead state, duplicated blocks and documentation drift in `package main`.
  Closed 2026-08-31 after a third-pass re-audit; the seven items that were
  still open moved into the carry-over section of the active Hermes plan, so
  one plan owns their ordering.
