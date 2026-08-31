# Execution Plan: Hermes-class harness on the Crush core

Date: 2026-08-31

## Status

Active — Phase 0 closed. D1, D2, and D3 are recorded in
`docs/decisions/0001`–`0003`, all three technical questions from the first
pass are resolved (step 1.6, step 4.3, risk R3), and the `.crush-pin`
consolidation is landed. The only condition on writing code is that the
`AGENTS.md` amendment from decision 0001 lands in the same change as the
first Phase 2 work.

## Outcome

Gotack gains the capabilities that make Hermes Agent distinctive — persistent
self-editing memory, cross-session recall, a learning loop, scheduled
autonomous runs, and graduated approvals — while keeping the REST + SSE
boundary to Crush intact.

Observable result, in order of proof strength:

1. The Tack/Sage persona is present in a released ZIP, not only on a developer
   machine. Today it is absent from the artifact (see Context).
2. A fact stated in session A is recalled by the agent in session B without the
   user repeating it.
3. The agent can answer "what did we decide about X last week" from prior
   sessions.
4. A destructive command is refused by policy, and the refusal names the rule.
5. A scheduled job runs unattended and delivers its result to a paired chat.

## Context

Verified by reading the tree on 2026-08-31 at Crush pin
`6d14dd93a9e526505f7de54ae5999431bc32a793`.

### The current prompt override is unsound

`third_party/crush/internal/agent/templates/assistant.md.tpl` is a **new
file** carrying the Tack persona, and `internal/agent/prompts.go` switches the
coder agent's embedded template to it. `task.md.tpl` carries the Sage
sub-agent persona plus newly added context-file blocks, and
`internal/agent/prompts_gotack_test.go` was added as a parity test. All of it
lives inside the vendored Crush checkout, which `README.md` records as "own
git history, ignored here; only third_party/README.md is tracked".

Verified by git on 2026-08-31: the checkout is detached at the pin with
**uncommitted working-tree modifications** — there are no local commits to
preserve — and the patch surface is eleven paths, not three:

- persona: `prompts.go` (embed switch), new `assistant.md.tpl`, patched
  `coder.md.tpl` (dead at runtime once the embed moved), patched
  `task.md.tpl`, new `prompts_gotack_test.go`;
- identity: `config.go` renames the agents Coder→"Assistant" and
  Task→"Sage";
- unrelated runtime changes: `server.go` drops `Protocols: &p` and adds a
  gzip handler (new `gzip.go`, `gzip_test.go`), and the MCP stdio process
  handling swaps `process_other.go` for a new `process_windows.go`.

Phase 1.4 must dispose of every one of these paths, or "clean checkout" is
not true.

Consequences, each independently checkable:

- `.github/workflows/release.yml` clones Crush fresh from
  `https://github.com/charmbracelet/crush.git` into `$RUNNER_TEMP\crush` and
  builds `crush.exe` from that tree. It never reads `third_party/crush`.
  **The released binary therefore carries upstream's stock coder prompt, not
  Tack/Sage.** A developer build via `scripts/update-crush.ps1` does carry it.
  Dev and release behaviour differ.
- `prompts_gotack_test.go` is in module `github.com/charmbracelet/crush`, while
  CI runs `go test ./...` in `github.com/Dyu-36/gotack`. The parity test has
  never executed in CI.
- `scripts/update-crush.ps1` runs `git checkout --detach $Commit`, so the next
  pin refresh conflicts with or discards the edited templates.
