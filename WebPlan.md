# WebPlan — Gotack input pipeline implementation handoff

Date created: 2026-09-05
Scope: Windows-only milestone
Purpose: durable handoff for a new ChatGPT/Web coding session that will act as the primary implementer for `ImplementPlan.md`.

> This file is a handoff/working protocol, not proof that any implementation phase is complete.
> `ImplementPlan.md`, `AGENTS.md`, `docs/WORKFLOW.md`, repository contracts/decisions, code, tests, and runtime evidence remain the system of record.
> If this file conflicts with an accepted repository contract or a newer explicit owner decision, stop and resolve the conflict before editing.

---

## 1. Mission

Implement the complete approved input-pipeline upgrade described by `ImplementPlan.md`, phase by phase, with the Web session acting as the **primary coder/integrator** and the owner's Windows machine acting as the **runtime/test environment** for gates that cannot be executed in the Web environment.

The final product objective is not simply "make Gotack faster". The target is:

1. deterministic prompt/request assembly;
2. no loss/clobber of configuration, todo state, session history, or provider reasoning state;
3. telemetry sufficient to attribute latency and request-shape changes without leaking secrets/content;
4. reproducible Crush/Fantasy provenance and patches from clean pins;
5. black-box executable/REST/SSE/fake-provider E2E evidence;
6. safe TACK context ownership/migration with recovery and rollback;
7. full OpenAI Responses reasoning continuity across tool loops/restart/compaction boundaries;
8. cache experimentation only after observability/correctness gates justify it;
9. a Windows release gate backed by executable evidence, not checkboxes.

Do **not** implement hybrid/local compaction in this milestone. Only the bounded PR5 history-anchor/selection fix defined by `ImplementPlan.md` is allowed.

---

## 2. Authority and files that MUST be read at the start of a new session

Before editing code, read in this order:

1. `AGENTS.md`
2. `docs/WORKFLOW.md`
3. `ImplementPlan.md`
4. `WebPlan.md` (this file)
5. only the contracts/decisions/code relevant to the current phase

Important repository rules:

- Gotack must never import `third_party/crush/internal/...`; desktop integration stays REST + SSE through `internal/crushapi`.
- UI-to-host calls go through `frontend/src/platform/desktop.ts`.
- Bound methods remain in package `main`.
- External/wire/UI contract changes require the owning contract doc to be updated in the same change.
- No field/method may be accepted and silently ignored.
- No polling when SSE already owns the event flow.
- Keep implementation files under 1000 lines and split by responsibility.
- A plan/checklist is not proof; completion requires executable/observable evidence.

For long-running implementation, maintain one durable active execution plan under `docs/plans/active/` if required by `docs/WORKFLOW.md`. Do not create a parallel task database. `WebPlan.md` is the session handoff and constraints ledger; it is not a substitute for repository-native phase progress if an active plan is needed.

---

## 3. Verified owner-machine baseline before implementation

These facts were supplied from the owner's Windows machine and are the baseline for future comparisons.

### 3.1 Gotack repository

Before this `WebPlan.md` commit, `main` was:

```text
e2ec5ad9158403e8fb1f88cd80571687d3005291
```

That commit adds/reviews `ImplementPlan.md`; its parent functional baseline is:

```text
d07c8929b8737fedcc7e2584e78f6e9c7d662bc3
```

Owner observed before implementation:

```text
git status --short --branch
## main...origin/main
```

The repository root was clean.

After removing the temporary audit directory that had caused Go package discovery conflicts, the owner ran:

```powershell
go test ./...
```

Result: **PASS** across the Gotack module.

This is the functional baseline. If this command starts failing after implementation, treat it as a regression until proven otherwise.

### 3.2 Windows toolchain observed

```text
OS/arch: Windows amd64
Go: go1.27.0
CGO_ENABLED: 0
Node: v24.7.0
pnpm: 11.20.0
PowerShell reported: 7.6.4
Wails: v2.15.0
```

