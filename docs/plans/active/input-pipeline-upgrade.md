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
| PR0 engine | IMPLEMENTED | `input-pipeline-core.patch`: RunTrace with monotonic spans (ready_wait/mcp_wait/model_refresh/history_load/prompt_prepare/stream plus the OnChunk-derived `request_write_to_first_byte` TTFB), one-shot first semantic, tri-state cache with run-scoped token sums, run-scoped `attempt` (completed steps plus retries), domain-separated HMAC request fingerprints (unavailable stays unavailable), sorted unique `change_reasons`. Run-level semantics align with the merged remote telemetry E2E coverage (fresh/retry/tool-loop). The socket-byte to first-SSE split (`first_byte_to_first_sse`) still needs a Fantasy transport hook and stays absent (null), never zero. A parallel remote PR0 patch series (`9cd9d27`) was superseded by this patch and removed from the manifest during the merge |
| PR1 | IMPLEMENTED (wire-proofed) | Engine: ordered context groups, Windows-canonical rendered paths, provider-options layers merged as flat maps with pre-network validation (`auto\|concise\|detailed`, union include with `reasoning.encrypted_content`, explicit null omits), todo reminder re-rendered from a fresh session snapshot at every model-call boundary inside `PrepareStep` as ephemeral user-role context. Gate-required E2E wire proofs PASS: options preserved, invalid options rejected with zero provider requests, todo reminder reflects real state + survives restart + never persisted. Remaining: 20-restart canonical-prompt wire proof and MCP instruction-order proof not run; `prefix_changed_reason`/`change_reasons` auto-record only `initial`/`compaction` (no automatic generation-diff detection yet) |
| PR2 | PARTIAL | Ordered prompt groups/sorting and `skills.Manager` single-source present in the engine patch; content-addressed snapshot generation and same-size context edit rebuild detection NOT implemented (`context-prompt/snapshot-<timestamp>` instability from Plan §0.3 remains) |
| PR4 | IMPLEMENTED (host+UI) | Transactional migration, Wails bind methods, desktop.ts bridge, ContextMigrationPanel UI, ADR 0006, contract + layout docs; `go build`, focused tests, frontend check/test/build all PASS. Portable agent-browser flow UNVERIFIED (owner gate); migration tests only on temp profiles |
| PR5 | PARTIAL | Fantasy patch verified with focused tests on pristine v0.41.3 (SHA `8d455a58…`, `release_eligible: false` — upstream submission/pinning needs owner authorization); Crush-side ordered reasoning-item replay plumbing in the engine patch; live acceptance BLOCKED_LIVE_ACCEPTANCE; bounded anchor-group history selection and `store=false`/`previous_response_id` rejection NOT implemented |
| PR3 | IMPLEMENTED (infrastructure, validated) | Paired AB/BA driver validated end-to-end against the patched engine: telemetry records written, nearest-rank percentiles + 10k bootstrap CI aggregated, report always `decision: no-rollout`, `prompt_cache_key_default: OFF`. Synthetic correctness only; live preregistered gate BLOCKED_LIVE_ACCEPTANCE |
| Hybrid compaction | OUT_OF_SCOPE | Only bounded PR5 anchor selection authorized |
| Release | PARTIALLY VERIFIED | At the final HEAD: full E2E gate PASS (8 required tests, zero unexpected skips), `go test ./...`, `go vet ./...` (+`-tags=e2e`), `staticcheck ./...`, repository invariants, frontend check/test/build all PASS. Remaining for release claim: live Responses acceptance (owner budget), Windows race gate (`go test -race` BLOCKED_ENVIRONMENT: CGO unavailable), portable migration flow, upstream Fantasy patch authorization |

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
semantics).

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
Windows race gate on a CGO-capable toolchain, the portable migration flow in
the packaged app, and an upstream Fantasy patch decision. PR2's
content-addressed snapshot generation and PR5's bounded anchor-group history
selection remain implemented-NO (not silently dropped): they are the next
engine-patch work items. Failure anywhere is failure, never a skip or a
weaker assertion.

## Recovery

Revert the checkpoint commit on `main` with `git revert <checkpoint-sha>` if
needed; do not reset/clean the owner's ignored engine checkout or real profile.
The harness only owns its uniquely named OS-temp build directory and test
profiles/processes. Stop/cleanup is scoped to processes started by the test.
A build/cleanup timeout or missing terminal is failure, never PASS.