- It contradicts three stated authorities: `third_party/README.md` ("Upstream
  source is read-only for desktop needs; desktop-only behavior belongs in
  `internal/`"), `AGENTS.md` hard rule 4 ("No agent logic in the desktop
  layer"), and `README.md` ("`gotack` should not reimplement Crush internals").

Deleting the override is not the fix. Hermes is heavily prompt-engineered —
`prompt_builder.py`, `system_prompt.py`, and a stable/context/volatile tier
split. Reverting to upstream's coder prompt moves *away* from a general-purpose
assistant. The persona must be relocated to a tracked, supported seam.

### Crush already supplies the seams this plan needs

- `internal/config` `Options` exposes `options.context_paths`,
  `options.global_context_paths`, `options.skills_paths`, and
  `options.disabled_skills`. A per-agent `ContextPaths` field also exists but
  is dead: `prompt.promptData` reads only the `Options` values (see 1.6), and
  the `Agents` map is `json:"-"`, so agents cannot be redefined through
  `crush.json` or `SetConfigField`.
- `internal/agent/prompt/prompt.go` `Build` calls `promptData` on **every**
  prompt construction, re-reading every context file each time. A file rewritten
  between turns is picked up on the next turn with no restart. This is what
  makes file-backed memory viable without touching the agent loop.
- `processContextPath` walks directories recursively, and `expandPath` handles
  `~` and `$VAR`. A single seeded directory is enough.
- `internal/skills` discovers builtin plus `SkillsPaths` skills, dedups, filters
  by `DisabledSkills`, and renders `AvailSkillXML` into the prompt. Level-0
  descriptions in-prompt with `View`-on-demand for the body is already Hermes'
  progressive disclosure.
- `internal/hooks` exists, **but exposes exactly one event:**
  `EventPreToolUse = "PreToolUse"`. There is no post-tool, run-complete, or
  session-end hook. This invalidates any design that plans a learning loop on
  hooks; see Phase 6.

### Gotack already supplies the delivery patterns

- `internal/mcp` is a complete stdio MCP server (`Server{Name,Version,Tools}`,
  `Tool{Name,Description,Schema,Handler}`, protocol `2024-11-05`), and
  `cmd/office/` is a working precedent for shipping one.
- `office_seed.go` `registerOfficeTools` shows the exact registration route:
  `SetConfigField(ctx, wsID, ConfigScopeWorkspace, "mcp_servers.gotack-office",
  map[string]any{"command": …, "type": "stdio", "timeout": 30})`, plus `env` and
  `options.skills_paths`.
- `internal/officecli/seed.go` is the seeding pattern to mirror: `Seed`,
  `copyDirIfChanged` with a size-keyed `.seed-report.json`, `InstallPath`,
  `CrushEnv`, `SkillsPathArg`, rooted at `appconfig.Dir()`
  (`%AppData%\gotack` on Windows).
- `internal/crushapi` already consumes the `run_complete` SSE payload, which is
  the only available post-run signal.

### Known defect to fix in passing

`registerOfficeTools` writes `merged := []string{skillsPath}` into
`options.skills_paths`, overwriting rather than merging. Any second skills
directory added later would silently erase the Office skills.

### Crush persistence shape, for Phase 3

`internal/db/migrations/20250424200609_initial.sql` defines
`sessions(id, parent_session_id, title, message_count, prompt_tokens,
completion_tokens, cost, updated_at, created_at)` and
`messages(id, session_id, role, parts TEXT DEFAULT '[]', model, created_at,
updated_at, finished_at)`; message text lives as JSON in `parts`. Timestamps are
Unix milliseconds. `internal/db/datadirlock.go` holds an exclusive `flock` on
`{dataDir}/crush.lock` for the lifetime of the running engine.

### Overlap with the existing active plan

`docs/plans/active/cleanup-dead-code-and-doc-drift.md` (status: applied) already
owns item B3 (collapse the Crush pin into one tracked file), C2 (extract the
supervisor into `internal/enginelink`), and D (add a `gofmt` gate). This plan
consumes B3 rather than re-deciding it, and must not race C2.

## Scope

In scope:

- Relocating the Tack/Sage persona to a gotack-owned, tracked, released seam.
- Persistent memory, cross-session recall, graduated approvals, scheduling, and
  a learning loop, all built in `internal/` and `cmd/` over REST + SSE.
- The contract documents and tests each of those requires.

Out of scope for this plan:

- Prompt tiering and context compression, multi-backend terminals
  (docker/ssh/sandbox), and an auxiliary background-review model. These require
  changing the agent loop itself; see Phase 7.
- Porting Hermes' Python implementation. Parity of capability is the goal, not
  parity of code.
- Generalising the Zalo bridge into a multi-platform gateway.

## Approach

Seven phases. Phase 0 is a gate, Phases 1–6 are ordered by
capability-gained-per-unit-of-risk, Phase 7 is explicitly deferred.

The load-bearing property of this ordering: **Phase 1 removes the only current
reason to patch Crush, and Phases 2–6 add no new ones.** If Phase 7 is never
started, gotack never needs a Crush fork at all.

### Phase 0 — Authority gate (no code)

Blocking. `docs/WORKFLOW.md` requires identifying authority for new externally
observable policy before editing, and states that configurable defaults are not
authority. Three policies below are new and externally observable, so they need
decision records, not defaults.

- **D1 — amend `AGENTS.md` hard rule 4.** Settled: see
  `docs/decisions/0001`. Rule 4 is narrowed to prohibit the turn loop, tool
  dispatch, message persistence, and permission adjudication; the amendment
  lands with the first Phase 2 code.
- **D2 — approval posture.** Settled: see `docs/decisions/0002`. One
  `PreToolUse` hook carries deny/ask/auto, the blocklist ships first, and
  remote and scheduled sessions default to the stricter posture.
- **D3 — memory as trusted context.** Settled: see `docs/decisions/0003`.
  No interactive approval; safety is caps, atomic writes, provenance, and
  denial of every write path except the dedicated tool.

Also in Phase 0, and mechanical — **done 2026-08-31**:

- Consumed cleanup-plan item B3: the Crush SHA now lives in the single
  tracked file `.crush-pin`. `ci.yml` and `release.yml` load it into
  `CRUSH_COMMIT` via a "Load Crush pin" step, `scripts/update-crush.ps1`
  reads it when `-Commit` is omitted, `third_party/README.md` and `README.md`
  reference it, and `scripts/check-repository-invariants.mjs` `checkCrushPin`
  validates the owning file plus a drift guard that fails if any retired
  location grows a hardcoded SHA again. The checker passes.
- Register this plan in the `## Active Plans` list in `docs/plans/README.md`.

Exit criteria: D1, D2, D3 recorded (done — `docs/decisions/0001`–`0003`);
`.crush-pin` consolidation landed (done); `node
scripts/check-repository-invariants.mjs` passes (done).

### Phase 1 — Relocate the persona, delete the upstream patch

Goal: the persona ships in the release ZIP, is tracked in git, survives a pin
bump, and requires zero changes to Crush.

**1.1 Extract the persona into tracked assets.**

Create `resources/context/TACK.md` from the current vendored
`assistant.md.tpl`, keeping only the prose policy: the opening role sentence,
`## Core Principles`, `## Task Management`,
`## Technical Capabilities and Environment` and its subsections,
`## Implementation Methodology`, `## Tool Selection`, `## Operating Behavior`.

**Critical detail:** context files are read as raw text — `processFile` does
`os.ReadFile` and no template execution. Every Go template construct must be
stripped, not copied: `<env>` with `{{.WorkingDir}}`, `{{.IsGitRepo}}`,
`{{.Platform}}`, `{{.Date}}`, `{{.GitStatus}}`, the `{{if gt (len .Config.LSP) 0}}`
`<lsp>` block, `{{.AvailSkillXML}}` with `<skills_usage>`, and the
`{{if .ContextFiles}}` / `{{if .GlobalContextFiles}}` blocks. Upstream's own
template already renders all of those. Copying them verbatim would emit literal
`{{.WorkingDir}}` text into the prompt.

**1.2 Seed and register it.**

Add `internal/contextseed/` mirroring `internal/officecli/seed.go`:

- `New(dataDir string, log *slog.Logger) *Seeder`
- `ContextDir() string` → `filepath.Join(dataDir, "context")`
- `Seed(sourceDir string) error` → `copyDirIfChanged(sourceDir/context, ContextDir())`
- `ContextPathArg() string` → `ContextDir()`

Register in `office_seed.go`'s workspace-open path (or a sibling
`context_seed.go` in `package main`, since these are not bound methods):

```go
svc.api.SetConfigField(ctx, workspaceID, crushapi.ConfigScopeWorkspace,
    "options.global_context_paths", []string{contextSeeder.ContextPathArg()})
```

Use `global_context_paths`, not `context_paths`. Rationale: the persona is
user-global rather than project-specific, it renders under the template's
`<user_preferences>` section, and it leaves `context_paths` free for its
upstream default job of discovering per-project `AGENTS.md` / `CRUSH.md` /
`.github/copilot-instructions.md`. Overwriting `context_paths` would disable
project-instruction discovery.

**1.3 Fix the `skills_paths` clobber.**

Change `registerOfficeTools` to merge instead of replace. Read the current value
before writing, or build the full intended list in one place. This is a
prerequisite for Phase 2, which adds a second seeded directory.

**1.4 Revert the vendored patch.**

Dispose of all eleven modified paths (verified list in Context), not just the
templates:

- Restore from upstream: `prompts.go` (embed back to `coder.md.tpl`),
  `coder.md.tpl`, `task.md.tpl`, `config.go`, `server.go`,
  `tools/mcp/process_other.go`.