`go test -race ./...` currently does not execute because Go reports:

```text
go: -race requires cgo; enable cgo by setting CGO_ENABLED=1
```

Therefore the Windows race gate is currently:

```text
BLOCKED_ENVIRONMENT: CGO disabled / compatible C compiler not proven
```

This is **not** a source-code test failure and must never be reported as PASS.

Do not instruct the owner to install a compiler, enable services, or modify global toolchains unless the owner explicitly approves that operation.

### 3.3 Running product and providers

Owner confirmed the Gotack app currently runs.

Providers are numerous and include at least:

- OpenAI through OAuth;
- OpenCode Go;
- MiniMax;
- Mistral;
- OpenRouter;
- additional providers.

Some providers are currently believed to be broken. Do not turn PR0 into a general provider-repair project. PR0 must add observability/harness evidence so later failures can be attributed to auth, provider adapter, options merge, request shape, transport, or provider behavior.

For reasoning-continuity live acceptance, OpenAI OAuth / OpenAI Responses is the mandatory reference path because PR5 defines OpenAI Responses continuity as P0.

Never request or log API keys, OAuth tokens, authorization headers, ciphertext reasoning, raw tool outputs, or raw session UUIDs.

### 3.4 Crush provenance on owner machine

Tracked pin:

```text
.tack-pin = 6d14dd93a9e526505f7de54ae5999431bc32a793
```

Owner's ignored `third_party/crush` checkout is at exactly that commit but is **dirty** with compatibility/hardening/local changes.

Critical rule:

```text
DO NOT reset, clean, checkout over, or use the owner's dirty third_party/crush as the place to develop/reconstruct patches.
```

Implementation must use an isolated checkout created from the pin and must produce tracked/replayable patch artifacts in the parent Gotack repository.

### 3.5 Scratch checkouts

The audit had scratch trees:

```text
tmp/crush-input-pipeline
  baseline commit: 40d74a1add85bb4a3d09db3d6959721ce600fbf0

tmp/fantasy-input-pipeline
  baseline/tag: f06034c7824ffddc4394d4cefa5ed5132a186b1b (v0.41.3)
```

They previously contained large dirty prototypes for prompt/runtrace/reasoning work. The owner intentionally cleaned the tracked dirty edits to reduce noise. Current scratch state must **not** be treated as source-of-truth, implementation evidence, or upstream acceptance.

`tmp/crush-input-pipeline` may still show an untracked nested `third_party/` directory. It is not a release artifact and should not be used as authority.

`third_party/fantasy` does not currently exist in the parent repository.

Do not rely on local scratch diff backups unless the owner deliberately supplies them later.

---

## 4. Primary-coder / owner-test protocol

The Web session is the primary coder and code reviewer. The owner/employee is the Windows runtime operator.

For every checkpoint:

1. Web coder reads authority and current repository state.
2. Web coder makes the smallest coherent change on a dedicated implementation branch.
3. Web coder performs any static/read-only checks available in the current environment, but does not invent Windows runtime evidence.
4. Web coder commits and reports:
   - branch;
   - commit SHA;
   - files changed;
   - exact Windows commands to run;
   - expected success/failure conditions;
   - which outputs/artifacts to return.
5. Owner pulls that branch on Windows and runs exactly the requested checks.
6. Owner sends raw relevant output/evidence back.
7. Web coder classifies every gate as exactly one of:
   - `PASS`
   - `FAIL`
   - `BLOCKED`
   - `SKIP` (only when explicitly allowed)
8. Fix failures before proceeding to the next phase/checkpoint.

Never call `BLOCKED`, unexpected `SKIP`, zero-capture E2E, missing binary, or scaffold-only behavior `PASS`.

Do not weaken assertions, delete tests, change gates, or silently change product behavior merely to make a test green.

---

## 5. Branch and integration policy

Do not implement directly on `main`.

Recommended branches:

