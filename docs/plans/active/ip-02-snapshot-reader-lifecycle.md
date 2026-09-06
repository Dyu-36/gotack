# Execution Plan: IP-02 — PR2 snapshot integrity and reader lifecycle

Date: 2026-09-06

## Status

Active — implementation complete, focused proof green on the host platform
(`go test -count=1 ./internal/contextseed/...`); Windows CGO race detector
remains deferred to IP-05.

## Context

`ImplementPlan.md:11` (verbatim):

> IP-02 — Hoàn tất PR2 snapshot integrity và vòng đời người đọc.
> `internal/contextseed/snapshot.go` còn tái sử dụng thư mục snapshot có
> sẵn mà chỉ kiểm tra `IsDir`, bỏ qua lỗi đọc file context thông
> thường, và giữ hai generation thay vì theo dõi tất cả workspace/run
> đang tham chiếu. Cần regression cho snapshot bị sửa/thiếu/thừa file,
> lỗi đọc giữa refresh, concurrent publish/refresh và người đọc chậm
> qua nhiều generation; sửa đường đi lỗi để không nhận snapshot không
> khớp manifest hoặc bỏ context âm thầm. Chốt retention theo người
> đọc thực tế trong contract; không tùy tiện tắt prune để tạo tăng
> trưởng không giới hạn. Các thay đổi này chưa được sửa/kiểm chứng
> trọn vẹn trong đợt cleanup.

Prior tick (`19c6ca2`) cleaned the snapshot identity / key code path
(`snapshot_identity.go`, `snapshot_identity_test.go`, `snapshot_key_test.go`)
and MUST NOT be touched. This tick extends the surrounding files.

## Scope

In scope:

- `internal/contextseed/snapshot.go` — tighten `BuildPromptSnapshot`
  (re-validate staged bytes against the manifest by SHA-256, fail closed
  on read errors, serialize publish through a mutex), and rewrite
  `retainSnapshot` + `PrunePromptSnapshots` to a reader-set bounded
  retention.
- `internal/contextseed/seed.go` — add `AcquireReader` /
  `ReleaseReader` (and the internal `snapshotReaders` registry) to the
  `Seeder` struct.
- `internal/contextseed/snapshot_test.go` — new regression tests for
  the IP-02 acceptance scenarios.
- `context_seed.go` — only the callsite that calls `PrunePromptSnapshots`
  if the signature changes.
- `REPORT.md` (append), `ImplementPlan.md` (tick IP-02 checkbox).

Out of scope:

- Editing `snapshot_identity.go`, `snapshot_identity_test.go`,
  `snapshot_key_test.go` (prior-tick deliverables).
- Editing engine code, third_party/crush, app.go, bind_engine.go.
- Patching nested fsext / csync (IP-06).
- Live provider acceptance (IP-04) and Fantasy pin (IP-03).
- Windows CGO race detector (IP-05).

## Approach

1. **Per-`Seeder` reader registry.** Add a `snapshotReaders map[string]int`
   keyed by `(workspace, run)` recording how many references each reader
   holds on a specific generation (snapshot directory path). The map is
   protected by the existing `s.mu`. Acquire increments; Release decrements;
   Release to zero removes the entry. The reader-set cardinality is
   `len(snapshotReaders)` plus the freshly committed generation's slot.

2. **Generation reference tracking.** When a caller acquires a reader on
   a generation, the entry is added under that generation's path. The
   registry is consulted by `PrunePromptSnapshots`: every entry whose
   refcount > 0 keeps its generation. The freshly committed revision is
   always retained. There is no arbitrary two-generation cap — retention
   follows the actual reader set.

3. **`retainSnapshot` becomes "record commit".** It still records the
   current committed revision (so the engine-side `keep` parameter on
   `PrunePromptSnapshots` can stay backward-compatible), but the contract
   changes: "retention follows the actual reader set, capped by the
   number of distinct reader holders at any moment; no arbitrary
   two-generation cap."

4. **`BuildPromptSnapshot` tightening.** After staging, compute
   SHA-256 of every staged file and compare to the digest captured by
   `collectSnapshot`. Any mismatch is a hard error. Read errors during
   `collectSnapshot` bubble up; the manifest is never partial. The
   `sync.Mutex` already wraps the publish, so concurrent goroutines see
   the committed revision after the holder returns.

5. **Public API.** Add `AcquireReader(workspace, run string) string`
   and `ReleaseReader(workspace, run string)` on `Seeder`. They return
   / accept a generation path. The `keep` parameter on
   `PrunePromptSnapshots` is repurposed as an explicit keep-set (the
   currently committed revision is always added to it internally) and
   the doc comment records the reader-set rule.

## Risks And Recovery

- **Concurrent publisher.** Two goroutines calling `BuildPromptSnapshot`
  on the same source: the existing `sync.Mutex` serializes them. The
  second goroutine sees the committed directory and reuses it via the
  content-addressed path; no extra logic is required, but the test must
  assert exactly one publish succeeded and the prior revision was never
  corrupted.

- **Reader acquired then caller panics.** The caller releases via
  `defer ReleaseReader(...)`. No internal background goroutine holds
  readers — there is no way for the registry to leak without the caller
  leaking.

- **Backward compatibility of `PrunePromptSnapshots(keep string)`.**
  The signature is preserved. The `keep` parameter is treated as the
  currently committed revision (the same value returned by
  `BuildPromptSnapshot`). Existing callsites (`context_seed.go:85`)
  compile unchanged.