- Delete: `assistant.md.tpl` (prose relocated to `resources/context/TACK.md`
  per 1.1), `prompts_gotack_test.go` (superseded by 1.5), `server/gzip.go`,
  `server/gzip_test.go`, `tools/mcp/process_windows.go`.
- The non-persona changes (agent rename, gzip handler, `Protocols` removal,
  MCP process handling) were never in a release build — `release.yml` clones
  upstream fresh — so dropping them makes dev match release. If any of them
  proves necessary later, it enters through the Phase 7 fork with tracked
  commits, not through an ignored working tree.
- The agent rename (Coder→Assistant, Task→Sage) has no config seam — the
  `Agents` map is `json:"-"` — so it cannot be restored from the host side at
  all before a fork. Until then the release keeps upstream agent names; the
  persona in `TACK.md` carries the identity.
- Confirm `scripts/update-crush.ps1` now leaves a clean checkout: after
  `git checkout --detach $Commit`, `git status --porcelain` in
  `third_party/crush` must be empty.

**1.5 Move the parity test where it runs.**

Add a test in the gotack module — `internal/contextseed/seed_test.go` — that
reads the tracked `resources/context/TACK.md` and asserts the markers the old
test asserted (`## Core Principles`, `## Task Management`,
`### Full Filesystem and Folder Access`,
`### Desktop and Screen Capture on Windows`,
`### Automatic Media and Document Delivery`, `## Implementation Methodology`),
plus a **negative** assertion that the file contains no `{{` sequence. The
negative test is what protects 1.1 from regressing.

**1.6 The Sage sub-agent persona — resolved as a known gap.**

Verified 2026-08-31 against upstream at the pin:

- Upstream `task.md.tpl` does **not** render `{{if .ContextFiles}}` or
  `{{if .GlobalContextFiles}}`. The context blocks in the patched copy were
  added by gotack.
- The fallback route fails too. `prompt.promptData` reads only
  `cfg.Options.ContextPaths` and `cfg.Options.GlobalContextPaths`; the
  per-agent `Agent.ContextPaths` field is never read by the prompt builder,
  and the `Agents` map is `json:"-"`.

Record it therefore as a gap: the Sage persona text is dropped with the
patch. What survives without a fork is the substantive part — the task
agent's read-only tool restriction (`resolveReadOnlyTools`) is upstream
behaviour. Reinstating the persona requires the template/context-rendering
change that belongs to the Phase 7 fork. Do not paper over the gap by
re-patching the vendored tree.

**1.7 Ship it.**

In `release.yml`, copy `resources/context` into the artifact next to the
existing `resources/skills` copy, and into the portable ZIP in the
"Assemble portable ZIP" step. Mirror the existing `if (Test-Path …)` guards.
Without this step the phase's headline outcome is not met.

**1.8 Update contracts.**

Hard rule 7 requires a contract document in the same change. `docs/README.md`
records that `internal/crushapi` has no contract document; this phase adds the
first gotack-owned Crush config keys, so create
`docs/contracts/crush-rest-sse.md` and record every config key gotack writes:
`mcp_servers.*`, `env`, `options.skills_paths`,
`options.global_context_paths`, and the model/provider keys already written by
`settings_crush.go`.

Exit criteria: `resources/context/TACK.md` tracked; vendored Crush checkout
clean; parity test green in `go test ./...`; a built ZIP contains
`resources/context/TACK.md`; a fresh workspace shows the persona in effect.

### Phase 2 — Persistent self-editing memory

Hermes' single largest differentiator, and reachable with no Crush change.

**Mechanism.** Memory files live inside the Phase 1 seeded context directory.
`prompt.Build` re-reads every context file on every prompt construction, so a
file the agent rewrites during turn N is in the system prompt at turn N+1 with
no engine restart. Writes go through a dedicated MCP tool rather than the
generic `write` tool so that size caps and approval are enforceable.

**2.1 Layout.**

```text
<appconfig.Dir()>/context/
  TACK.md            seeded, read-only to the agent
  memory/
    MEMORY.md        agent-curated durable facts
    USER.md          user preferences and stable profile
```

Both memory files must sit under the registered context directory so injection
is automatic, and **must not** sit inside any workspace the agent can reach with
the `write`/`edit` tools — otherwise the caps in 2.3 are bypassable.
`appconfig.Dir()` is `%AppData%\gotack`, which satisfies this as long as the
default workspace is not set to a parent of it.

**2.2 The MCP server.**

Add `cmd/memory/` using `internal/mcp`, exposing one tool `memory`:

| Field | Type | Meaning |
| --- | --- | --- |
| `action` | `"view" \| "add" \| "replace" \| "remove"` | required |
| `target` | `"memory" \| "user"` | which file, default `memory` |
| `section` | string | `§`-delimited section heading |
| `content` | string | new text for `add` / `replace` |

Follow Hermes' shape: `§` as the section delimiter, `add` appends within a
section, `replace` swaps one section, `remove` drops one. Return the resulting
file size and remaining budget in the tool result so the model can self-manage.

Register exactly as Office does, in the workspace-open path:

```go
entry := map[string]any{"command": memoryCmd, "type": "stdio", "timeout": 30}
svc.api.SetConfigField(ctx, workspaceID, crushapi.ConfigScopeWorkspace,
    "mcp_servers.gotack-memory", entry)
```

Resolve `memoryCmd` with the same bundled-then-PATH strategy as
`resolveOfficeCommand`, and call `RemoveConfigField` when it cannot be found, so
the agent is never shown a half-configured tool (hard rule 8).

**2.3 Budgets and safety.**

- Cap `MEMORY.md` and `USER.md` (Hermes uses ~2200 and ~1375 characters). Reject
  writes that would exceed the cap with an actionable error naming the cap, so
  the model consolidates instead of failing silently.
- Write atomically: temp file plus rename, so a crash cannot leave a truncated
  file that then becomes the system prompt.
- Honour decision 0003: no interactive approval for the dedicated tool. If a
  future revision reintroduces approval, route it through the existing
  `internal/permission` relay rather than inventing a second prompt path.
- Record provenance on each entry (timestamp, session id) so a poisoned entry is
  traceable.

**2.4 Release wiring.** Add a `go build -trimpath -o …\resources\memory.exe
./cmd/memory` step to `release.yml` beside the existing `cmd/office` step, and
copy `memory.exe` in the ZIP assembly step. Same for `ci.yml` if it builds
`cmd/office`.

**2.5 Proof.** Unit tests for caps, section editing, atomic write, and
malformed input. Behaviour proof: state a fact in session A, start session B,
ask for it, observe it answered without repetition — this is outcome 2 in the
Outcome section and cannot be replaced by a unit test.

Exit criteria: memory round-trips across two sessions in a built binary; caps
enforced with a test proving an over-cap write is refused.

