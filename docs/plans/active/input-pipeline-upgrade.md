# Input Pipeline Upgrade Execution

Authority: `ImplementPlan.md`, `WebPlan.md`, `AGENTS.md`, and
`docs/WORKFLOW.md`. Updated 2026-09-05.

## Owner decision and boundary

The owner's latest explicit instruction is to implement directly on `main`,
without creating a Gotack branch or worktree. The 2026-09-05 owner instruction
also overrides all older per-phase stops, owner-test pauses and sequential
checkpoint handoffs in ImplementPlan/WebPlan. Implement every remaining phase
continuously, with independent agents assigned explicit file ownership; run
consolidated acceptance after integration. Safety, architecture, provenance and
release acceptance requirements remain mandatory. Hybrid/local compaction is
outside this milestone. Live paid calls have no authorized budget.

Current execution starts at `cc48aeeabbbdbf61e9a089a736d39bb01408d613` on main.
Pre-existing owner edits in wails-bindings.md, timetable_template_test.go and
the timetable skill/template must be preserved. Older evidence below is historical,
not acceptance of this candidate. Status vocabulary: IMPLEMENTED means code exists;
PASS means an executed check passed; FAIL, BLOCKED, UNVERIFIED and OUT_OF_SCOPE
are distinct and must never be converted into PASS.

## Current progress ledger

Updated 2026-09-05 after integration: the vendored-engine patch landed, the
E2E gate passes with the patched engine, and the benchmark driver was
validated end-to-end. Statuses use the owner vocabulary: IMPLEMENTED (code
exists), PASS (executed check passed), FAIL, BLOCKED, UNVERIFIED, OUT_OF_SCOPE.