```text
impl/phase-0a-harness-provenance
impl/phase-0b-observability
impl/phase-1-correctness
impl/phase-2-prompt-architecture
impl/phase-3-tack-migration
impl/phase-4-reasoning-continuity
impl/phase-5-cache-experiment
```

If a phase is too large, split it into explicitly named checkpoints but preserve the phase gate.

Do not implement all PR0–PR5 in one unreviewable branch.

At the start of every branch, re-check current `main`, the current `.tack-pin`, relevant patch inventory, and docs. This file records the 2026-09-05 baseline, not permission to ignore newer repository truth.

---

## 6. Global non-negotiable implementation constraints

### 6.1 Provenance and ignored engine trees

All final Crush changes must be replayable from a clean pin.

Required order:

```text
clean pinned Crush
  -> tracked compatibility patches (in explicit order)
  -> Gotack hardening step
  -> input-pipeline phase patches
  -> build/test candidate
```

Do not simply depend on alphabetical `zz-` ordering if the replay script applies all patches before hardening. The phase ordering itself must be encoded and proven.

Record at minimum:

- Gotack commit;
- Crush pin;
- ordered patch filenames;
- SHA256 of every patch;
- hardening-script SHA256;
- Fantasy version/commit if used;
- candidate binary SHA256;
- build command;
- provenance of any explicit binary used by E2E.

Do not infer binary provenance from filename or PATH.

If a patch must include new files/tests, generate it in a way that includes untracked additions (for example via a reviewed staged diff), not a plain unstaged diff that drops new files.

Use UTF-8/LF. Do not create release patches through Windows PowerShell 5.1 redirection that may emit UTF-16.

### 6.2 Security / telemetry

Never emit or hash secret/opaque content that the Plan excludes.

Forbidden from telemetry/log evidence includes:

- prompt text;
- OAuth/API tokens;
- authorization headers;
- raw tool output;
- raw session UUID;
- encrypted reasoning ciphertext/signatures;
- secret provider payloads.

Sanitize before every sink, including error/warning paths.

HMAC failures must produce `unavailable`, not a plaintext/SHA fallback.

### 6.3 Determinism

Determinism means canonical rendered bytes, not merely sorted maps.

On Windows contract v1:

- path identity is case-insensitive;
- relative/absolute aliases and separator aliases must dedupe to the same canonical rendered identity;
- overlapping roots dedupe within the same lane;
- project/global precedence remains distinct;
- do not change symlink/junction semantics with unconditional `EvalSymlinks`;
- per-directory case-sensitive Windows mode is out of scope for this milestone.

Stable prompt identity may not depend on timestamped/staging physical snapshot paths when content is unchanged.

### 6.4 Provider semantics

Do not assume model-name substrings imply Responses reasoning capability.

Invalid provider options must fail before network dispatch and must not mutate user config.

`reasoning_summary: null` is explicit omission and is different from absent/default.

### 6.5 Reasoning replay

For OpenAI Responses continuity:

- explicit local history replay;
- `store=false`;
- do not combine replay with `previous_response_id`;
- preserve ordered multiple reasoning items;
- encrypted-only reasoning items must survive;
- duplicate item IDs must fail pre-network;
- never transform reasoning summary/thinking into assistant text;
- provider/model/account compatibility must be checked without exposing credentials.

### 6.6 Migration safety

TACK migration is a transaction/state machine, not a series of best-effort renames.

Never test destructive migration against the owner's real profile/data.

Use isolated temp profiles/portable artifacts only.

Rollback must be durable across restart; preview/accept must detect concurrent edits; legacy active data cannot be destroyed before a valid committed replacement exists.

---

## 7. Phase 0A — harness, provenance, replay/build, fail-closed E2E

Status at creation of this file: **NOT STARTED**.

Goal: make the validation machinery trustworthy before performance or correctness changes are claimed.

### 7.1 Required implementation

Build/fix a black-box E2E harness that:

