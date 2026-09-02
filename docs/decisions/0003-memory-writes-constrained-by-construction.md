# 0003 Memory writes: no interactive approval, constrained by construction

Date: 2026-08-31

## Status

Accepted

## Context

Memory files (`MEMORY.md`, `USER.md`) are re-injected into the system prompt
on every turn, so anything written there is trusted instruction text.
Combined with the Zalo remote entry point, a prompt injection absorbed from a
web page or a remote message that reaches a memory write becomes a persistent
cross-session instruction — the most severe risk in the learning loop.
At the same time, unattended and scheduled runs cannot answer interactive
prompts, and a per-write prompt in an interactive session would be noise the
agent learns to route around. The user delegated this decision on 2026-08-31.

## Decision

The dedicated `memory` MCP tool writes without interactive approval, and the
safety is structural instead:

- Hard size caps per file; a write that would exceed the cap is rejected with
  an actionable error naming the cap, forcing consolidation.
- Mutations can be submitted as one all-or-nothing batch, so
  consolidation does not expose an intermediate prompt state.
- Every candidate write is scanned for instruction override, exfiltration
  instructions, and hidden Unicode.
- A cross-process lock plus atomic replacement (temporary file, sync, close,
  rename) prevents concurrent lost updates and truncated prompt state.
- The guard denies generic file-writing tools aimed at
  `<appconfig.Dir()>/context/`, so normal agent edits cannot bypass the tool.

Recovery is trivial by design: memory files are plain text in a known
directory; deleting or editing them cleans the next turn.

## Alternatives Considered

1. Route every memory write through the `internal/permission` relay for
   approval. Rejected: scheduled and remote sessions cannot answer, and in
   interactive sessions the prompt cost outweighs the gain given the bounded
   tool and write-path denial above.
2. Read-only memory (agent may not write at all). Rejected: it deletes the
   plan's core outcome — persistent self-editing memory.

## Consequences

Positive:

- Memory works unattended while every write is bounded and reversible through
  the plain-text files.

Tradeoffs:

- A poisoned entry can exist until noticed; the scan and caps reduce the
  blast radius instead of preventing it absolutely.
- This decision must be revisited if a second remote entry point is added or
  the caps are raised materially.

## Follow-Up

- `docs/plans/active/hermes-learning-loop.md` records the current
  implementation and validation.
- `0004-memory-refuses-instead-of-evicting.md` settles the cap clause above:
  an overflowing write is refused and reports what is stored, and no entry is
  evicted. Every other clause of this record stands unchanged.
