# Contract: Hermes background review

Status: implemented by `internal/reflection`, `reflection_host.go`, the
existing Crush REST/SSE integration, and `internal/guard`.

Gotack maintains two bounded learning cadences and, when either is due,
launches one detached Crush session to update persistent memory and/or the
managed skill library. The desktop host owns counters and lifecycle only;
Crush remains the sole agent-turn executor.

There is no literal nudge appended to user prompts, no UI binding, and no
session-deletion review trigger.

## Cadence and final-response gate

| Review target | Due condition | Persistence |
| --- | --- | --- |
| Memory | Every 10 user prompts successfully accepted by Crush. | On first use/restart, hydrate from persisted user-message count modulo 10. |
| Skills | Every 10 unique assistant/model messages observed on SSE across turns. | Process-local. A message containing `skill_manage` resets the counter. |

The counters are evaluated on `run_complete`. A due flag is consumed even
when that run errored, was cancelled, or produced no final text, but only a
successful non-empty final response may launch a review. If both cadences are
due, one review handles both.

Only one launch/review may be in flight host-wide and gates do not queue. A
new foreground user turn cancels pending background work before its prompt is
sent. Review completions are recognized by session id, never advance either
cadence, and trigger cleanup instead of another review.

Scheduled runs never launch background review and their model-iteration state
is discarded on completion, matching Hermes' `skip_background_review` cron
path and avoiding a second autonomous token spend.

## Review session and prompt

The host performs this bounded sequence:

1. Snapshot the source session transcript through the REST service.
2. Create a session titled `Background review`.
3. Add its id to `<appconfig.Dir()>/review-sessions.json` before any tool can
   run.
4. Build and send the review prompt through the normal Crush agent endpoint.

Preflight requires a configured model and whichever `memory`/skills binary
the due targets need. Launch is bounded by 30 seconds. The review uses the
currently configured engine model; no separate model-selection path is
implemented.

Because the detached session cannot reuse the source process's warm prompt
cache, its snapshot contains exactly the latest 24 transcript items. User and
assistant text are normalized to one-line previews capped at 300 Unicode
runes. Tool-role text and every hydrated tool result are capped at 200 runes;
tool names remain in bounded assistant metadata. Truncation is explicit and
rune-safe. The adapter does not extend the tail or pass an unbounded recent
message merely because it is near the review boundary.

The memory prompt asks only for durable user facts/preferences. The skill
prompt actively prefers correcting a relevant loaded agent-owned skill, then
an existing class-level skill, and creates a new umbrella only when no
existing one fits. It excludes transient failures, unresolved attempts, and
one-session narratives. When both cadences are due, a dedicated combined
prompt evaluates both dimensions before allowing `Nothing to save.`; it cannot
stop after the memory check without considering skills.

The review is cancelled at 16 assistant iterations only when the 16th message
still requests tools. A normal final response at that boundary is allowed to
complete. On completion or failed launch, the host removes the review roster
entry and deletes the detached session. Application shutdown first cancels and
deletes any detached review while the REST link is still available, then
disconnects from Crush, so a hidden review is not stranded across exit.

## Guard boundary

The dedicated review roster selects a stricter PreToolUse posture. The
destructive-command blocklist remains the first rule. The review allowlist is:

- `memory`;
- `skill_view` (the host-side read-proof handshake) and `skill_manage`;
- local `ls`, `glob`, `grep`, and `view` equivalents of read/search tools.

Shell execution, workspace writes, network-backed search, delegation, and
unknown tools are denied by rule `background-review-tool-whitelist`.

For every Gotack skill call, the hook overwrites hidden `_session_id` and
`_background_review` fields. The skill server then enforces background
ownership and fresh-read rules. The memory server independently enforces its
content scan, cap, lock, and atomic write contract.

## Adaptation boundary

Hermes' post-turn reviewer is represented as a detached Crush session because
the current REST boundary has no session-fork operation. The routed digest,
strict review roster, cleanup, and cancellation keep that adaptation bounded;
the host does not reproduce the agent loop or tool dispatcher.

To remove the feature, remove `internal/reflection`, `reflection_host.go`, the
tracker callbacks in `app.go`/`bind_session.go`/`internal/uievents`, and the
review-roster branch in `internal/guard`. Memory, skills, and recall remain
independently usable MCP tools.
