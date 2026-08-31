# 0001 Narrow "no agent logic in the desktop layer" to named responsibilities

Date: 2026-08-31

## Status

Accepted

## Context

`AGENTS.md` hard rule 4 reads: "No agent logic in the desktop layer. Crush
owns agent execution, sessions, permissions, LSP, MCP and persistence."

The `hermes-parity-harness` plan's Phases 2–6 place memory curation
(`cmd/memory`), read-only history indexing (`cmd/recall`), scheduling
(`internal/schedule`), and post-run reflection in the desktop layer. As
written, rule 4 forbids all of it, so the plan is invalid unless the rule
changes. `docs/WORKFLOW.md` requires authority for new externally observable
policy before editing; the user explicitly delegated these decisions on
2026-08-31.

## Decision

Narrow hard rule 4 to prohibit exactly the things that would duplicate the
engine: the agent turn loop, tool dispatch, message/session persistence, and
permission adjudication. The narrowed rule states that desktop-layer services
may schedule runs, curate seeded context files, index engine history
read-only, and reflect on completed runs — always over the REST + SSE
boundary, never by importing `third_party/crush/internal/...`.

The `AGENTS.md` text change lands in the same change as the first Phase 2
code that needs it, and the invariant checker is not involved.

## Alternatives Considered

1. Keep rule 4 as written and drop Phases 2–6. Rejected: it abandons the
   plan's core outcome (Hermes-class memory, recall, scheduling, learning)
   even though every capability has a verified supported seam.
2. Move the capabilities into a Crush fork instead. Rejected: a fork carries
   permanent rebase cost against a fast-moving upstream, and none of
   Phases 2–6 requires touching the agent loop.

## Consequences

Positive:

- Phases 2–6 become permissible without weakening the rule's real intent:
  gotack still never implements turn execution, tool dispatch, persistence,
  or permission decisions.

Tradeoffs:

- The boundary is now a list of named responsibilities rather than one
  sentence, so future proposals must be checked against the list.

## Follow-Up

- Apply the `AGENTS.md` amendment together with the first Phase 2 change.
- `docs/plans/active/hermes-parity-harness.md` Phase 0 consumes this record.