### Phase 3 — Cross-session recall (`session_search`)

**3.1 Hard constraint discovered in the source.** The running engine holds an
exclusive `flock` on `{dataDir}/crush.lock` for its lifetime
(`internal/db/datadirlock.go`), and schema is owned by goose migrations. So:

- Open `crush.db` strictly read-only, via a `file:…?mode=ro` DSN.
- Do **not** acquire the data-dir lock, and do **not** set
  `CRUSH_SKIP_DATADIR_LOCK`.
- Do **not** create an FTS5 table inside `crush.db`. It would be an unmanaged
  schema object in a migrated database and would collide with a future upstream
  migration.

**3.2 Design.** Add `cmd/recall/` exposing one tool `session_search`:

- Maintains its own index at `<appconfig.Dir()>/recall.db` with an FTS5 table.
- Syncs incrementally using an `updated_at` watermark against
  `messages.updated_at` and `sessions.updated_at` (Unix milliseconds).
- Extracts searchable text from the `messages.parts` JSON column and joins
  `sessions.title`; later migrations add `provider`, `is_summary_message`, and
  `todos`, so select columns explicitly rather than `SELECT *`.
- Returns session id, title, timestamp, role, and a snippet, ranked by FTS rank.
- Filters `is_summary_message` rows out of results, or labels them, so summaries
  do not crowd out real content.

**3.3 Data dir path.** The host already opens workspaces with a known data
directory (`OpenWithDataDir` in the workspace activation path). Pass that path to
the recall server as an argument or environment variable at registration time
rather than re-deriving it.

**3.4 Pin coupling — accept and guard.** This phase reads Crush's private
schema, which is a real coupling that the REST boundary otherwise avoids. It is
justified because no REST endpoint exposes historical message search. Guard it:
extend `scripts/update-crush.ps1`'s existing marker check with the table and
column names this phase depends on, so a pin bump that renames them fails loudly
rather than silently returning empty results. Record the coupling in
`docs/contracts/crush-rest-sse.md` as an explicit exception with its rationale.

Exit criteria: outcome 3 demonstrated against a database with at least two prior
sessions; a renamed column causes `update-crush.ps1` to fail.

### Phase 4 — Graduated approvals and a destructive-command blocklist

This is the largest *divergence* from Hermes today, not merely a missing
feature. Hermes ships an approval mode matrix, an unrecoverable-command
blocklist, and a write-safe root. Gotack attaches every workspace with
permission prompts skipped and defaults the assistant workspace to the drive
root. Combined with the Zalo bridge, anyone who obtains the six-digit pairing
code reaches an auto-approving agent scoped to the whole drive.

**4.1 Use the one hook event that exists.** `internal/hooks` supports only
`PreToolUse`, which is exactly the right event for this. Verified contract:

- stdin JSON: `{event, session_id, cwd, tool_name, tool_input}`.
- environment: `CRUSH_EVENT`, `CRUSH_TOOL_NAME`, `CRUSH_SESSION_ID`,
  `CRUSH_CWD`, `CRUSH_PROJECT_DIR`, and, when present in the tool input,
  `CRUSH_TOOL_INPUT_COMMAND` and `CRUSH_TOOL_INPUT_FILE_PATH`.
- stdout JSON: `{version, decision: "allow"|"deny", halt, reason, context,
  updated_input}`; Claude Code's `hookSpecificOutput` form is also accepted.
- exit code 2 blocks the current tool call; exit code 49 (`HaltExitCode`) halts
  the whole turn.
- Aggregation across hooks: `deny` beats `allow` beats none, `halt` is sticky,
  `reason` strings concatenate, `updated_input` patches shallow-merge.

**4.2 Implement `cmd/guard/`.** A small executable registered as a `PreToolUse`
hook that:

- denies unrecoverable shell commands by pattern on `CRUSH_TOOL_INPUT_COMMAND`
  (recursive force delete, disk format, mass permission changes, shutdown,
  history rewrite, credential exfiltration to network sinks);
- enforces a write-safe root against `CRUSH_TOOL_INPUT_FILE_PATH`, denying
  writes outside it — in particular denying any write into
  `<appconfig.Dir()>/context/`, which closes the Phase 2 cap-bypass hole;
- returns `reason` text naming the rule, because outcome 4 requires the refusal
  to be legible;
- uses `halt` only for the genuinely unrecoverable cases, `deny` otherwise.

**4.3 Register it.** Config shape verified 2026-08-31 from `schema.json` and
`docs/hooks/README.md`: the top-level `hooks` key is `map[event][]HookConfig`
with `HookConfig{name, matcher, command, timeout}`; `matcher` is a regex
tested against the tool name (empty matches all tools), `command` is the only
required field. Register with:

```go
svc.api.SetConfigField(ctx, workspaceID, crushapi.ConfigScopeWorkspace,
    "hooks.PreToolUse", []map[string]any{{
        "name":    "gotack-guard",
        "matcher": "",
        "command": guardAbsPath,
        "timeout": 10,
    }})
```

The command resolves relative to the session's working directory, so the
registered path must be absolute (same bundled-then-PATH resolution as
`resolveOfficeCommand`, and `RemoveConfigField` when the binary is absent).
Read the current value before writing and merge, so a user-defined hook on
the same event is not clobbered — the same class of bug as step 1.3.

**4.4 Reverse the blanket skip, per decision D2.** Replace "skip all prompts"
with a mode: `auto` for ordinary local reads and edits inside the safe root,
`ask` for everything else, `deny` for the blocklist. Keep an explicit opt-in
escape hatch, and make remote-originated (Zalo) sessions default to the
stricter mode regardless of the desktop setting — an unattended remote session
is precisely where auto-approval is least defensible.

**4.5 Proof.** Positive: an allowed command inside the safe root runs. Negative:
a blocklisted command is refused for the intended reason, and a write to the
memory directory is refused. `docs/patterns/encoding-invariants.md` requires
both directions.

Exit criteria: outcome 4 demonstrated; the negative test fails for the right
reason; D2's stated posture matches observed behaviour.

### Phase 5 — Scheduled autonomous runs

Hermes treats cron as first class. Gotack has an always-running host process and
an existing remote delivery path, so this is host work with no Crush change.

**5.1 Implement `internal/schedule/`.**

- Job record: id, name, cron expression or interval, prompt text, target
  workspace, model override, enabled flag, last run, last outcome.
- Persist to `<configDir>/schedule.json`, matching the existing
  `<configDir>/zalo.json` convention.
- On fire: create a session through `internal/crushapi` and send the prompt, the
  same route the UI uses. Do not add a second execution path.
