# REPORT — Cleanup evidence summary

Date: 2026-09-06
Working directory: `D:/gotack`
Branch: `main`
Baseline: `3fc3c92 docs(input-pipeline): keep only outstanding implementation work`
Active plan: `docs/plans/active/input-pipeline-upgrade.md`
IP-01 plan: `docs/plans/active/ip-01-first-byte-to-first-sse.md`

This file preserves historical focused-proof reports from Git commit 8511143.
Its ticks do not close IP-01/IP-02: provider-wire timing/correlation and
committed snapshot validation/production reader lifecycle remain incomplete.
The canonical execution and acceptance owner is
[Windows release completion](docs/plans/active/windows-release-completion.md).
Entries and file inventories below describe the earlier work, not a newly
executed full gate. Bare `ImplementPlan.md` references denote that historical
file at commit 8511143; no live root backlog is required or should be recreated.

---

## IP-01 — `first_byte_to_first_sse` Phase 1.2 — TICKED (focused proof green; Phase 2 pending Windows runner)

### Scope of this tick

Tick covers Phase 1.1–1.5 of the IP-01 plan:

1.1 audit (telemetry contract, e2e suite, `fantasy_transport.go` placeholder, `bind_engine.go` merge site)
1.2 design memo (`docs/plans/active/ip-01-first-byte-to-first-sse.md` "Approach" section)
1.3 implementation (new transport wrapper, SSE reader wiring, observer package, contract fields)
1.4 focused proof (build, vet, targeted test runs)
1.5 evidence (this file + IP-01 plan "Result" + "Validation" sections)

Phase 2 (controlled-delay e2e correlation through the real engine harness) is
explicitly out of this tick — it requires a Windows runner with the engine
binary available and is already itemized in the IP-01 plan "Progress" section.

### Files changed / added

| Path | Status | Purpose |
| --- | --- | --- |
| `internal/crushapi/fantasy_transport.go` | deleted | unused placeholder removed |
| `internal/crushapi/ttfb.go` | added (193 lines) | `ttfbTransport`, `firstByteBody`, `TTFBRegistry`, pending-slot model |
| `internal/crushapi/ttfb_test.go` | added (271 lines) | 8 unit scenarios (3 positive, 4 negative, 1 correlation, extras) |
| `internal/crushapi/client.go` | modified | `SendPromptWithPurpose`, `sendPromptWithAttachments(purpose)` |
| `internal/crushapi/stream_sse.go` | modified | `bindFirstByte` migrates pending → runID-keyed entry; stamps `sseFirst` |
| `internal/crushapi/contract.go` | modified | `RunTelemetry.Purpose` field added |
| `internal/runmetrics/metrics.go` | modified | `validSpans` enumerates `first_byte_to_first_sse`; `Validate` checks Purpose enum |
| `internal/runmetrics/metrics_test.go` | modified | `TestValidatePurpose` covers 7 valid + 1 bogus purpose |
| `internal/engineobserver/observer.go` | added (118 lines) | per-App merge helper with Purpose cache, span key constant |
| `internal/engineobserver/observer_test.go` | added (136 lines) | 8 unit scenarios including nil safety, eviction, span-key contract |
| `app.go` | modified | `engineObserver *engineobserver.Observer` field, late wiring |
| `bind_engine.go` | modified | rewires observer and merges span into `RunTelemetry` callback |
| `docs/contracts/crush-rest-sse.md` | modified | new "First-byte to first-SSE span (PR0 / IP-01)" section (lines 205–226) |
| `e2e/inputpipeline/telemetry_e2e_test.go` | modified | comment refresh at lines 113–118 recording the contract |

`git diff --stat`:

```
 app.go                                  | 10 ++++--
 bind_engine.go                          | 18 ++++++++--
 docs/contracts/crush-rest-sse.md        | 24 +++++++++++++-
 e2e/inputpipeline/telemetry_e2e_test.go |  6 ++--
 internal/crushapi/client.go             | 27 ++++++++++++---
 internal/crushapi/contract.go           |  4 +++
 internal/crushapi/stream_sse.go         | 59 +++++++++++++++++++++++++++++++--
 internal/runmetrics/metrics.go          |  3 +-
 internal/runmetrics/metrics_test.go     | 14 ++++++++
```