- resolves repository root correctly;
- uses a unique temp directory (`RUNNER_TEMP` when available, OS temp fallback locally);
- checks every native command exit code;
- fails on timeout/missing artifact;
- never applies patches into the owner's dirty ignored checkout;
- clones/checks out the exact Crush pin;
- applies compatibility patches, then hardening, then input-pipeline phase patches;
- builds Crush from root `.` using a reproducible command such as `go build -mod=readonly -trimpath`;
- stores/validates provenance and binary hash;
- supports `SkipBuild` only with an explicit absolute binary and matching provenance;
- does not silently fall back to PATH;
- launches isolated engine state/data/home/workspace/provider environment;
- uses the real CLI contract for `server --host ... --data-dir ...`;
- tests named-pipe transport on Windows where required;
- uses a real fake Responses provider speaking valid SSE;
- uses real fake MCP JSON-RPC stdio, protocol-only stdout;
- clears live credentials in fake-provider child processes;
- creates workspace/session through actual REST interfaces instead of guessing IDs;
- subscribes SSE before prompt dispatch;
- waits for the matching terminal `run_complete`;
- counts provider requests/tool calls/retries and requires counts greater than zero when expected;
- performs bounded cleanup only for processes created by that test;
- reuses the isolated DB for restart tests;
- fails closed on unsupported/missing required conditions.

Fix/remove any no-op `$SkipInputPipeline` behavior. A declared but ignored flag is forbidden.

### 7.2 Mandatory negative controls

Harness must itself be proven to fail when appropriate, including at least:

- engine binary missing;
- endpoint never ready;
- zero provider captures;
- malformed provider schema/SSE lifecycle;
- dropped terminal event;
- unexpected required-test skip;
- unsupported required platform path.

The old behavior where E2E passed with a nonexistent `TACK_ENGINE_BINARY` and zero provider attempts must be impossible.

### 7.3 Owner Windows gate after Phase 0A

Coder must supply exact commands based on the implemented scripts/tests. Expected minimum families:

```powershell
go test ./...
node scripts/check-repository-invariants.mjs
# focused harness tests
# clean-pin replay/build command
# required fake-E2E lane after it exists
```

Race remains separately BLOCKED until a usable Windows CGO/C toolchain exists.

Do not proceed to performance claims while harness can false-pass.

---

## 8. Phase 0B — PR0 observability and baseline

Status: **NOT STARTED**.

Goal: produce real wire/runtime telemetry that explains where latency and request changes occur.

### 8.1 RunTrace model

Implement a per-root-run trace with separate levels for:

- root run;
- model call/step;
- HTTP attempt;
- retry;
- title/summarize/tool-loop purposes.

Use a monotonic origin (`time.Now` anchor + `time.Since` style durations). Store offsets/durations; absent event = absent, never numeric zero.

Required timing domains include:

- ready wait;
- MCP wait;
- exclusive/union local preparation as appropriate;
- model refresh;
- skill scan;
- tool build;
- history load;
- prompt prepare;
- request encode;
- request write to first response byte;
- first complete SSE frame;
- first reasoning;
- first tool;
- first non-empty text;
- stream;
- summarize;
- total root run;
- retry delays/attempts.

Do not double-count overlapping parent/child spans.

`GotFirstResponseByte` is not equivalent to first complete SSE event.

Engine text-available TTFT must not be described as UI-visible TTFT unless the UI itself records visibility.

### 8.2 Request shape and usage

Add optional telemetry to `run_complete` while preserving backward compatibility.

Track, when available:

- provider/model;
- endpoint/API-mode fingerprint data that is safe;
- effective reasoning effort;
- service tier;
- provider request ID only if safe/contracted;
- attempt/retry counts and delay;
- compaction flag;
- usage presence;
- cache status `hit|miss|unreported`;
- cached/uncached token counts as optional/pointer values;
- stable/dynamic/request shape HMACs and byte counts;
- explicit change reasons.