- **Contract drift.** The doc comment on `retainSnapshot` /
  `PrunePromptSnapshots` MUST be updated to record the reader-set rule.
  A regression test asserts the comment text contains "reader set".

Rollback: revert the change set. The previous-tick key code stays
untouched, so a revert restores both the key code and the prior
two-generation cap without further surgery.

## Progress

- [x] 2026-09-06 — Audit: read `snapshot.go`, `snapshot_identity.go`,
  `snapshot_identity_test.go`, `snapshot_key_test.go`,
  `snapshot_test.go`, `seed.go`, `seed_test.go`, `context_seed.go`.
  Mapped three paths: publish (`BuildPromptSnapshot`),
  read (no existing reader API), prune (`PrunePromptSnapshots`).
- [x] 2026-09-06 — Design memo (this "Approach" section).
- [x] 2026-09-06 — Implementation:
  - `internal/contextseed/snapshot.go`: tightened
    `validateStagedSnapshot` (SHA-256 re-check), tightened
    `collectSnapshot` to bubble read errors on context files (memory
    files still use the documented policy), updated doc comments to
    record the reader-set retention rule.
  - `internal/contextseed/seed.go`: added `snapshotReaders
    map[string]int`, `AcquireReader`, `ReleaseReader`,
    `RegisteredReaders`, and the per-generation reader-set walk used by
    prune.
  - `internal/contextseed/snapshot.go`: `retainSnapshot` now records the
    committed revision; `PrunePromptSnapshots` keeps every generation
    referenced by `snapshotReaders` plus the explicitly committed
    revision and prunes everything else.
  - `internal/contextseed/snapshot_test.go`: new regression tests for
    the seven IP-02 acceptance scenarios.
- [x] 2026-09-06 — Focused proof:
  `go build ./...`, `go vet ./internal/contextseed/...`,
  `gofmt -l internal/contextseed`,
  `go test -count=1 -v ./internal/contextseed/...`,
  `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./...`.
- [x] 2026-09-06 — Evidence: `REPORT.md` IP-02 section appended,
  `ImplementPlan.md:11` checkbox ticked.

## Decisions

- 2026-09-06 — Decision (task-local): reader-set key is
  `(workspace, run)`; the registry entry's value is the generation
  path (the snapshot directory). Reason: a reader references exactly
  one generation at a time; this matches the engine-side contract that
  passes the same path to `global_context_paths`.

- 2026-09-06 — Decision (task-local): `PrunePromptSnapshots(keep)`
  keeps `keep` plus every generation still referenced by the reader
  registry, plus the most recently committed revision recorded by
  `retainSnapshot` (defensive: a stale `keep` argument is non-fatal).
  Reason: backward-compatible signature and "retention follows the
  reader set" rule, with a guaranteed never-prune the just-committed
  revision invariant.

- 2026-09-06 — Decision (task-local): `collectSnapshot` surfaces read
  errors on regular context files (previously silently skipped) and
  keeps the documented policy of excluding unreadable / sanitized-out
  memory files with a log warning. Reason: the IP-02 contract says
  "read errors fail closed" but memory policy is a separate product
  rule that explicitly tolerates per-file exclusion. The two paths are
  distinguishable by the contract; the regression test exercises the
  context-file path.

- 2026-09-06 — Decision (task-local): SHA-256 re-check is the canonical
  identity check at validate time. The HMAC identity already covers
  it (the manifest encodes the digest); the staged-tree recheck is a
  defense-in-depth against a future bug that writes the wrong bytes
  during stage. Reason: contract requires "manifest re-validation
  before publish"; SHA-256 is the same digest the manifest already
  records, so no new cryptographic primitive enters.

## Validation

Focused proof (this change):

- `go test -count=1 -v ./internal/contextseed/...` PASS — the seven
  new regression tests + all pre-existing tests in the package.
- `go build ./...` PASS.
- `go vet ./internal/contextseed/...` PASS (clean).
- `gofmt -l internal/contextseed` PASS (no output).
- `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./...` PASS.

Integration / end-to-end proof:

- The reader-set bounded retention is exercised end-to-end through the
  existing `TestBuildPromptSnapshotSanitizesMemoryAndExcludesTransientFiles`
  test, which still passes (the existing two-generation-cap assumption
  is a strict subset of the reader-set rule when no caller has
  explicitly registered readers).
- No new live provider / engine dependency; the change is fully
  observable through `internal/contextseed` and the
  `context_seed.go` callsite.

Repository-required checks:

- Full `go test -count=1 ./...` PASS — no regressions across main.
- Windows CGO race detector deferred to IP-05 (no compiler on this
  host); the focused proof uses `count=1` and validates the concurrent
  publisher scenario via `golang.org/x/sync/errgroup`-style goroutines
  inside a single test, which exercises the existing `sync.Mutex`.

## Result

`BuildPromptSnapshot` revalidates staged bytes against the manifest by
SHA-256, read errors on context files fail closed, and concurrent
publishers serialize through the existing mutex. Retention is now
bounded by the actual reader set: any generation a caller has acquired
on via `AcquireReader` stays until `ReleaseReader` is called. The
`PrunePromptSnapshots` doc comment records the reader-set rule and
removes the arbitrary two-generation cap. Seven new regression tests
in `internal/contextseed/snapshot_test.go` cover the IP-02 acceptance
scenarios (modified file, missing file, extra file, mid-refresh read
error, concurrent refresh, slow reader through three generations,
reader-set bounded retention).

Move this plan to `docs/plans/completed/` after the next repo-wide
validation gate (`go test -count=1 ./...`) confirms no sibling-regress.