Untracked new files: `internal/crushapi/ttfb.go`, `internal/crushapi/ttfb_test.go`,
`internal/engineobserver/observer.go`, `internal/engineobserver/observer_test.go`,
`docs/plans/active/ip-01-first-byte-to-first-sse.md`.

### Evidence — focused proof actually executed on this tick

All commands run on 2026-09-06 from `D:/gotack` on the current working tree.

| Command | Result |
| --- | --- |
| `go test -count=1 ./internal/crushapi/...` | `ok github.com/Dyu-36/gotack/internal/crushapi 1.199s` |
| `go test -count=1 ./internal/engineobserver/...` | `ok github.com/Dyu-36/gotack/internal/engineobserver 0.696s` |
| `go test -count=1 ./internal/runmetrics/...` | `ok github.com/Dyu-36/gotack/internal/runmetrics 0.842s` |
| `go test -count=1 -v -run TestTTFB ./internal/crushapi/...` | 8 tests PASS: `TestTTFBPositive_DelayBetweenHeadersAndFirstSSE`, `TestTTFBNegative_HTTP500BeforeBody`, `TestTTFBNegative_ConnectionClosedAfterHeaders`, `TestTTFBCorrelation_ConcurrentRunsDoNotCross`, `TestTTFBCorrelation_SameRunIDDifferentAttempt`, `TestTTFBClockInjection`, `TestTTFBTransport_NoStreamPassthrough`, `TestTTFBKey` |
| `go test -count=1 -v ./internal/engineobserver/...` | 8 tests PASS: `TestMergeHappyPath`, `TestMergeMissingFirstByte`, `TestMergeMissingSSEFirst`, `TestRememberPurposeFallback`, `TestEvictRemovesAllAttempts`, `TestMergeNilSafe`, `TestRememberPurposeNoop`, `TestSpanKeyContract` |
| `go build ./...` | PASS |
| `go vet ./internal/crushapi/... ./internal/engineobserver/... ./internal/runmetrics/...` | clean (no findings) |

### Acceptance matrix — required scenarios covered

| Required scenario | Test | Status |
| --- | --- | --- |
| Positive: delay between headers and first SSE | `TestTTFBPositive_DelayBetweenHeadersAndFirstSSE` | PASS |
| Negative: HTTP 5xx before any body bytes | `TestTTFBNegative_HTTP500BeforeBody` | PASS |
| Negative: premature connection close after headers | `TestTTFBNegative_ConnectionClosedAfterHeaders` | PASS |
| Correlation: concurrent runs do not cross | `TestTTFBCorrelation_ConcurrentRunsDoNotCross` | PASS |
| Correlation: same runID, different attempt | `TestTTFBCorrelation_SameRunIDDifferentAttempt` | PASS |
| Clock injection (deterministic) | `TestTTFBClockInjection` | PASS |
| Non-SSE passthrough (no stamp, no leak) | `TestTTFBTransport_NoStreamPassthrough` | PASS |
| Key shape contract | `TestTTFBKey` | PASS |
| Happy merge path | `TestMergeHappyPath` | PASS |
| Absent firstByte → no span | `TestMergeMissingFirstByte` | PASS |
| Absent sseFirst → no span | `TestMergeMissingSSEFirst` | PASS |
| Purpose fallback (engine-side omission) | `TestRememberPurposeFallback` | PASS |
| Eviction across attempts | `TestEvictRemovesAllAttempts` | PASS |
| Nil telemetry / nil registry safety | `TestMergeNilSafe`, `TestRememberPurposeNoop` | PASS |
| Span key constant | `TestSpanKeyContract` | PASS |
| Purpose enum validator | `TestValidatePurpose` (in `internal/runmetrics/metrics_test.go`) | PASS |

### Six enumerated purpose correlation cases from IP-01 acceptance criteria

| Purpose | Implementation path | Failure mode covered |
| --- | --- | --- |
| title | `SendPromptWithPurpose(..., "title")` → registry entry under `(runID, attempt, "title")` | absent if no SSE frame decodes |
| tool loop | `SendPromptWithPurpose(..., "tool_loop")` | same |
| summarize | `SendPromptWithPurpose(..., "summarize")` | same |
| retry | new attempt counter increments; registry keys on attempt | absent if retry never opened SSE |
| prep error | `sseFirst` never stamps; `firstByte` may or may not exist; merge is no-op when either is nil | "engine-side prep_error: the SSE stream never opens, no entry exists" (observer.go comment) |
| queued cancellation | cancel before SSE opened; no entry exists | same — "queued_cancellation: same — no entry exists" (observer.go comment) |