Request-shape HMAC must be computed from a sanitized, canonical post-adapter projection after all relevant option/tool transforms. It is not a full-wire plaintext hash.

### 8.3 Gotack side

Expected ownership includes:

- `internal/crushapi` contract extension;
- event forwarding compatibility;
- `internal/runmetrics/` validation and redacted JSONL output;
- contract update (`docs/contracts/crush-rest-sse.md`).

No dashboard/UI graph is required in PR0.

### 8.4 Fake-provider acceptance

Required tests include:

- timing ordering;
- one-shot first-event recording;
- 429 then retry success;
- no token double count;
- same `run_id` correlation across provider capture, SSE terminal telemetry, and JSONL;
- old events without telemetry still decode;
- new telemetry does not break UI/Zalo/scheduler consumers;
- synthetic canaries never leak into prohibited logs/telemetry/error sinks.

Only after this passes may the owner run live synthetic baseline workloads.

### 8.5 Live baseline

Use synthetic prompts only, explicit approved account/endpoint/model, explicit request/cost cap, and no secrets in chat.

Collect the five workload classes defined by `ImplementPlan.md` before making performance claims.

---

## 9. Phase 1 — PR1 bounded correctness

Status: **NOT STARTED**.

This phase contains three independent correctness areas that must all receive regression proof.

### 9.1 Deterministic prompt/path/MCP ordering

Implement ordered context groups instead of map iteration where needed.

Canonicalize/dedupe Windows paths according to the v1 contract and render canonical identity bytes independent of input alias casing/order.

Sort MCP names before rendering connected non-empty instructions.

Required proof includes randomized permutations and real Windows/NTFS cases, plus executable fake-MCP restart repetitions.

### 9.2 Provider options must not clobber user configuration

Implement normalized union for Responses `include`, preserving configured values while adding required `reasoning.encrypted_content` exactly once and in canonical order.

`reasoning_summary="auto"` is only a default when the key is absent. Explicit valid user choice wins. Explicit null means omit.

All merges happen on deep/local copies.

Invalid options fail before network and generate sanitized reason codes without echoing rejected secret/value content.

Required proof: unit representation matrix, precedence integration fixture, fake-provider wire capture, zero network attempts on invalid config.

### 9.3 Todo state must reflect actual immutable session state

Remove false unconditional "todo list is currently empty" behavior.

At every model-call boundary, capture immutable todo state after any tool updates in the same run.

Render deterministic escaped/capped ephemeral user-role context; do not promote task content to system policy and do not persist synthetic reminders to transcript/history.

Preserve session task ordering; caps account for escaping/wrappers/truncation and report omitted counts.

Required proof includes create/update/complete, restart persistence, XML edge cases, truncation, and absence of synthetic reminder messages in API history.

---

## 10. Phase 2 — PR2 prompt architecture

Status: **NOT STARTED**.

Goal: separate stable policy/context from dynamic per-run material without changing instruction semantics.

### 10.1 Architecture

Create typed immutable snapshots and pure renderers/collectors for:

- stable system prefix;
- dynamic system suffix;
- ephemeral user context.

Stable includes policy, ordered context, canonical skills index.

Dynamic includes working directory/platform/date/Git/MCP/run notes where allowed.

Todo remains ephemeral user-role context.

Do not split arbitrary rendered content by searching for markers.

### 10.2 Single skills owner

`skills.Manager.ActiveSkills()` (or the accepted owning API) must be the single source used to render skills.

Remove duplicate prompt-side filesystem discovery.

Initial build, refresh endpoint, and pre-run refresh must use the same builder logic.

### 10.3 Stable generation

Stable generation/hash changes for actual stable inputs, including same-size context edits, skills, model/template/policy changes.

Git/date/MCP/todo dynamic changes must not invalidate stable generation.

Use content-addressed stable snapshot identity so unchanged bytes remain stable across restart/refresh.

Do not cache model/tool objects in this phase merely because stable prompt rendering now exists.

