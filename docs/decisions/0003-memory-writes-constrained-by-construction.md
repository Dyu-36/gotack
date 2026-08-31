# 0003 Memory writes: no interactive approval, constrained by construction

Date: 2026-08-31

## Status

Accepted

## Context

Memory files (`MEMORY.md`, `USER.md`) are re-injected into the system prompt
on every turn, so anything written there is trusted instruction text.
Combined with the Zalo remote entry point, a prompt injection absorbed from a
web page or a remote message that reaches a memory write becomes a persistent
cross-session instruction — the most severe risk in the hermes-parity plan.
At the same time, unattended and scheduled runs cannot answer interactive
prompts, and a per-write prompt in an interactive session would be noise the
agent learns to route around. The user delegated this decision on 2026-08-31.

## Decision

The dedicated `memory` MCP tool writes without interactive approval, and the
safety is structural instead:

- Hard size caps per file; a write that would exceed the cap is rejected with
  an actionable error naming the cap, forcing consolidation.
- Atomic writes (temp file plus rename), so a crash can never leave a
  truncated file inside the system prompt.
- Provenance on every entry (timestamp, session id), so a poisoned entry is
  traceable and removable.
- The Phase 4 guard denies every *other* write path into
  `<appconfig.Dir()>/context/`: generic `write`/`edit`/shell tools cannot
  touch memory files, so the caps cannot be bypassed through a side door.

Recovery is trivial by design: memory files are plain text in a known
directory; deleting or editing them cleans the next turn.

## Alternatives Considered

1. Route every memory write through the `internal/permission` relay for
   approval. Rejected: scheduled and remote sessions cannot answer, and in
   interactive sessions the prompt cost outweighs the gain given the caps,
   provenance, and the write-path denial above.
2. Read-only memory (agent may not write at all). Rejected: it deletes the
   plan's core outcome — persistent self-editing memory.

## Consequences

Positive:

- Memory works unattended (a prerequisite for Phases 5 and 6) while every
  write is bounded, traceable, and reversible.

Tradeoffs:

- A poisoned entry can exist until noticed; provenance and caps bound the
  blast radius instead of preventing it absolutely.
- This decision must be revisited if a second remote entry point is added or
  the caps are raised materially.

## Follow-Up

- `docs/plans/active/hermes-parity-harness.md` Phases 2 and 4 implement this.
- Risk R2 in that plan carries the mitigation list.