| Scope | Status | Evidence / remaining work |
| --- | --- | --- |
| Phase 0A | PASS (baseline engine) | Real Windows clean-pin replay/build + 5 required executable E2E tests passed twice on the unpatched baseline (`d0ada5ba…` provenance) |
| PR0 consumer | IMPLEMENTED + unit PASS | runmetrics Writer/validation/redaction wired via `bind_engine.go`; optional sorted `change_reasons` with allowlist validation; contract doc updated |
| PR0 engine | IMPLEMENTED + wire PASS | `input-pipeline-core.patch` + `input-pipeline-telemetry.patch`: RunTrace monotonic spans, tri-state cache with run-scoped token sums, run-scoped `attempt`, domain-separated HMAC fingerprints (unavailable stays unavailable), sorted unique `change_reasons`. Telemetry patch adds: per-run prompt generation with labeled component digests so `prefix_changed_reason`/`change_reasons` come from generation diffs (template→context, model_switch, context, skills, tool_set; dynamic-only date/git_status/mcp/todo never move the primary reason), per-kind one-shot first reasoning/tool_call/text offsets as nullable pointers (absent stays absent; a sub-microsecond event is a real zero, never conflated with absent), `first_semantic` aligned to the contract token `tool_call`, and `request_shape_hmac` computed at the end of `PrepareStep` over the FINAL prepared request (history shape, attachment metadata, canonical per-tool schemas; credentials/ciphertext/tool-output excluded). E2E wire PASS: HMAC shape, per-kind timing presence, stable prefix across turns, no invented stable reasons. `first_byte_to_first_sse` still needs a Fantasy transport hook and stays absent (null), never zero. A parallel remote PR0 patch series (`9cd9d27`) was superseded by this patch and removed from the manifest during the merge |
| PR1 | IMPLEMENTED (wire-proofed) | Engine: ordered context groups, Windows-canonical rendered paths, provider-options layers merged as flat maps with pre-network validation (`auto\|concise\|detailed`, union include with `reasoning.encrypted_content`, explicit null omits), todo reminder re-rendered from a fresh session snapshot at every model-call boundary inside `PrepareStep` as ephemeral user-role context. Gate-required E2E wire proofs PASS: options preserved, invalid options rejected with zero provider requests, todo reminder reflects real state + survives restart + never persisted, 20-restart canonical-prompt proof (`TestE2ECanonicalPromptAcrossRestarts`), MCP instruction-order proof under reversed config order (`TestE2EMCPInstructionOrderDeterministic`). Change-reason auto-detection landed with the telemetry patch (see PR0 engine) |
| PR2 | IMPLEMENTED (host + engine, tested) | Host (`b53a09b`): `context-prompt/snapshot-<UnixNano>` replaced by an install-key HMAC identity over the canonical manifest (layout version, migration mode, ordered case-folded source-relative paths, per-file content digests); identical content reuses the committed immutable directory across refresh and restart; a same-size edit rotates the identity exactly once; collect-once → validate → atomic rename; a failed refresh keeps the committed revision and no longer clears the registered engine path; identity-key corruption fails closed (no unkeyed fallback); bounded retention keeps current+previous generations. Engine: prompt build returns one `PromptBuild` (text + stable/dynamic split + labeled generation) so every builder path describes the same inputs; stable/dynamic HMACs and generation-diff reasons wire-verified. Engine tests PASS (generation-diff unit suite, same-size context edit, date dynamic-only, model/skills/template kinds). Full E2E gate PASS |
| PR4 | IMPLEMENTED (host+UI) | Transactional migration, Wails bind methods, desktop.ts bridge, ContextMigrationPanel UI, ADR 0006, contract + layout docs; `go build`, focused tests, frontend check/test/build all PASS. Portable agent-browser flow UNVERIFIED (owner gate); migration tests only on temp profiles |
| PR5 | IMPLEMENTED (Crush side) + BLOCKED (Fantasy upstream, live) | New `input-pipeline-history-anchor.patch`: bounded history-selection contract in `selectHistoryWithAnchor` — keeps the latest complete valid assistant anchor group (ordered reasoning parts + tool calls + results, never split, duplicated or orphaned; incomplete groups walked back; no anchor when none complete), and drops uncommitted summary messages so a crash at the summarize boundary leaves no half-committed state. Unit suite PASS (8 tests). `store=false` default and `previous_response_id`+replay rejection are provided by Fantasy v0.41.3 itself (`params.Store` defaults to false; `validatePreviousResponseIDPrompt` rejects the conflict pre-network), and the authoring patch additionally emits ordered reasoning items + duplicate-ID rejection. Fantasy patch verified with focused tests on pristine v0.41.3 (SHA `8d455a58…`, `release_eligible: false` — upstream submission/pinning needs owner authorization); E2E compaction anchor proof PASS (`TestE2ECompactionPreservesLatestAnchorGroup`); live acceptance BLOCKED_LIVE_ACCEPTANCE |
| PR3 | IMPLEMENTED (infrastructure, validated) | Paired AB/BA driver validated end-to-end against the patched engine: telemetry records written, nearest-rank percentiles + 10k bootstrap CI aggregated, report always `decision: no-rollout`, `prompt_cache_key_default: OFF`. Synthetic correctness only; live preregistered gate BLOCKED_LIVE_ACCEPTANCE |
| Hybrid compaction | OUT_OF_SCOPE | Only bounded PR5 anchor selection authorized |
| Release | PARTIALLY VERIFIED | At the final HEAD: full E2E gate PASS (14 gate-required tests including the six new release-matrix proofs, zero unexpected skips), `go test ./...`, `go vet ./...` (+`-tags=e2e`), `staticcheck ./...`, repository invariants, harness unit tests, frontend check/test/build all PASS. Remaining for release claim: live Responses acceptance (owner budget), Windows race gate (`go test -race` BLOCKED_ENVIRONMENT: CGO/GCC unavailable, re-probed this session), portable migration UI flow (owner gate), upstream Fantasy patch authorization (BLOCKED_OWNER_AUTH) |