### 10.4 Acceptance

Prove:

- same snapshot => byte-identical output;
- dynamic-only edits affect only dynamic identity;
- stable changes advance stable generation exactly once;
- concurrent refresh/runs do not mix revisions;
- `/agent/refresh-prompt` refreshes prompt state without resetting session/queue/history;
- fake provider captures expected stable/dynamic behavior across unchanged, Git change, skill change, and MCP instruction change turns.

---

## 11. Phase 3 — PR4 TACK ownership and migration

Status: **NOT STARTED**.

Goal: establish one product-managed core plus one user-owned context layer, with safe migration from legacy TACK data.

### 11.1 Ownership model

Target:

- `TACK_CORE.md`: product-managed Gotack-specific identity/capabilities/integration policy;
- `USER.md`: user-managed context/customization;
- generic coding/workflow policy remains in the coder template and must not be duplicated across policy owners.

New installs must not seed legacy `TACK.md` as an active parallel owner.

### 11.2 Migration state machine

Implement explicit durable states/generations such as:

```text
legacy
pending
staged
committed-layered
rolled-back
```

Stage and validate complete state before commit marker/retirement of legacy owner.

Preserve committed generation across crash/restart.

Stock legacy hashes may migrate automatically only when exact known stock is proven.

Modified/unknown legacy content must not be overwritten; use preview/manual acceptance and 3-way merge only with known exact base bytes.

Do not invent an unknown base or use an LLM/heuristic to extract "rules".

Preview/accept must be compare-and-swap against generation/hash so concurrent user edits produce conflict instead of overwrite.

Rollback token/backup/state must survive restart and must not be auto-migrated away on the next seed.

### 11.3 UI/host contract

Implement real Wails bound methods, desktop bridge, generated bindings, and actual preview/accept/rollback UI.

Contract stubs or allowlists are not UI proof.

Update relevant layout docs/ADR/contracts in the same change.

### 11.4 Safety evidence

All migration tests use isolated temp profiles and portable builds, never the owner's live profile.

Required tests include new install, known stock migration, modified legacy, unknown hash, concurrent conflict, interrupted commit, durable rollback, restart recovery, provider prompt capture showing exactly one generic rule owner, and agent-browser evidence for user flows.

---

## 12. Phase 4 — PR5 OpenAI Responses reasoning continuity (P0 release requirement)

Status: **NOT STARTED**.

Goal: preserve full ordered reasoning state across stream -> message -> DB -> replay, including tool loops, restart, and bounded post-summary history selection.

### 12.1 Message model

Represent ordered reasoning parts individually rather than one collapsed metadata record.

Each part may carry:

- event/output identity;
- provider item ID;
- encrypted content;
- provider/model/account-scope fingerprint metadata;
- timestamps;
- optional display summary metadata.

Start/delta/end/finish updates must target the correct part and preserve unrelated fields.

Deep-copy nested metadata.

Encrypted-only parts must be serializable/replayable even when display thinking text is empty.

### 12.2 Fantasy/OpenAI mapping

Upstream (or temporary reviewed pin when allowed) must convert reasoning parts into structured Responses input items in correct relative order, including:

```json
{"type":"reasoning","id":"...","encrypted_content":"...","summary":[]}
```

Never replay thinking/summary as assistant text.

Duplicate item IDs are deterministic pre-network errors.

### 12.3 Replay contract

Use explicit local-history replay with `store=false`.

Reject incompatible `previous_response_id`/store combinations before dispatch.

Drop incompatible anchors on provider/model/account-scope switch and emit a safe reason such as `model_switch`; never log/hash ciphertext.

Legacy rows missing compatibility metadata may be replayed only when compatibility can be established safely from existing metadata; otherwise produce an explicit unsupported reason.

### 12.4 Bounded compaction/history selection

Do not build hybrid compaction.

Fix only the approved PR5 history-selection behavior so post-summary replay retains one complete latest valid assistant anchor group with required associated call/results, never an orphan/duplicate half-group.

