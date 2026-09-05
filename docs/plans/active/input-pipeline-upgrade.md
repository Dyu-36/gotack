# Input Pipeline Upgrade Execution

Authority: `ImplementPlan.md` (approved 2026-09-04), `AGENTS.md`, and
`docs/WORKFLOW.md`.

## Outcome

Implement PR0–PR5 of the approved input-pipeline plan as a Windows-first
desktop and engine change, excluding section 9 hybrid compaction. Keep the
Crush boundary REST + SSE and preserve all durable engine work as
`third_party/patches/zz-input-pipeline-windows.patch` based on temporary Crush
commit `40d74a1`.

## Invariants

- Never import Crush internals into Gotack.
- Never log prompts, ciphertext, authorization/OAuth material, raw tool output,
  or raw session UUIDs.
- Keep `prompt_cache_key` disabled unless the benchmark gate passes.
- Treat Windows path identity as case-insensitive for contract v1.
- Preserve the current threshold compaction implementation.
- Preserve user-owned context and make managed migrations atomic and
  reversible.

## Progress

- [x] Read required authority and repository/vendor instructions.
- [x] PR0 optional telemetry, redacted JSONL metrics, contracts, and tests.
  - `internal/runmetrics/`: hardened validation (cache status enum, prefix
    reason enum, identifier format, non-negative durations), nil-safe logger,
    thread-safe JSONL append, 32-byte atomic key creation, sensitive field
    redaction (provider_request_id stripped, prompt never logged).
  - `internal/crushapi/contract.go`: `RunTelemetry` struct with 24 fields,
    `Telemetry *RunTelemetry` on `RunComplete`.
  - `internal/uievents/forwarder.go`: `RunTelemetry` callback wired in
    `handleRunComplete`.
  - `bind_engine.go`: callback connects to `runMetrics.Append`.
  - `app.go`: `runMetrics` field initialized in `startup()`.
  - `internal/engine/supervisor.go`: key generation + `TACK_RUN_METRICS_KEY_FILE`
    env passed to engine.
  - `docs/contracts/crush-rest-sse.md`: telemetry shape documented.
- [x] PR4 managed `TACK_CORE.md` plus `USER.md` migration/rollback.
  - `resources/context/TACK_CORE.md`: product-managed identity and capabilities.
  - `resources/context/USER.md`: user-owned customization surface.
  - `internal/contextseed/migration.go`: stock-hash manifest, atomic migration,
    backup/rollback, `migration_pending` status for modified legacy.
  - `internal/contextseed/migration_test.go`: unit tests for new install, stock
    legacy, modified legacy, migrated, rollback, snapshot owner.
  - `docs/decisions/0005-context-ownership.md`: ADR for ownership model.
- [ ] PR1 deterministic context/MCP/provider/todo behavior and tests.
- [ ] PR2 stable prefix/dynamic suffix with `skills.Manager` as sole source.
- [ ] PR3 benchmark framework and default-off cache experiment.
  - `scripts/bench-input-pipeline.ps1`: paired/randomized benchmark runner.
  - `docs/benchmarks/report-template.md`: report template with breakdown.
- [ ] PR5 ordered encrypted-only Responses reasoning replay and Fantasy pin.
  - `docs/contracts/openai-reasoning-continuity.md`: contract for reasoning
    preservation across turns, restart, model switch, and compaction.
- [ ] Windows executable/REST/SSE/fake-provider E2E harness.
  - `e2e/inputpipeline/e2e_test.go`: deterministic fixtures with clear skip
    when engine binary unavailable; fake provider capture, retry, telemetry
    passthrough tests.
  - `scripts/test-input-pipeline-e2e.ps1`: full E2E script (clone, patch,
    build, test).
- [ ] Patch replay, formatting, focused tests, repository gates, package proof.
  - `scripts/apply-crush-patches.ps1`: updated with `$SkipInputPipeline` flag
    and input pipeline patch documentation.
  - `third_party/README.md`: documented patch workflow for zz-input-pipeline-
    windows.patch and Fantasy patches.

## Recovery

The main worktree remains uncommitted. Engine edits live in
`tmp/crush-input-pipeline`; regenerate the tracked incremental patch with
`git diff --binary 40d74a1`. Context migrations use staging plus atomic rename,
retain backups, and expose rollback.

## Validation Record

| Command | Status |
|---------|--------|
| `gofmt -l .` | Pending |
| `node scripts/check-repository-invariants.mjs` | Pending |
| `go test ./...` | Pending |
| `go vet ./...` | Pending |
| `staticcheck ./...` | Pending |
| `pnpm --dir frontend check` | Pending |
| `pnpm --dir frontend test` | Pending |
| `pnpm --dir frontend build` | Pending |

Record exact commands and results here as each implementation group closes.