- Consume `run_complete` from the existing SSE stream to record the outcome.
  Hard rule 5 forbids polling where SSE exists.
- Borrow two guards from Hermes: a failure-nudge threshold that disables a job
  after N consecutive failures, and a preflight check that skips a run when no
  provider or model is configured rather than burning a failed run.

**5.2 Deliver results.** Reuse the Zalo bridge's existing behaviour: a completed
answer that names a local file path is uploaded to the paired chat. A scheduled
job therefore needs no new delivery code, only a paired chat.

**5.3 Respect the binding rules.** Bound methods live in `package main`, so add
`bind_schedule.go` (hard rule 1), expose it through
`frontend/src/platform/desktop.ts` only (hard rule 3), and update
`docs/contracts/wails-bindings.md` in the same change (hard rule 7). Hard rule 8
applies: do not add a job field the host accepts and ignores.

**5.4 Interaction with Phase 4.** A scheduled run is unattended, so an `ask`
approval can never be answered. Scheduled sessions must run under an explicit
policy — either restricted to auto-approvable operations, or failing the run
with a legible reason. Silently blocking forever is the failure mode to avoid.

Exit criteria: outcome 5 demonstrated end to end; a job that fails N times is
disabled and reported.

### Phase 6 — Learning loop, driven by SSE rather than hooks

**6.1 Correction to the obvious design.** A learning loop wants a post-run
signal. `internal/hooks` has only `PreToolUse`, so hooks cannot carry this.
The only available post-run signal is the `run_complete` SSE payload, which
`internal/crushapi` already consumes and `third_party/README.md` already records
as part of the checked contract. Drive the loop from the host, not from Crush.

**6.2 Design.** On `run_complete`, the host may start a bounded reflection run:

- Gate it. Not every turn deserves reflection; trigger on session end, on an
  explicit `/learn`-style user command, or on a turn count threshold. An
  unconditional reflection per run doubles token cost.
- Give the reflection run a small model and a narrow toolset: `session_search`
  from Phase 3 to read what happened, and `memory` from Phase 2 to propose
  durable facts.
- Route proposed writes through the D3 approval decision. An unattended,
  unapproved write into trusted prompt context is the highest-risk operation in
  this plan.
- Recursion guard: a reflection run also emits `run_complete`. Tag reflection
  sessions and ignore their completion events, or the loop feeds itself.

**6.3 Skill generation, deliberately deferred within the phase.** Hermes' `/learn`
can emit a new skill. Gotack can write a `SKILL.md` into the seeded skills
directory, which Crush discovers on the next prompt build. Do this only after
memory proposals are trustworthy in practice; a wrong skill is more damaging
than a wrong memory entry because its description competes for tool selection on
every turn.

**6.4 Skills hygiene, cheap and worth doing here.** Adopt the agentskills.io
frontmatter shape for the twelve existing skills in `resources/skills/`, and add
discovery for a user skills directory and a project `.agents/skills` directory
by extending the merged `options.skills_paths` list from step 1.3. Crush already
supports multiple paths, dedup, and `options.disabled_skills`.

Exit criteria: a reflection run proposes a memory entry from a real session, the
proposal is approved, and the fact is recalled in a later session. No recursion
observed.

### Phase 7 — Deferred: the parts that need the agent loop

Do not start these until Phases 1–6 are landed and stable. Each requires
modifying Crush itself, which means adopting a real fork with tracked commits
and accepting ongoing rebase cost against a fast-moving upstream.

- **Prompt tiering and context compression.** Hermes splits the prompt into
  stable, context, and volatile tiers for cache efficiency, and compresses
  context via `context_engine.py` / `context_compressor.py`. Crush builds one
  template per prompt with no tier boundary.
- **Multi-backend terminals.** Hermes supports local, docker, ssh, singularity,
  modal, daytona, and vercel sandboxes. Crush's `internal/shell` is local only.
- **Auxiliary background review model.** Hermes runs a second model for
  background review. Gotack does not currently even expose a distinct small
  model: per the cleanup plan's item A3, `SettingsInfo.SmallModel` is accepted
  and discarded, and `settings_crush.go` pins `models.large` and `models.small`
  to the same selection. Fixing A3 is a prerequisite.

If Phase 7 is adopted, the fork mechanics are: create `Dyu-36/crush`, carry
gotack changes as tracked commits on a `gotack` branch, point `.crush-pin` and
`scripts/update-crush.ps1` at the fork, and change `update-crush.ps1` from
`git checkout --detach` to a rebase-onto-upstream flow so the patch series is
replayed and conflicts surface. Until then, keep `third_party/crush` a clean,
unmodified checkout.

## Risks And Recovery

**R1 — Every context file is billed on every turn.** `promptData` re-reads and
re-injects all context files on each prompt build. `TACK.md` plus memory files
sit in the system prompt for the whole session. Mitigation: keep `TACK.md` to
prose policy only, enforce the Phase 2 caps, and measure prompt token count
before and after Phase 1 and Phase 2 rather than assuming the cost is small.

**R2 — Memory is trusted instruction text.** Anything in `MEMORY.md` becomes
system-prompt content next turn. A prompt injection absorbed from a web page or
a Zalo message that reaches a memory write becomes persistent, cross-session
instruction. This is the most severe risk in the plan. Mitigation: decision D3,
approval on writes, provenance per entry, caps, and the Phase 4 rule denying
writes into the context directory through generic file tools. Recovery: memory
files are plain text in a known directory — delete or edit them and the next
turn is clean.

**R3 — SQLite read-only access — resolved.** Crush opens `crush.db` with
`journal_mode = WAL` (`internal/db/connect.go` pragmas), and upstream itself
reads project databases while engines run: `openDBReadOnly` in
`internal/db/connect_ncruces.go` uses `file:…?mode=ro&_txlock=immediate` for
cross-project stats. Use that exact DSN shape. Fallbacks if it fails in the
field: `immutable=1` on a snapshot copy, or a copy-then-index cycle. Do not
solve it by taking the data dir lock or setting `CRUSH_SKIP_DATADIR_LOCK`;
either would risk corrupting the engine's database.

**R4 — Pin bumps can silently break Phase 3.** Reading a private schema is
outside the REST contract. Mitigation: extend the `update-crush.ps1` marker
check to the depended-on tables and columns so drift fails loudly. Recovery:
`session_search` degrades to returning nothing; the tool should report a
schema-mismatch error rather than an empty result, so the failure is visible.

**R5 — Hook latency multiplies.** A `PreToolUse` hook runs before every tool
call. A slow guard process adds latency to every step of every turn. Mitigation:
keep `cmd/guard` a single static binary with no I/O beyond stdin and stdout,
and measure per-call overhead before enabling it by default.