Recovery tests must ensure no half-summary/half-anchor becomes committed state.

### 12.5 Acceptance

Required proof includes:

- metadata lifecycle and JSON round-trip;
- encrypted-only multi-item preservation;
- structured Fantasy conversion/order;
- fake-provider two-turn + tool loop + restart replay;
- model switch strips incompatible ciphertext;
- forced compaction/summary keeps latest valid anchor group;
- no ciphertext in logs/JSONL/SSE/UI transcript;
- live synthetic acceptance against the exact OpenAI Responses endpoint/model before claiming real provider continuity.

Fantasy changes must be upstreamed when required by `ImplementPlan.md`; do not create an indefinite private fork. A local `replace` is never a release solution.

---

## 13. Phase 5 — PR3 prompt_cache_key experiment (conditional)

Status: **NOT STARTED / CONDITIONALLY REQUIRED**.

Do not implement/enable this early merely to satisfy a checklist.

Entry requirements:

- PR0 telemetry is trustworthy;
- correctness and data-safety phases are integrated;
- current candidate request/prefix shape is stable enough to benchmark;
- telemetry indicates an opportunity worth measuring.

Default remains OFF.

### 13.1 Benchmark design

Use real executable/REST/SSE/provider path for live performance; fake provider validates harness/correctness only.

Freeze account, endpoint, model, reasoning effort, tool schema, prompt/history fixtures, engine/Fantasy revisions, warm-up, concurrency, seed, and request budget.

Use paired control/treatment from equivalent isolated starting state, randomized AB/BA, minimum independent pairs/workload per `ImplementPlan.md`.

Record all retries/errors/timeouts and missing semantic text; do not silently discard outliers or turn absent TTFT into zero.

Percentile/bootstrap method and rollout thresholds are fixed by `ImplementPlan.md`; do not tune thresholds after seeing results.

### 13.2 Rollout

Only roll out if the preregistered improvement/non-regression criteria pass with sufficient precision.

If evidence is insufficient or negative:

```text
leave feature OFF
record result/limitations
close phase as no-rollout
```

That is a valid completion. Forcing the experiment ON is not.

---

## 14. Hybrid compaction workstream

Status: **BLOCKED / OUT OF MILESTONE**.

Do not implement local/hybrid compaction algorithm in PR0–PR5.

Only create ADR/contract/fixture design after its entry gates are satisfied and only if separately authorized.

Do not opportunistically change summary algorithm, summary role, compaction threshold, file-state reconciliation, or lifetime token accounting while implementing the current milestone.

---

## 15. Windows release gate

No release/"performance improved"/"reasoning continuity complete" claim until the integrated candidate has evidence for every applicable gate.

Minimum categories:

- clean-pin patch replay in correct phase order;
- candidate provenance and binary SHA;
- Gotack repository tests;
- focused and nested Crush/Fantasy tests as applicable;
- repository invariants;
- frontend checks/build/tests for touched UI work;
- real fail-closed fake-provider E2E;
- real REST/SSE terminal event path;
- retry/tool-loop counts greater than zero where expected;
- security canary scan;
- Windows named-pipe tests;
- ACL/portable migration tests;
- migration crash/recovery/rollback tests;
- OpenAI Responses live reasoning acceptance;
- Windows race evidence when environment/toolchain blocker is resolved;
- live benchmark evidence only if making performance/cache claims.

Linux/WSL and Linux case-sensitive filesystem tests are not milestone requirements.

If the Windows race toolchain remains unavailable, report the release gate as BLOCKED; do not quietly waive it.

---

## 16. Provider compatibility policy

Do not require the owner to list every provider before coding.

Use this strategy:

1. fake provider for deterministic pipeline and E2E gates;
2. OpenAI OAuth / Responses for P0 live reasoning acceptance;
3. current provider compatibility matrix for other providers after observability exists;
4. repair other providers only when evidence isolates an issue within the phase scope or the owner explicitly authorizes a provider-specific workstream.

