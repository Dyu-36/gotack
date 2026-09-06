# Execution Plan: IP-01 — `first_byte_to_first_sse` with correlation

Date: 2026-09-06

## Status

Historical work-package record, superseded for execution by
[Windows release completion](windows-release-completion.md), WP2.
IP-01 acceptance remains OPEN: the baseline host workspace-SSE observation
is not provider-wire timing and call/attempt correlation is incomplete.
Original implementation claims, checkboxes and commands below are preserved
as history, not current acceptance. Bare root-plan references mean the
historical file at Git commit 8511143; local:// evidence labels are not
verified repository files. Do not follow obsolete follow-up instructions.

## Context

- `ImplementPlan.md` IP-01: PR0 telemetry must report `first_byte_to_first_sse`
  with model-call / HTTP-attempt / purpose correlation for title, tool loop,
  summarize, retry, prep error and queued cancellation. The first HTTP byte
  must not be conflated with a complete SSE frame.
- `local://ip-01-design.md` (this directory's parent): design memo. Read
  before continuing.
- `local://ip-01-impl-evidence.md` (this directory's parent): implementation
  evidence. Updated by each tick.
- `docs/contracts/crush-rest-sse.md:205-226`: new "First-byte to first-SSE
  span (PR0 / IP-01)" section appended.
- `internal/runmetrics/metrics.go:208-216`: `validSpans` already lists the
  span; the new code publishes it.
- `third_party/crush/internal/agent/agent.go:1183-1189`: engine still emits
  only `request_write_to_first_byte`; the desktop side closes the gap
  without patching the engine.

## Scope

In scope:

- Implement `first_byte_to_first_sse` measurement on the host side of REST
  + SSE with `(runID, attempt, purpose)` correlation.
- Six negative + positive unit scenarios in `internal/crushapi/ttfb_test.go`.
- Host-side merge into `RunTelemetry.SpansMicros` at the
  `bind_engine.go:87-91` `RunTelemetry` callback.
- Delete `internal/crushapi/fantasy_transport.go` placeholder and the
  broken import in `transport.go`.
- Update `docs/contracts/crush-rest-sse.md` and the e2e comment block.

Out of scope:

- Patching `third_party/crush` or the Fantasy engine.
- Live provider acceptance (IP-04 remains BLOCKED_LIVE_ACCEPTANCE).
- Windows race evidence (IP-05 remains BLOCKED_ENVIRONMENT).
- Auto-summarize / hybrid compaction (out of milestone).

## Approach

1. **Crush-side observation (no engine patch).** A `ttfbTransport` wraps
   the `http.Client.Transport` and only intercepts responses whose
   `Content-Type: text/event-stream`. Each intercepted response gets a
   `*firstByteBody` that lazy-stamps a monotonic timestamp on the first
   successful `Read`. A per-Client `TTFBRegistry` stores entries keyed
   by `(runID, attempt, purpose)` with atomic timestamps.
2. **SSE first-frame stamp.** `readEvents` extracts `runID` from the first
   decoded `run_complete` envelope, migrates the pending `firstByte` into
   the runID-keyed entry, and stamps `sseFirst` at that instant.
3. **Host merge.** A new `internal/engineobserver` package owns a per-App
   observer with a last-known Purpose cache. `Merge(*RunTelemetry)` looks
   up the registry, computes `sseFirst.Sub(firstByte)` in microseconds,
   writes the span only when both timestamps are non-zero, and evicts.
4. **Validation.** `runmetrics.Validate` enforces the Purpose enum.

## Risks And Recovery

- **Race on registry key when runID repeats with the same Purpose.** The
  `bind_engine.go` host passes Attempt=1 per fresh call; if two genuine
  attempts share `(runID, Purpose)`, the second overwrites the first.
  Mitigated by the engine-side attempt counter reset; the e2e test
  `TestE2ETelemetryStablePrefixStableAcrossTurns` proves the digest
  shape is unaffected.
- **Body wrapper streaming interference.** The wrapper is a strict
  passthrough after stamping; `bufio.Reader` depends only on `Read`/`ReadByte`,
  both implemented. No regressions in `stream_sse_test.go`.
- **Engine does not echo `telemetry.purpose` today.** The observer falls
  back to the Purpose the host stamped on the POST body (cached
  per-runID), so absence on the wire does not break correlation.

Rollback: revert the change set (this directory plus the files listed in
"Files changed"). The pre-existing
`fantasy_transport.go` placeholder never ran, so reverting only removes
the new observation; it does not restore prior functionality.

## Progress

- [x] 2026-09-06 — Phase 1.1 audit (PHASE 1.1 of the cleanup plan) reads
  the telemetry contract, e2e suite, fantasy_transport placeholder and
  bind_engine.go merge site.
- [x] 2026-09-06 — Phase 1.2 design memo `local://ip-01-design.md`.
- [x] 2026-09-06 — Phase 1.3 implementation:
  - `internal/crushapi/fantasy_transport.go` deleted.
  - `internal/crushapi/transport.go` broken import removed.
  - `internal/crushapi/ttfb.go` (193 lines) and `ttfb_test.go` (271 lines) added.
  - `internal/crushapi/client.go` wraps Transport; new
    `SendPromptWithPurpose`; internal `sendPromptWithAttachments` adds
    `purpose` param.
  - `internal/crushapi/stream_sse.go` binds firstByte and stamps sseFirst.
  - `internal/crushapi/contract.go` adds `RunTelemetry.Purpose`.
  - `internal/runmetrics/metrics.go` validates Purpose enum.
  - `internal/engineobserver/observer.go` (118 lines) + `observer_test.go`
    (136 lines) added.
  - `app.go` adds `engineObserver *engineobserver.Observer` field.
  - `bind_engine.go` rewires observer and merges span into telemetry.
  - `docs/contracts/crush-rest-sse.md:205-226` new section appended.
  - `e2e/inputpipeline/telemetry_e2e_test.go:113-118` comment refreshed.
- [x] 2026-09-06 — Phase 1.4 focused proof:
  `go build`, `go vet`, `go test -count=1 ./internal/crushapi/...`,
  `go test -count=1 ./internal/runmetrics/...`,
  `go test -count=1 ./internal/engineobserver/...`,
  `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ./...`,
  `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./...`,
  `go test -count=10 ./internal/crushapi/... ./internal/engineobserver/...`.
- [x] 2026-09-06 — Phase 1.5 evidence: `local://ip-01-impl-evidence.md`
  populated with the acceptance table and test inventory.
- [ ] 2026-09-06 — Phase 2 follow-up: extend `fakeProvider` in
  `e2e/inputpipeline/fixtures_test.go` with a `headerDelay` knob and add
  the controlled-delay assertion to
  `e2e/inputpipeline/telemetry_e2e_test.go` so the populated path is
  exercised end-to-end against the real engine harness. This requires a
  Windows runner with the engine binary available; no live provider is
  invoked.

## Decisions

- 2026-09-06 — Decision (task-local): the registry stores pending
  `firstByte` per-response until the SSE reader extracts `runID` from
  the first `run_complete` envelope. Reason: the SSE stream URL carries
  only `wsID`, not `runID`; the engine publishes `runID` inside the SSE
  body. Pending-slot + runID-keyed entry is the minimal correct shape.
- 2026-09-06 — Decision (task-local): the host merge is the only
  publication site for `first_byte_to_first_sse`. Reason: hard rule 4
  forbids duplicating the engine; the engine's
  `request_write_to_first_byte` is untouched.
- 2026-09-06 — Decision (task-local): the registry defaults `purpose=""`
  for legacy callers; the merge site skips correlation when Purpose is
  empty but still writes the span if both timestamps are non-zero.
  Reason: existing public `SendPromptWithAttachments` callsites stay
  binary-compatible.

## Validation

- Focused proof (this change):
  - `go test -count=1 ./internal/crushapi/...` PASS (8 new TTFB tests +
    existing tests).
  - `go test -count=1 ./internal/engineobserver/...` PASS (8 tests).
  - `go test -count=1 ./internal/runmetrics/...` PASS
    (`TestValidatePurpose` covers 7 valid values + 1 bogus).
  - `go test -count=10 ./internal/crushapi/... ./internal/engineobserver/...`
    PASS (race detector unavailable without cgo on this Windows host).
  - `go build ./...`, `GOOS=linux go build ./...`, `GOOS=windows go build ./...`
    PASS.
  - `go vet ./...` PASS.
- Integration / end-to-end proof: deferred to Phase 2 above (controlled
  delay fixture variant). The e2e comment at
  `e2e/inputpipeline/telemetry_e2e_test.go:113-118` records the contract.
- Repository-required checks: full
  `go test -count=1 ./...` PASS — no regressions across main,
  e2e/inputpipeline, or internal packages.

## Result

`first_byte_to_first_sse` is observable on the desktop side of REST +
SSE with `(runID, attempt, purpose)` correlation, validated by six
positive + negative unit scenarios and Purpose enum validation. The
engine continues to own `request_write_to_first_byte`; the new span
begins exactly where the engine's span ends and does not double-count
because the two keys differ. Absent-means-absent governs every path
(5xx before body, premature close after headers, prep error, queued
cancellation, missing firstByte, missing sseFirst).

The `first_byte_to_first_sse` span is now wire-ready. Integration with
the real engine harness via a controlled-delay synthetic fixture is
queued as Phase 2 and requires a Windows runner with the engine
binary; no live provider is invoked and no third_party files are
modified.

Move this plan to `docs/plans/completed/` after Phase 2 lands.