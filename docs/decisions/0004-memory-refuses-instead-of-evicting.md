# 0004 Memory refuses an overflowing write instead of evicting

Date: 2026-09-01

## Status

Accepted

## Context

The first memory implementation enforced its caps by eviction: when a write
pushed `MEMORY.md` past 2200 characters, the store deleted the oldest entries
until the file fit, kept the new entry, and reported how many it dropped.
Current Hermes does the opposite: over cap the tool refuses the write and
returns the unchanged entries and usage needed for one corrective
consolidation batch. The parity review is pinned to the upstream commit in
`docs/plans/active/hermes-learning-loop.md`, not to the retired local design
note.

Eviction is also wrong on its own terms. Memory holds the durable facts the
assistant is supposed to carry between sessions; the oldest entry is often
the most stable one, a standing preference, while the newest is the most
likely to be noise from the task at hand. Deleting silently inside a tool
call the user never sees is unrecoverable: the entry is gone from a plain
file that no history tracks.

`docs/decisions/0003-memory-writes-constrained-by-construction.md` already
described the intended behaviour — "a write that would exceed the cap is
rejected with an actionable error naming the cap, forcing consolidation" —
but the implementation and its contract had drifted into eviction. This
record settles the behaviour and makes it testable.

## Decision

Over cap, the memory tool refuses the write and deletes nothing:

- The file on disk is left unchanged.
- The error reports current/proposed usage and lists every unchanged entry, so
  the model can remove or shorten entries and add the new entry in one atomic
  batch without a read action.
- Exactly at the cap is accepted; one character over is refused.
- Successful writes remain compact and do not echo the stored entries.

The store therefore has no eviction code. The behavior is pinned by
`TestCapsCountUnicodeCharactersAndDelimiter`,
`TestAtomicBatchUsesFinalBudgetAndIsAllOrNothing`, and
`TestToolErrorsAreStructuredAndExposeStateOnlyWhenActionable`.

## Alternatives Considered

1. Keep eviction but surface it in the UI. Rejected: it still destroys the
   user's knowledge without consent; reporting an irreversible deletion does
   not make it safe.
2. Raise the caps instead. Rejected: the caps are the prompt-cost budget
   copied from Hermes; raising them moves the same failure into the context
   window, where it is harder to see.
3. Refuse with a bare error and no payload. Rejected: without the current
   entries the model cannot consolidate in the same turn, so it retries the
   same oversized add.

## Consequences

Positive:

- No stored fact is lost unless the model explicitly removes it.
- The refusal is actionable within one turn, which is what makes a hard cap
  workable for a tool that has no read action.

Tradeoffs:

- A model that ignores the refusal can wedge itself: every add fails until it
  consolidates. The unchanged-entry payload and atomic batch are the
  mitigation.
- Staying under the cap now depends on the model merging entries; the store
  alone can no longer make room.

## Follow-Up

- `docs/contracts/gotack-memory-mcp.md` carries the policy and the tests
  that pin it.
- `docs/plans/active/hermes-learning-loop.md` records its current
  implementation and proof.
- Decision 0003 stays in force; this record only settles the cap behaviour
  it described.