A provider already broken at baseline must not automatically be blamed on new code, but new code also must not make it worse unnoticed. Record baseline-known provider failures when discovered through reproducible tests.

---

## 17. Evidence format returned to the owner after every checkpoint

Every coding checkpoint report must include:

### Outcome

- intended scope;
- completed scope;
- explicitly not completed / deferred.

### Changes

- branch;
- commit SHA;
- files changed;
- engine/Fantasy patch/provenance impact.

### Validation already possible in Web environment

List only commands actually executed and their real result.

### Windows commands for owner

Provide exact copy/paste commands, including working directory and any temporary environment variables.

### Expected observations

For each command, state what constitutes PASS/FAIL/BLOCKED.

### Evidence to send back

Request only relevant outputs/artifacts. Never request secrets.

### Rollback

State how to return to the previous candidate without destroying the owner's dirty ignored engine checkout or real profile.

### Remaining risks

List unresolved correctness/security/runtime/environment questions.

---

## 18. Initial status ledger for the next Web session

```text
Baseline Gotack go test ./...       PASS (owner Windows, before implementation)
App launches                        PASS (owner observation)
Gotack root working tree            CLEAN before WebPlan creation
Crush pin                           6d14dd93a9e526505f7de54ae5999431bc32a793
Owner third_party/crush             DIRTY; DO NOT RESET/CLEAN
Windows go test -race               BLOCKED_ENVIRONMENT (CGO disabled; C toolchain not proven)
Scratch Crush prototype             NOT AUTHORITY; tracked dirty edits cleaned
Scratch Fantasy prototype           NOT AUTHORITY; tracked dirty edits cleaned
Parent third_party/fantasy          ABSENT
Known provider set                  MULTIPLE; some baseline failures suspected
OpenAI OAuth                        AVAILABLE per owner
Phase 0A                            NOT STARTED
Phase 0B                            NOT STARTED
Phase 1                             NOT STARTED
Phase 2                             NOT STARTED
Phase 3 / PR4 migration             NOT STARTED
Phase 4 / PR5 reasoning             NOT STARTED
Phase 5 / PR3 cache experiment      NOT STARTED / CONDITIONAL
Hybrid compaction                   BLOCKED / DO NOT IMPLEMENT
Release approval                    NOT GRANTED
```

Update this ledger only with actual evidence. Do not mark a phase complete merely because code was written or a document checkbox was checked.

---

## 19. First action for the next session

When the owner starts a new session and asks to implement this plan:

1. inspect current `main` and verify `WebPlan.md`/`ImplementPlan.md` have not been superseded;
2. read `AGENTS.md` and `docs/WORKFLOW.md`;
3. inspect current patch/replay/E2E scripts and current `third_party/patches` inventory;
4. do **not** touch the owner's ignored `third_party/crush` working tree;
5. create/resume the durable active implementation plan if repository workflow requires it;
6. create branch `impl/phase-0a-harness-provenance` (or a clearly equivalent bounded branch);
7. implement **Phase 0A only**;
8. commit and hand the owner exact Windows tests;
9. wait for owner evidence in the conversation before declaring the Phase 0A gate accepted and before starting Phase 0B.

The new session must not ask the owner to repeat already recorded baseline facts unless repository state has changed or new evidence is genuinely required.

---

## 20. Definition of success

The Web implementation is successful only when:

- the requested architecture/correctness/data-safety behavior exists in code;
- engine/Fantasy modifications are durably reproducible from accepted upstream/pins;
- the owner has supplied Windows runtime evidence for required gates;
- all failures/blockers/skips are reported truthfully;
- security constraints remain intact;
- no live user data was endangered by migration testing;
- optional cache rollout is evidence-based and may legitimately remain OFF;
- release claims match the evidence actually observed.

Until then, the correct status is "implementation in progress", not "done".
.