**R6 — Reversing auto-approval is a user-visible regression.** Phase 4 makes the
product ask questions it currently never asks, and unattended Phase 5 jobs can
deadlock on a prompt that nobody answers. Mitigation: decision D2 first; ship
the blocklist (deny-only, no new prompts) before the approval-mode change, so
the security floor arrives without the UX cost; give scheduled sessions an
explicit unattended policy per 5.4.

**R7 — Learning-loop recursion.** Reflection runs emit `run_complete`.
Mitigation: tag reflection sessions and ignore their events; add a hard per-hour
reflection budget as a second stop.

**R8 — File conflicts with the cleanup plan.** Phases 1, 2, and 5 touch
`office_seed.go` and the workspace activation path, which cleanup items C2 and
B2 also rewrite. Mitigation: land cleanup C2/B2 first, or land Phase 1 first and
rebase; do not run them concurrently. Also keep every touched implementation
file under 1000 lines or
`scripts/check-repository-invariants.mjs` fails the release.

**R9 — Artifact growth.** Phases 2, 3, and 4 each add an executable to the ZIP
(`memory.exe`, `recall.exe`, `guard.exe`) on top of `crush.exe`, `office.exe`,
and `officecli.exe`. Expect a larger download and more antivirus false
positives on an unsigned build. No mitigation beyond awareness; consider whether
one `gotack-tools.exe` with subcommands is preferable to three binaries.

**R10 — Seeding is effectively Windows-only.** Cleanup item A7 records that
`resolveOfficeSourceDir` hardcodes `officecli.exe`, so the seeding branch is
unreachable on macOS and Linux. Any new seeder copied from that pattern inherits
the defect. Fix the platform-aware name in `internal/contextseed` rather than
copying the bug.

**General rollback.** Every capability in Phases 1–5 is attached by a Crush
config key written from the host. Each is removable with
`RemoveConfigField` on `mcp_servers.gotack-memory`, `mcp_servers.gotack-office`,
`options.global_context_paths`, `options.skills_paths`, or the hooks key, plus
deleting the seeded directory under `appconfig.Dir()`. No migration is required
to retreat, because nothing in Phases 1–5 modifies Crush's own state.

## Progress

Phase 0 — authority gate:

- [x] D1 recorded: `docs/decisions/0001` (rule 4 narrowed; amendment lands
      with the first Phase 2 code).
- [x] D2 recorded: `docs/decisions/0002` (target approval posture).
- [x] D3 recorded: `docs/decisions/0003` (who may write memory, no
      interactive approval, structural constraints).
- [x] Crush pin collapsed into one tracked owner (`.crush-pin`); invariant
      checker updated to the new owning format with a drift guard.
- [x] This plan listed in `docs/plans/README.md`.

Phase 1 — relocate the persona:

- [x] `resources/context/TACK.md` extracted, template directives stripped.
      Evidence: `resources/context/TACK.md` tracked on this branch; the
      no-`{{` guard lives in `internal/contextseed/seed_test.go`
      (`TestRepoTrackedTackContext`).
- [x] `internal/contextseed/` seeds it to `<appconfig.Dir()>/context/`.
      Evidence: `internal/contextseed/seed.go` wired in `app.go` startup via
      `ensureContextSeed`; table-driven tests cover fresh/idempotent/
      preserved/propagated seeding.
- [x] `options.global_context_paths` registered at workspace open.
      Evidence: `context_seed.go` `registerContextPaths`, called from
      `rebindWorkspaceRuntime` (`bind_workspace.go`) so both activation paths
      get it; key name verified against vendored `Options.GlobalContextPaths`.
- [x] `options.skills_paths` merges instead of overwriting.
      `registerOfficeTools` now reads the workspace config via
      `crushapi.GetWorkspaceConfig` and appends the bundled path only when
      absent (`mergeSkillsPaths` in `office_seed.go`); regression table tests
      in `office_seed_test.go` green 2026-08-31.
- [x] Vendored patch disposed per 1.4 (all eleven paths); `third_party/crush`
      checkout clean, verified 2026-08-31 by an empty `git status --porcelain`.
      Step 1.1 must now read the persona from the off-repo backup named in the
      third pass below.
- [x] Parity test moved into the gotack module, including the no-`{{` assertion.
      Evidence: `internal/contextseed/seed_test.go` runs in CI via
      `go test ./...` and asserts the old markers plus the negative `{{` check.
- [x] Sage persona recorded as the known gap per 1.6 (no re-patch).
      Evidence: this plan's 1.6 plus Decisions (second pass) already record
      the gap; `docs/contracts/crush-rest-sse.md` carries it forward as the
      durable record. No re-patch was made.
- [x] `release.yml` copies `resources/context` into the artifact and the ZIP.
      Evidence: `release.yml` gained a `Prepare bundled runtime payloads` step
      (runs `scripts/prepare-resources.ps1`) and the ZIP assembly now copies
      both `resources/bin` payloads and `resources/context`.
- [x] `docs/contracts/crush-rest-sse.md` created listing every written key.
      Evidence: the document inventories every config key, REST endpoint and
      SSE event, each with its `RemoveConfigField` undo path.

Phase 2 — memory:

- [ ] `AGENTS.md` hard rule 4 amended per decision 0001, in the same change
      as the first memory code.
- [ ] `cmd/memory/` with the `memory` tool over `internal/mcp`.
- [ ] Caps, atomic writes, provenance per decision 0003 (no interactive
      approval).
- [ ] Registered as `mcp_servers.gotack-memory`; absent binary removes the key.
- [ ] `memory.exe` built and shipped in `release.yml`.
- [ ] Cross-session recall of a stated fact demonstrated.

Phase 3 — recall:

- [x] Journal mode verified (WAL) and read-only strategy chosen:
      `file:…?mode=ro&_txlock=immediate`, per upstream `openDBReadOnly` (R3).
- [x] `cmd/recall/` with FTS5 index at `<appconfig.Dir()>/recall.db`.
      Implemented as `<appconfig.Dir()>/recall/recall.db` per the WP5 work
      order; proven by `internal/recall` tests over fixture databases built
      with the real SQLite driver (WP5 gate `artifacts/gates/wp5-test.txt`).
- [x] Incremental sync by `updated_at` watermark.
      Watermarks stored in `recall_meta`, inclusive bound keeps re-reads
      idempotent; proven by `TestIncrementalSyncByWatermark`.
- [x] `update-crush.ps1` marker check extended to the depended-on schema.
      `scripts/update-crush.ps1` now fails a pin bump missing the sessions/
      messages tables or the title/role/parts/session_id/updated_at columns.
