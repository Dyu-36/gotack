# Input Pipeline Upgrade Execution

Authority: `ImplementPlan.md`, `WebPlan.md`, `AGENTS.md`, and
`docs/WORKFLOW.md`. Updated 2026-09-05.

## Owner decision and boundary

The owner's latest explicit instruction is to implement directly on `main`,
without creating a Gotack branch or worktree. This overrides only the older
branch instructions in `WebPlan.md` sections 4, 5, and 19. Phase order, safety,
evidence, and owner Windows acceptance requirements still apply.

Current checkpoint: Phase 0A harness/provenance candidate. Do not start Phase
0B or claim PR0-PR5/release completion before the Phase 0A Windows evidence is
accepted. Hybrid/local compaction remains outside this milestone.

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

The final script must report five required E2E/negative-control tests as
RUN/PASS and zero skips. Its printed unique artifact directory contains
`provenance.json`, `tests.jsonl`, and (only on success) `result.json`.
Return these safe artifacts and the command exit status. Do not return tokens,
raw provider requests, engine profiles, or raw engine logs.

For direct tagged tests, all four explicit variables are required:
`TACK_ENGINE_BINARY`, `TACK_ENGINE_PROVENANCE`, `TACK_E2E_REPO_ROOT`, and
`TACK_E2E_NODE` (absolute Node executable). Prefer the script, which verifies
and supplies them without modifying the parent shell environment.

## Acceptance and remaining work

Phase 0A remains IN PROGRESS until real Windows clean-pin replay/build and
all required executable tests pass and their negative-control behavior is
reviewed. Any mismatch in the pinned engine's actual API, fake schema, MCP
lifecycle, or replay must be fixed, not converted to a skip or weaker assertion.
The new CI lane is a validation entrypoint, not a successful validation run.

Phase 0B/PR0 observability, PR1 correctness, PR2 snapshots, PR4 transactional
migration/UI, PR5 reasoning continuity, and conditional PR3 benchmarks remain
unaccepted. This checkpoint does not change engine product behavior, migration
logic, provider options, reasoning replay, cache defaults, or compaction.

## Recovery

Revert the checkpoint commit on `main` with `git revert <checkpoint-sha>` if
needed; do not reset/clean the owner's ignored engine checkout or real profile.
The harness only owns its uniquely named OS-temp build directory and test
profiles/processes. Stop/cleanup is scoped to processes started by the test.
A build/cleanup timeout or missing terminal is failure, never PASS.