Committed checkpoint series on `main`: `0348877` (contextseed migration),
`ef2722e`/`6ef1259`/`5a0eec8` (runmetrics, benchmark scaffolding, e2e
rejection counters), `81119be` (fantasy authoring patch), `c564e66` (bench
driver), `3c1de3e` (migration UI), `a3b9002` (change_reasons), `c6522d4`
(fantasy tests), `7796ca8` (wire-proof E2E tests), `12c0908` (checkpoint
ledger), `437ee9a` (engine input-pipeline patch), `5008a26` (todo reminder at
model-call boundary + options at the selected-model layer), `1554cbd`
(flat option layers in fixtures), `46fcada`/`d8fb10b`/`9878038` (gated
synthetic item diagnostics), `749c817` (tool-item streamed arguments),
`ceda015` (transcript read order), `7751877` (benchmark schedule
subcommand), `f614c39` (bench treatment flat layer), `8518393` (return the
owner's timetable edits to uncommitted state), `af0407f` (merge of the
remote PR0 telemetry lane: hardening opt-out + telemetry E2E coverage
kept, superseded telemetry patches removed), `6f7efa6` (TTFB span +
adopted telemetry coverage), `de2738b` (run-scoped attempt and token
semantics), `b53a09b` (content-addressed prompt snapshots),
`54f1177` (telemetry generation diff + final-request fingerprint +
compaction anchor patches, contract updates), `a6d212f` (release-matrix
E2E evidence), `211425d` (gate enforcement of the new proofs).

The starting Gotack commit was `b6dcf68320b708df7a5e3c8e1750689cf5621ec1`.
The Crush pin is owned by `.tack-pin`; the owner's ignored `third_party/crush`
is known dirty and must not be reset, cleaned, or used for reconstruction.

## Implemented in this checkpoint

- Explicit patch manifest: compatibility -> hardening -> input-pipeline.
  The final phase currently has no accepted patches. Removed the ignored-flag
  behavior of `SkipInputPipeline`, and fail on incomplete patch inventory.
- Unique isolated clean-pin fetch/build, root `.` entrypoint, checked native
  exits/timeouts, no PATH engine fallback, and verified provenance for SkipBuild.
  Git directory override environment variables are removed from child Git/build
  commands; no owner global settings or services are changed.
- Real executable/REST/SSE Windows named-pipe tests replacing the old scaffold:
  fresh turn, actual 429 retry, MCP JSON-RPC stdio tool loop, restart with the
  same database, and rejection of malformed provider SSE.
- Dependency-free tests for missing binary, readiness timeout, zero captures,
  malformed schemas/lifecycle, dropped terminal, invalid provenance, missing
  required tests, unexpected skips, and unsupported platform.
- A separate Windows CI lane calling the same entrypoint and uploading only
  the safe receipt/test-summary artifacts. No branch protection was changed.

The prior checked-off PR0/PR4 entries described partial prototypes. They were
not full wire/migration/UI/release proof and are superseded by this record.
Scratch commit `40d74a1` and the previously described nonexistent zz patch are
not implementation authority or accepted provenance.

## Evidence actually observed in the Web environment

Environment: Linux, Node 22.16.0, Go 1.23.2; no PowerShell or Windows runtime.
The runtime had candidate source files only, not a full Gotack checkout.

| Check | Observed result |
| --- | --- |
| `node --test scripts/input-pipeline/gate.test.mjs` on candidate files | PASS: 12 tests, no skips |
| `GO111MODULE=off go test -count=1 -timeout=20s -v e2e/inputpipeline/fixtures_test.go e2e/inputpipeline/harness_test.go` | PASS initially; expanded suite subsequently passed with race checking below |
| `GO111MODULE=off go test -race -count=1 -timeout=30s -v e2e/inputpipeline/fixtures_test.go e2e/inputpipeline/harness_test.go` | PASS: 8 top-level tests plus 5 negative-control subtests on Linux; not Windows race evidence |
| `gofmt` on the three changed Go test files | Applied; formatting rechecked |
| Real PowerShell clean-pin replay/build | BLOCKED in this environment; not claimed PASS |
| Real Windows executable/REST/SSE/MCP tests | NOT RUN here; candidate not accepted |
| Full Gotack tests, invariants, vet, frontend checks | NOT RUN here; full checkout/toolchain unavailable |
| Windows race gate | BLOCKED_ENVIRONMENT per owner baseline: CGO/compiler not proven |
| New CI job run / required branch check status | Unverified until an actual run is inspected |

The orchestration tests inject a command runner for build-order/provenance
fixtures. Their PASS does not prove that PowerShell patches apply or that the
engine builds. Child exit/timeout tests and local HTTP/MCP fixture tests do
execute real local code, but are not Windows product E2E evidence.