- [x] Schema mismatch surfaces as an error, not an empty result.
      Missing tables/required columns return `ErrSchemaMismatch` through the
      MCP tool as an isError result; proven by `TestSchemaMismatchSurfacesAsError`
      and `TestToolSurfacesSchemaMismatch`; missing optional columns degrade
      with a logged warning (`TestMissingOptionalColumnsDegradeGracefully`).

Phase 4 — approvals:

- [x] Hooks config key and matcher syntax confirmed from `docs/hooks` and
      `schema.json` (`hooks.PreToolUse`, regex matcher on tool name).
- [x] `cmd/guard/` denies the unrecoverable blocklist with a named reason.
      Evidence: `internal/guard/blocklist.go` implements six named rules;
      `internal/guard/blocklist_test.go` pins the deny matrix and asserts the
      reason names the rule (`TestDenyReasonNamesTheRule`).
- [x] Write-safe root enforced, including denying writes to the context dir.
      Evidence: `internal/guard/policy.go` denies `memory-context-write` and
      `write-outside-safe-root`; `TestEvaluateTierMatrix` pins both denials,
      including the context dir nested inside a drive-root workspace.
- [x] Blocklist shipped before the approval-mode change (R6).
      Evidence: the deny-only hook landed in its own commit (6c5d734) with no
      permissions-skip change; every non-blocklisted call still passed through
      untouched at that point (stage-1 `TestPassThroughEmitsNothing`).
- [x] Approval modes implemented per D2; remote sessions default stricter.
      Evidence: graduated allow/ask/deny tiers in `internal/guard` (auto for
      reads and in-root writes, ask via the existing permission relay, deny
      floor); `activateWorkspace` now honours `auto_approve` instead of
      forcing skip; Zalo sessions are marked in the unattended roster and
      ask-tier calls are denied there (`ruleUnattendedApproval`), never
      hanging on a prompt.
- [x] Positive and negative proofs both recorded.
      Evidence: positive — in-root writes and reads are pre-approved
      (`TestEvaluateTierMatrix` allow cases, `TestAllowRoundTrip`);
      negative — blocklisted commands, out-of-root writes, context-dir writes
      and unattended ask-tier calls are refused naming the rule
      (`TestMatchBlocklistDenies`, `TestDenyRoundTrip`, the deny cases).

Phase 5 — scheduling:

- [ ] `internal/schedule/` with `<configDir>/schedule.json` persistence.
- [ ] Runs created through `internal/crushapi`; outcomes read from SSE.
- [ ] Failure-nudge threshold and preflight guard.
- [ ] `bind_schedule.go`, `desktop.ts`, and `wails-bindings.md` in one change.
- [ ] Unattended approval policy defined (5.4).

Phase 6 — learning loop:

- [ ] Reflection triggered from `run_complete`, gated, with a recursion guard.
- [ ] Proposals routed through the D3 approval path.
- [ ] Skills frontmatter normalised; user and project skills directories added.
- [ ] Skill generation deliberately deferred until memory proposals are trusted.

Carry-over hygiene, inherited 2026-08-31 from the now-completed cleanup plan
(`docs/plans/completed/cleanup-dead-code-and-doc-drift.md`). Re-audited the
same day; thirteen of its twenty items had landed, and only these are open:

- [ ] A8: mark `cfg.Zalo.Token` and `cfg.Zalo.AllowedChats` deprecated in
      `internal/appconfig` with a removal target. `app.go:133` and
      `internal/zalo/manager.go:152` still call `ImportLegacy` with no stated
      expiry.
- [x] B2: collapse `activateWorkspace` (`bind_workspace.go:77`) and
      `activateAssistantWorkspace` (`:117`) onto `replaceWorkspaceStream`
      (`bind_engine.go:316`). Do this together with C2, never separately (R8).
      Done together with C2: both now route through one shared
      `activateCurrent` path in `bind_workspace.go`, and
      `replaceWorkspaceStream` rebinds via `rebindWorkspaceRuntime` with the
      documented transportLost-versus-cancel policy difference preserved.
- [x] C2: extract the supervisor into `internal/enginelink`. The package still
      does not exist, which is what keeps A8, B2 and Phase 5 entangled.
      Done: connection state machine (status, connect scope, stream attach,
      loss reporting) lives in `internal/enginelink` with race-safe lifecycle
      tests; package main keeps only thin bound-method wrappers.
- [ ] C4: decide typed errors versus one string table before adding more
      user-facing Vietnamese strings; seven non-test files carry them today.
- [ ] C5: write the frontend placement rule into `docs/README.md`.
      `features/conversations` is imported by five components, so the
      directory is used, not dead; only the rule is missing.
- [x] C6: `release.yml` packages neither `resources/bin/` nor
      `resources/context/`. Phase 1.5 must fix both in one release-job change.
      Done 2026-08-31 on `feat/phase1-persona-context`: the release job runs
      `scripts/prepare-resources.ps1` and copies both directories into the
      portable ZIP; copy logic proven by a temp-dir dry run of the same
      commands.
- [ ] F: `docs/contracts/crush-rest-sse.md` and `contracts/zalo-bot.md` are
      still absent while hard rule 7 requires a contract per external
      boundary. Phase 1.6 already owes the first file. Partial: `crush-rest-sse.md`
      landed 2026-08-31 on `feat/phase1-persona-context`; `zalo-bot.md`
      remains open.

## Decisions

- 2026-08-31: Relocate the persona to `options.global_context_paths` rather than
  deleting it or keeping the vendored patch. Reason: deleting it regresses to
  upstream's coder prompt, which is the opposite of the goal; the patch is
  invisible to the release pipeline and is destroyed by the next pin bump.
- 2026-08-31: Use `global_context_paths`, not `context_paths`. Reason: the
  persona is user-global, it renders under `<user_preferences>`, and overwriting
  `context_paths` would disable upstream discovery of per-project `AGENTS.md`
  and `CRUSH.md`.
- 2026-08-31: Do not fork Crush for Phases 1–6. Reason: verification found a
  supported seam for every capability in those phases. A fork is only required
  for Phase 7, and carries permanent rebase cost against a fast-moving upstream.
- 2026-08-31: Drive the learning loop from the `run_complete` SSE event, not from
  hooks. Reason: `internal/hooks` exposes only `PreToolUse`; there is no post-run
  hook. This corrects an earlier assumption.
- 2026-08-31: Keep the recall FTS5 index in a separate gotack-owned database.
  Reason: `crush.db` schema is goose-managed and the engine holds an exclusive
  data-dir lock; an unmanaged FTS table there would collide with a future
  upstream migration.
- 2026-08-31: Ship the destructive-command blocklist before the approval-mode
  reversal. Reason: it raises the security floor without introducing prompts
  that unattended sessions cannot answer.
