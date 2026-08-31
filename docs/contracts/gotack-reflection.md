# Gotack Reflection Contract

Gotack runs bounded reflection agent jobs from the desktop host
(`internal/reflection`, wired in `reflection_host.go`). Reflection is the
Phase 6 learning loop of `docs/plans/completed/hermes-parity-harness.md`: the
host watches completed runs, and when a gate opens it launches one short
engine run whose only job is to distil durable lessons into the persistent
memory files through the gotack-memory MCP server (D3,
`docs/decisions/0003`). The unattended posture it depends on is
`docs/contracts/gotack-approvals.md`; the memory write path it is confined
to is `docs/contracts/gotack-memory-mcp.md`.

The desktop never executes agent logic itself (ADR 0001): a firing is one
agent run submitted over the same REST path the UI uses, and its outcome
arrives over the same SSE stream. No Wails-bound methods exist for
reflection in this phase: there is no UI surface yet, and hard rule 8
forbids bound methods nothing consumes.

## Triggers and gates

Reflection consumes the same `run_complete` SSE event the scheduler books,
routed through `App.RunDone`, plus session deletion as the session-end
signal. A gate opening starts at most one bounded run; gates never queue.

| Gate | Opens when | Rationale |
| --- | --- | --- |
| Turn threshold | 8 completed turns accumulated for one session (errored and cancelled runs never count) | learning happens after substantive use, not every turn |
| Session end | the session is deleted with at least 1 completed turn, once per session | a closed conversation is a natural review point; the firing runs BEFORE the delete so the source conversation is still readable |

A refused or failed launch is logged at debug level and never surfaced: the
reflection loop must not disturb the event stream that feeds it. An
explicit user command (a `/learn` surface) is deferred: no UI seam exists
yet this phase, and the two event gates already cover the plan's 6.1
intent.

## Budget and in-flight guard

- **Hourly budget**: at most 1 reflection launch per sliding one-hour
  window, host-wide. The budget counts launched runs; preflight skips
  consume nothing.
- **In-flight guard**: a second gate never opens while a reflection run is
  still in flight; the flight clears when its `run_complete` arrives
  (or when a launch fails after the claim).
- **Preflight skip**: when no model is configured (`config.json`), firings
  are skipped instead of burning a failed run.

## Recursion guard

Reflection runs are engine runs, so they emit `run_complete` too. Every
reflection session is created with the title `Reflection: <source session
id>` and tagged in the tracker; completion events from tagged sessions are
ignored by construction, so the loop can never feed itself. A tagged
completion also clears the in-flight guard and never counts toward any turn
threshold.

## Firing semantics

One firing = one agent run, in this pinned order (same shape as
`docs/contracts/gotack-schedule.md`):

1. Create a session through `internal/session`
   (`POST /v1/workspaces/{id}/sessions`), in the currently active
   workspace, titled `Reflection: <source session id>`.
2. Mark the session id in the unattended roster **before** any prompt runs
   (`guard.MarkUnattendedSession`), exactly like a scheduled or Zalo run.
3. Submit the reflection prompt (`POST /v1/workspaces/{id}/agent`).

A failed mark aborts the firing and releases the claim. A launch timeout of
30 s bounds the whole sequence; the session-end gate bounds itself with a
15 s context so a refused or slow launch never blocks the delete that
follows it.

## Memory routing (D3)

The reflection prompt instructs the run to review the source session and
record durable lessons **only** through the gotack-memory `memory` tool
(view/add/replace), and to never write context files directly or touch
MEMORY.md/USER.md with write, edit or shell tools. This keeps every memory
write on the D3-sanctioned path: caps, atomic writes, provenance and the
denial of every other write path are enforced by `internal/memory`, not by
the reflection run's goodwill. Reflection proposes; the memory tool decides.
There is no interactive approval step for memory writes by construction
(D3) — the unattended posture below is the second stop.

## Unattended posture linkage

Every reflection session is recorded in
`<appconfig.Dir()>/unattended-sessions.json` before its prompt runs, so the
guard applies the unattended posture of `docs/contracts/gotack-approvals.md`:
reads and in-root writes are pre-approved, ask-tier operations are denied
with rule `unattended-approval`, and the blocklist floor always applies. A
reflection run therefore either proceeds or fails legibly — it never hangs.

## Removal

Delete `internal/reflection` and `reflection_host.go`, and drop the
`startReflection`/`RunDone`/`sessionEnded` wiring in `app.go` and
`bind_session.go`. Nothing persists: tracker state is in-memory only, no
config keys are written, and there is no `RemoveConfigField` undo to
perform. Reflection sessions are ordinary Crush sessions titled
`Reflection: …`; remove them through the UI if wanted. Their unattended
roster entries are shared with schedule and Zalo, are capped, and can be
deleted freely.