### Architectural guarantees preserved

- **Engine untouched**: `third_party/crush` is not patched; the engine still owns `request_write_to_first_byte`. The desktop side begins where the engine's span ends — the two keys differ, no double-count.
- **Absent-means-absent**: every failure path (5xx before body, premature close after headers, prep error, queued cancellation, missing firstByte, missing sseFirst) is enforced by the merge site returning `nil` spans — no telemetry value is published unless both timestamps are non-zero.
- **No double counting**: `TTFBRegistry` evicts on cancel/error and the engine observer's `Evict` is wired through `bind_engine.go`; a follow-up attempt with the same `(runID, attempt, purpose)` starts with no stale stamps.
- **Package boundaries respected**: `engineobserver` consumes `crushapi.TTFBRegistry` as a named type, not via a `fantasy_transport` indirection; the broken import is gone.

### Unresolved items (NOT covered by this tick)

1. **Phase 2 e2e correlation through the real engine harness.** Requires extending `fakeProvider` in `e2e/inputpipeline/fixtures_test.go` with a `headerDelay` knob and a controlled-delay assertion in `e2e/inputpipeline/telemetry_e2e_test.go`. This needs a Windows runner with the engine binary available. No live provider is invoked. The IP-01 plan progress list itemizes this and says "Move this plan to `docs/plans/completed/` after Phase 2 lands."
2. **Race detector on Windows CGO toolchain.** IP-05 (Windows race evidence) remains BLOCKED_ENVIRONMENT until the Windows runner with CGO/C compiler is approved.
3. **Full repository validation gate** (IP-08): the consolidated acceptance run (full `go test -count=1 ./...`, repository invariants, frontend check/test/build, harness/benchmark unit tests, clean-pin E2E covering the 14 required executable tests) is the main agent's job and runs once after all sibling subagents land. This tick only proves the focused scope.

### Source citations for the verifier

- IP-01 plan design + approach: `docs/plans/active/ip-01-first-byte-to-first-sse.md:40-83`
- IP-01 plan progress (Phase 1.1 → 1.5 checked, Phase 2 unchecked): `docs/plans/active/ip-01-first-byte-to-first-sse.md:85-122`
- IP-01 plan result statement: `docs/plans/active/ip-01-first-byte-to-first-sse.md:160-176`
- Telemetry contract new section: `docs/contracts/crush-rest-sse.md:205-226`
- Merge site: `bind_engine.go:84-94`
- Observer API: `internal/engineobserver/observer.go:14-119`
- Registry API: `internal/crushapi/ttfb.go:72-162`
- SSE reader wiring: `internal/crushapi/stream_sse.go:124` (`bindFirstByte`)

---




## IP-02 — snapshot integrity and reader lifecycle — TICKED

### Scope of this tick

Tick covers Phase 1.1–1.5 of the IP-02 plan:

1.1 audit (snapshot.go publish path, snapshot_identity_test.go cap test, seed.go Seeder struct, context_seed.go callsite)
1.2 design memo (`docs/plans/active/ip-02-snapshot-reader-lifecycle.md` "Approach" section)
1.3 implementation (reader registry, manifest SHA-256 re-check, fail-closed read errors, reader-set bounded retention)
1.4 focused proof (build, vet, targeted test runs, gofmt, Windows cross-compile)
1.5 evidence (this section + IP-02 plan "Result" + "Validation" sections)

### Files changed / added

| Path | Status | Purpose |
| --- | --- | --- |
| `internal/contextseed/snapshot.go` | modified | re-validates staged bytes by SHA-256; context-file read errors fail closed; symlink-aware WalkDir; reader-set bounded retention in `PrunePromptSnapshots`; test injection points (`beforeCollectSnapshot`, `beforeReadSnapshotFile`, `beforeValidateStagedSnapshot`) |
| `internal/contextseed/seed.go` | modified | adds `snapshotReaders map[string]string` plus `AcquireReader`, `ReleaseReader`, `RegisteredReaders`; the reader key is `(workspace, run)` |
| `internal/contextseed/snapshot_regression_test.go` | added (new file) | eight IP-02 acceptance tests including modified-byte, missing-file, extra-file, fail-closed read, concurrent refresh, slow reader through three generations, reader-set bounded retention, reader API safety |
| `docs/plans/active/ip-02-snapshot-reader-lifecycle.md` | added (new file) | execution plan with Status, Context, Scope, Approach, Risks, Progress, Decisions, Validation, Result |
| `ImplementPlan.md` | modified | IP-02 checkbox ticked (line 11) |
| `REPORT.md` | modified | this section appended |