- 2026-08-31 (second pass): D1, D2, and D3 promoted to
  `docs/decisions/0001`–`0003` under delegated authority.
- 2026-08-31 (second pass): The Sage persona is a known gap, not a Phase 1
  deliverable. Reason: upstream `task.md.tpl` renders no context files, the
  per-agent `ContextPaths` field is never read by the prompt builder, and the
  `Agents` map is `json:"-"`. The read-only tool restriction survives as
  upstream behaviour; the persona returns only with the Phase 7 fork.
- 2026-08-31 (second pass): Drop the non-persona vendored patches (agent
  rename, gzip handler, `Protocols` removal, MCP process handling) in 1.4.
  Reason: they were never in a release build, so keeping them only widens the
  dev/release gap; if any is needed later it enters through the tracked fork.
- 2026-08-31 (second pass): Register the guard hook under `hooks.PreToolUse`
  with an absolute `command` path, merging with any existing user hooks.
  Reason: verified config shape; `command` resolves relative to the session
  working directory.
- 2026-08-31 (second pass): Phase 3 opens `crush.db` with
  `file:…?mode=ro&_txlock=immediate`. Reason: the engine uses WAL, and
  upstream's own `openDBReadOnly` proves this DSN shape works while engines
  run.
- 2026-08-31 (WP5): Phase 3 adds exactly the SQLite driver Crush itself
  uses, `modernc.org/sqlite v1.56.0` — the exact requirement line in
  `third_party/crush/go.mod`, pure Go, driver name `sqlite`. Reason:
  reading `crush.db` is impossible without a driver, and the work order's
  dependency exception authorizes this one module and nothing else;
  matching Crush's pin line keeps both SQLite stacks on one version line.
- 2026-08-31 (WP5): Recall probes `{dataDir}/crush.lock` with bounded
  retry/backoff but never acquires it, and proceeds read-only when the
  engine holds it. Reason: 3.1 forbids holding the data-dir lock, the
  engine holds it for its whole lifetime, and recall must stay usable while
  the engine runs; a read-only open against WAL is safe.

## Validation

- Focused proof: unit tests for `internal/contextseed` path resolution and the
  no-template-directive assertion; `cmd/memory` caps, section editing, atomic
  write, malformed input; `cmd/recall` watermark sync and `parts` JSON
  extraction; `cmd/guard` blocklist matching, with a negative case per
  `docs/patterns/encoding-invariants.md`.
- Integration or end-to-end proof, per phase, because none of these are provable
  by unit test: (1) a built ZIP contains `resources/context/TACK.md` and a fresh
  install shows the persona in effect; (2) a fact stated in session A is recalled
  in session B; (3) a question about a prior week is answered from history;
  (4) a blocklisted command is refused with a legible reason and an allowed one
  succeeds; (5) a scheduled job runs unattended and delivers to a paired chat;
  (6) a reflection run proposes a memory entry, it is approved, and it is later
  recalled, with no recursion.
- Repository-required checks, every phase: `gofmt -l` empty,
  `go build ./...`, `go vet ./...`, `go test ./...`,
  `node scripts/check-repository-invariants.mjs`,
  `pnpm --dir frontend check`, `pnpm --dir frontend test`,
  `pnpm --dir frontend build`, and
  `wails build -platform windows/amd64 -clean -webview2 download` for any phase
  that changes packaging. Also `staticcheck ./...`, `deadcode -test ./...`, and
  `actionlint` where available, matching the gates the cleanup plan used.
- Environment note: `scripts/check-repository-invariants.mjs` shells out to
  `git ls-files`, so the checker must be run from a shell where `git`
  resolves. Verified 2026-08-31 that `git` is on the `PATH` on this machine
  (git 2.51.0.windows.1) and the checker passes.
- Not proof: this plan, the phase checklists, and a completion message. Per
  `docs/WORKFLOW.md`, plans and checklists do not establish product behaviour.

## Result

Not started as code. The second pass (2026-08-31) closed every item that was
blocking the plan:

- D1, D2, D3 are recorded in `docs/decisions/0001`–`0003` under delegated
  authority.
- Step 1.6 resolved: upstream `task.md.tpl` renders no context files and the
  per-agent `ContextPaths` fallback is dead code; Sage persona recorded as a
  known gap until the Phase 7 fork.
- Step 4.3 resolved: `hooks.PreToolUse` shape verified from `schema.json` and
  `docs/hooks`.
- Risk R3 resolved: WAL journal mode with an upstream read-only precedent.
- The vendored patch was fully enumerated by git (eleven uncommitted paths on
  the detached pin, no local commits), and 1.4 now disposes of every one.
- The `.crush-pin` consolidation (cleanup item B3) is landed and the
  invariant checker passes with the new owning format.

Remaining before code:

- Apply the `AGENTS.md` amendment from decision 0001 together with the first
  Phase 2 change.
- Step 1.1 must now extract `resources/context/TACK.md` from the off-repo
  backup named in the third pass below. The vendored copy no longer exists.

## Third pass, 2026-08-31

Step 1.4 was executed ahead of the rest of Phase 1, on request, so the vendored
patch could stop rotting against the next pin bump.

- Evidence before disposal: `git -C third_party/crush status --porcelain`
  listed exactly the eleven paths this plan enumerated, six tracked and five
  untracked, on detached HEAD `6d14dd9`, which matches `.crush-pin`.
- Backup taken first, deliberately outside the module tree, at
  `C:\MCP-Machine\backups\gotack-vendored-patch-2026-08-31\`: ten verbatim
  file copies plus `vendored-tracked.patch` (13,205 bytes, the `git diff` of the
  six tracked paths). `assistant.md.tpl` (7,904 bytes) is the only surviving
  source of the Tack persona text and is the input for step 1.1.
- Disposal: `git checkout HEAD --` on the six tracked paths, then a delete of
  the five untracked files. `git status --porcelain` is now empty,
  `internal/agent/prompts.go` embeds only the `coder`, `task` and `initialize`
  templates again, and `process_other.go` is restored.
- Accepted consequence: development builds run upstream's coder prompt with no
  persona until 1.1 through 1.3 land. Recovery is the patch file above.
- Do not restore that backup under `/artifacts/`. A `.go` file anywhere below
  the module root is compiled by `go build ./...` even when the directory is
  git-ignored, so an in-tree backup breaks the build and the invariant check.

Environment correction, verified 2026-08-31: `git` works on this machine
(git 2.51.0.windows.1). The earlier session's empty `git` output was an
environment failure of that session, not a repository property, and the
"clean tree" reading that followed it was retracted here with evidence.