## Owner Windows commands

From the owner's existing Gotack checkout on `main`, with no local tracked
edits to the gate inputs:

```powershell
git pull --ff-only
if ($LASTEXITCODE -ne 0) { throw 'Pull failed' }
node --test scripts/input-pipeline/gate.test.mjs
if ($LASTEXITCODE -ne 0) { throw 'Harness unit tests failed' }
go test ./...
if ($LASTEXITCODE -ne 0) { throw 'Gotack tests failed' }
node scripts/check-repository-invariants.mjs
if ($LASTEXITCODE -ne 0) { throw 'Repository invariants failed' }
./scripts/test-input-pipeline-e2e.ps1
```

The final script must report eight required E2E/negative-control tests as
RUN/PASS and zero skips. Its printed unique artifact directory contains
`provenance.json`, `tests.jsonl`, and (only on success) `result.json`.
Return these safe artifacts and the command exit status. Do not return tokens,
raw provider requests, engine profiles, or raw engine logs.

For direct tagged tests, all four explicit variables are required:
`TACK_ENGINE_BINARY`, `TACK_ENGINE_PROVENANCE`, `TACK_E2E_REPO_ROOT`, and
`TACK_E2E_NODE` (absolute Node executable). Prefer the script, which verifies
and supplies them without modifying the parent shell environment.

## Acceptance and remaining work

Release acceptance still requires owner-side items that this milestone cannot
supply: a live Responses acceptance run (no authorized paid budget), the
Windows race gate on a CGO-capable toolchain (re-probed 2026-09-05: `gcc`
absent, `CGO_ENABLED=0` — installing a toolchain was not authorized), the
portable migration UI flow in the packaged app, and an upstream Fantasy patch
decision (`release_eligible: false`; the Crush-side engine behavior that the
Fantasy patch unblocks — ordered reasoning-item replay — is fully implemented
and tested at the unit level, and v0.41.3 itself already provides the
`store=false` default and the `previous_response_id` replay-conflict
rejection). PR2's content-addressed snapshots, PR0's generation-diff change
reasons and final-request fingerprint, and PR5's bounded anchor-group
history selection are now IMPLEMENTED with executable evidence; they are no
longer open engine-patch work items. Failure anywhere is failure, never a
skip or a weaker assertion.

### Consolidated validation evidence (2026-09-05, this session)

| Check | Result |
| --- | --- |
| `./scripts/test-input-pipeline-e2e.ps1` (clean-pin replay + build + full E2E) | PASS: 14 gate-required tests, zero unexpected skips |
| `go test ./...` (Gotack) | PASS |
| `go vet ./...` and `go vet -tags=e2e ./e2e/...` | PASS |
| `staticcheck ./...` | PASS |
| `node scripts/check-repository-invariants.mjs` | PASS |
| `node --test scripts/input-pipeline/gate.test.mjs` | PASS (12 tests) |
| `pnpm --dir frontend check` / `test` | PASS (0 errors / 39 tests) |
| Nested patched-engine focused tests (agent, agent/prompt, message) | PASS |
| Nested patched-engine full `go test ./...` | One failure: `internal/fsext` `TestGlobWithDoubleStar`, verified failing on the pristine pin with no patches (pre-existing upstream Windows glob behavior, outside this milestone); all other nested packages PASS (`-p 2`; higher parallelism OOMs the machine) |
| Nested patched-engine `go vet ./...` | One pre-existing upstream finding in pinned `internal/csync/maps.go` (lock-by-value), untouched by all patches |
| `go test -race` on Windows | BLOCKED_ENVIRONMENT (no CGO compiler; re-probed) |
| Live Responses acceptance | BLOCKED_LIVE_ACCEPTANCE (no owner budget) |
| Fantasy upstream/pin authorization | BLOCKED_OWNER_AUTH |

## Recovery

Revert the checkpoint commit on `main` with `git revert <checkpoint-sha>` if
needed; do not reset/clean the owner's ignored engine checkout or real profile.
The harness only owns its uniquely named OS-temp build directory and test
profiles/processes. Stop/cleanup is scoped to processes started by the test.
A build/cleanup timeout or missing terminal is failure, never PASS.
