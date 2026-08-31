# Gotack Schedule Contract

Gotack runs scheduled autonomous agent jobs from the desktop host
(`internal/schedule`, wired in `schedule_host.go`). This is a host-internal
capability with one external boundary: the persisted job file
`schedule.json`, which users hand-edit while the app is closed. The phased
authority is Phase 5 of `docs/plans/active/hermes-parity-harness.md`; the
unattended posture it depends on is `docs/contracts/gotack-approvals.md`.

The desktop never executes agent logic itself (ADR 0001): a firing is one
agent run submitted over the same REST path the UI uses, and its outcome
arrives over the same SSE stream. No Wails-bound methods exist for the
scheduler in this phase: there is no UI surface yet, and hard rule 8 forbids
bound methods nothing consumes.

## schedule.json schema

Location: `<appconfig.Dir()>/schedule.json`, matching the `zalo.json`
convention. Written atomically (temp file plus rename) so a crash mid-write
never leaves a truncated file. The host is the only writer while the app
runs; edit the file with the app closed and changes apply on the next start.

```json
{
  "jobs": [
    {
      "id": "daily-digest",
      "name": "Daily digest",
      "prompt": "Summarise the workspace changes since yesterday.",
      "at": "08:30",
      "enabled": true,
      "hourly_budget": 1,
      "last_run": "2026-09-01T08:30:02+07:00",
      "last_outcome": "complete",
      "recent_fires": ["2026-09-01T08:30:02+07:00"]
    }
  ]
}
```

| Field | Owner | Meaning |
| --- | --- | --- |
| `id` | user | unique per job; required |
| `name` | user | session title for launched runs (`Schedule: <name>`, or `Schedule: <id>` when empty) |
| `prompt` | user | text submitted to the engine; required |
| `every` | user | Go duration (`30m`, `2h`), minimum 1m; fires repeatedly. Exactly one of `every`/`at` |
| `at` | user | local 24h `HH:MM`; fires once a day. Exactly one of `every`/`at` |
| `enabled` | user | `false` jobs never fire; the host also writes this when it disables a job |
| `hourly_budget` | user | max launches per sliding hour; `0` or absent means the default (2) |
| `last_run` | host | last successful claim of a firing; the interval guard reads it |
| `last_outcome` | host | most recent result: `fired`, `complete`, `cancelled`, `run failed: …`, `launch failed: …`, `skipped: …` |
| `consecutive_failures` | host | failure strikes toward the disable threshold |
| `disabled_reason` | host | set when the host disables a job; cleared if the user re-enables it |
| `recent_fires` | host | launch timestamps inside the budget window; older entries pruned on save |

Validation errors name the offending field. A malformed file never crashes
the host: the scheduler runs with no jobs, the file stays on disk, and
scheduling resumes on the next start once it parses again.

## Firing semantics

One firing = one agent run, in this pinned order:

1. Create a session through `internal/session` (`POST /v1/workspaces/{id}/sessions`),
   in the currently active workspace.
2. Mark the session id in the unattended roster **before** any prompt runs
   (`guard.MarkUnattendedSession`), exactly like a Zalo turn.
3. Submit the prompt (`POST /v1/workspaces/{id}/agent`).

A failed mark aborts the firing and records a failure: an unmarked scheduled
session could hang on an approval prompt nobody can answer (ADR 0002, plan
5.4). The claim (`last_run`, `last_outcome: "fired"`) is persisted before any
network call, so the interval guard survives restarts even if the host dies
mid-launch; a failed launch rolls the claim back.

Duplicate and window guards:

- A job never fires while its previous run is still in flight (matched by
  session id until `run_complete` arrives).
- A job never re-fires before `last_run + every` (interval jobs) or before
  tomorrow's occurrence (time-of-day jobs already run today).
- A missed time-of-day occurrence (host down at `HH:MM`) catches up once when
  the engine next becomes ready; missed interval slots are not replayed — at
  most one catch-up firing per job, so a long outage never causes a burst.

Engine readiness is pushed to the scheduler by the connection flow
(`SetEngineReady` from the attach commit, transport loss and stop paths) —
never polled. While the engine is down, due firings defer without counting
failures, and the transition back to ready re-evaluates due jobs
immediately. Outcomes ride the existing `run_complete` SSE event through the
host's `DoneSink`; hard rule 5 is honoured: nothing here polls the engine.

## Budget and failure policy

- **Hourly budget** (per job): at most `hourly_budget` launches per sliding
  one-hour window, default 2. The budget counts launched runs; a launch that
  fails before a run starts is bounded by the retry policy below instead.
- **Preflight skip**: when no model is configured (`config.json`), firings
  are skipped with `last_outcome: "skipped: …"` instead of burning a failed
  run (plan 5.1). Skips never count as failures.
- **Retry**: a failed launch is recorded, rolls back the interval claim, and
  retries after a 5-minute backoff. Failures are never silently dropped —
  `last_outcome`, `consecutive_failures` and the log carry them.
- **Failure-nudge threshold**: 3 consecutive failures (launch failures and
  run errors from `run_complete` both count; cancellations do not) disable
  the job with a legible `disabled_reason`. Re-enabling requires a hand
  edit: set `"enabled": true` (the streak and reason reset on load).
- The evaluation loop ticks every 30 s; time-based firing has no event
  source, so this internal clock is the only timer in the package.

## Unattended posture linkage

Every scheduled session is recorded in
`<appconfig.Dir()>/unattended-sessions.json` before its prompt runs, so the
guard applies the unattended posture of `docs/contracts/gotack-approvals.md`:
reads and in-root writes are pre-approved, ask-tier operations are denied
with rule `unattended-approval`, and the blocklist floor always applies. A
scheduled run therefore either proceeds or fails legibly — it never hangs.

## Removing everything

Delete `<appconfig.Dir()>/schedule.json` (or set `"jobs": []`, or set
`"enabled": false` per job) and restart the app: the scheduler runs with no
jobs and creates no state of its own beyond that file. Scheduled sessions
are ordinary Crush sessions; remove them through the UI if wanted. Their
unattended roster entries are shared with Zalo, are capped, and can be
deleted freely. Nothing in the scheduler writes Crush config keys, so there
is no `RemoveConfigField` undo to perform.