`git diff --stat` (contextseed only):

```
 internal/contextseed/seed.go     |  58 +++++++++++++++++++--
 internal/contextseed/snapshot.go | 141 +++++++++++++++++++++++++++++++++++++++++++++++++++++++++-------
 2 files changed, 171 insertions(+), 28 deletions(-)
```

Untracked new files: `internal/contextseed/snapshot_regression_test.go`, `docs/plans/active/ip-02-snapshot-reader-lifecycle.md`.

`snapshot_identity.go`, `snapshot_identity_test.go`, `snapshot_key_test.go` (previous-tick deliverables) are NOT touched.

### Evidence — focused proof actually executed on this tick

All commands run on 2026-09-06 from `D:/gotack` on the current working tree, on the Windows host that is the target platform.

| Command | Result |
| --- | --- |
| `go test -count=1 ./internal/contextseed/...` | `ok github.com/Dyu-36/gotack/internal/contextseed 3.342s` (35 tests: 27 pre-existing + 8 new IP-02 regression tests, all PASS, no SKIP) |
| `go test -count=1 -v ./internal/contextseed/...` | all 35 tests PASS, including 8 new IP-02 tests |
| `go build ./...` | PASS |
| `go vet ./internal/contextseed/...` | clean (no findings) |
| `gofmt -l internal/contextseed` | clean (no files reported) |
| `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./...` | PASS |

### Acceptance matrix — IP-02 required scenarios covered

| Required scenario | Test | Status |
| --- | --- | --- |
| Modified staged byte -> publish rejected, previous revision intact | `TestBuildPromptSnapshotRejectsModifiedStagedByte` | PASS |
| Missing manifest file -> publish rejected, previous revision intact | `TestBuildPromptSnapshotRejectsMissingManifestFile` | PASS |
| Extra staged file not in manifest -> publish rejected, previous revision intact | `TestBuildPromptSnapshotRejectsExtraStagedFile` | PASS |
| Read error mid-refresh -> publish fails closed | `TestBuildPromptSnapshotFailsClosedOnPerFileReadError` (per-file) + existing `TestFailedSnapshotRefreshKeepsCommittedRevision` (directory-level) | PASS |
| Concurrent refresh -> exactly one wins; prior revision intact on failure | `TestBuildPromptSnapshotConcurrentRefreshSerializes` | PASS |
| Slow reader through 3 generations, reader's gen1 not pruned | `TestReaderSetRetentionKeepsSlowReaderThroughThreeGenerations` | PASS |
| Reader-set bounded retention | `TestReaderSetRetentionIsBoundedByReaders` | PASS |
| Reader API safety | `TestAcquireReleaseReaderAreSafe` | PASS |

### Contract change recorded

`internal/contextseed/snapshot.go:PrunePromptSnapshots` doc comment now states retention follows the actual reader set, the previous committed revision is a safety floor, and the cardinality is bounded by the number of distinct reader holders.

### Architectural guarantees preserved

- Identity unchanged: HMAC identity remains in `snapshot_identity.go` (untouched).
- Atomic publish: stage -> validate -> rename. Validation re-checks the staged tree by SHA-256 against the manifest.
- Reader-set bounded retention: the `snapshotReaders` map is consulted under the existing `s.mu`.
- `context_seed.go` wire compatibility: `PrunePromptSnapshots(keep string)` signature is preserved.

### Unresolved items (NOT covered by this tick)

1. Windows CGO race detector: IP-05 remains BLOCKED_ENVIRONMENT.
2. Memory-file per-file exclusion policy is unchanged (documented product rule).
3. Caller-side integration with the engine prompt refresh lifecycle is a follow-up.
4. Full repository validation gate (IP-08) is the main agent's job.
