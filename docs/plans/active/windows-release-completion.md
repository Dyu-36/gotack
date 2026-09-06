# Execution Plan: Hoàn tất Gotack Windows Release

Date: 2026-09-06
Status: ACTIVE — implementation và release acceptance CHƯA hoàn tất.
Canonical path: `docs/plans/active/windows-release-completion.md`
Baseline: `8511143de90d0ae13d3eb307fca692d7bf7e210d` trên `main`.

## 1. Outcome và cách dùng

AI tiếp nhận phải hoàn thiện candidate Windows x64 có input pipeline đúng, snapshot an toàn, reasoning continuity thực sự có trong dependency phát hành, và bằng chứng kiểm thử gắn với đúng candidate. Không kết thúc bằng việc tick checklist hoặc chỉ viết báo cáo.

Đây là plan điều phối duy nhất cho phần việc còn lại. Chủ repo ngày 2026-09-06 yêu cầu chọn phương án tốt nhất, viết plan bàn giao hoàn chỉnh và xóa `ImplementPlan.md` ở root. Yêu cầu lập plan KHÔNG có nghĩa implementation, external publication, chi phí live API hay cài compiler đã được thực hiện/phê duyệt.

AI thực thi một yêu cầu tiếp theo phải:
1. Đọc `AGENTS.md`, `docs/WORKFLOW.md`, plan này và đúng contract của phạm vi đang sửa.
2. Kiểm tra HEAD, diff và môi trường lại; baseline dưới đây không bảo đảm trạng thái tương lai.
3. Làm theo work package (WP), bổ sung fail-before/pass-after proof và cập nhật duy nhất mục Progress/Evidence của plan này.
4. Không làm lại tính năng đã có; chỉ sửa gap đã xác minh, tích hợp và nghiệm thu.
5. Nếu gặp blocker ngoài quyền được cấp, ghi chính xác blocker rồi làm tiếp WP độc lập an toàn; không tự bỏ gate và không dừng toàn bộ công việc vì một blocker không liên quan.

Sau khi toàn bộ gate bắt buộc đóng, chuyển plan vào `docs/plans/completed/` và cập nhật indexes/references trong cùng thay đổi. Không tạo lại backlog ở root.

## 2. Context, authority và baseline đã kiểm tra

### Tài liệu cần đọc

- `docs/contracts/crush-rest-sse.md`: REST/SSE, telemetry và context registration.
- `docs/contracts/openai-reasoning-continuity.md`, `provider-usage-and-compaction.md`: replay và anchor boundary.
- `docs/decisions/0001-narrow-no-agent-logic-rule.md`, `0005-context-ownership.md`, `0006-context-migration-transaction.md`.
- `docs/contracts/wails-bindings.md`, `gotack-reflection.md`, `gotack-memory-mcp.md`, `gotack-skills-mcp.md`, `gotack-recall-mcp.md` khi kiểm thử app/Hermes.
- `docs/patterns/encoding-invariants.md`: accepted authority, positive/negative proof và giới hạn CI enforcement.
- `third_party/README.md`, `.tack-pin`, `third_party/patches/manifest.json`, `third_party/fantasy-patches/manifest.json`.
- `REPORT.md`, `docs/plans/active/input-pipeline-upgrade.md`, `ip-01-first-byte-to-first-sse.md`, `ip-02-snapshot-reader-lifecycle.md`, `hermes-learning-loop.md` chỉ là evidence/history bổ trợ. Các tick/claim cũ không thay acceptance trong plan này.

Yêu cầu chi tiết trước khi root backlog bị rút gọn vẫn được giữ trong Git:

```powershell
git show 10a9879b745098f03ac0dcf97baa523125732c5e:ImplementPlan.md
```

Đây là historical blob reference, không phải yêu cầu phục hồi file đã xóa. Các ràng buộc gốc chưa được kiểm chứng không bị miễn trừ bởi việc thay plan. Khi prose cũ mâu thuẫn: giữ accepted safety/product invariants; dùng code và executable proof để xác định trạng thái, không dùng prose để chứng nhận code.

### Quan sát tại baseline (không phải kết quả implementation của plan này)

- Repo chính clean; `main` và local `origin/main` cùng baseline. Ignored `third_party/crush` dirty, detached ở pin `6d14dd93a9e526505f7de54ae5999431bc32a793`; tuyệt đối không reset/clean/update nó.
- CI `https://github.com/Dyu-36/gotack/actions/runs/34029296568`: Go tests/vet, frontend, Windows build PASS; Staticcheck FAIL vì `withCollectHook` và `validPurposes` unused. Deadcode, gofmt, generated-events check bị skip sau failure.
- Windows gate `https://github.com/Dyu-36/gotack/actions/runs/34029296538`: PASS cho harness hiện tại. Workflow đặt `CGO_ENABLED=0`; không phải Windows race hay live-provider acceptance.
- Read-only gofmt inventory: `app.go`, `bind_engine.go`, `internal/crushapi/contract.go`, `internal/crushapi/ttfb.go`, `internal/crushapi/ttfb_test.go`, `internal/engineobserver/observer.go`, `internal/engineobserver/observer_test.go`.
- IP-01: `stream_sse.go` chỉ lấy run ID từ `run_complete`, dùng first byte của workspace SSE connection; registry/merge dùng `(runID,0,"")`, có fallback `time.Now()`. `SendPromptWithPurpose`/`RememberPurpose` không có caller production. Comment nhắc `requireTelemetryWithDelay` nhưng chưa có implementation.
- IP-02: `snapshot.go` reuse/adopt existing directory tại baseline lines 157/198 chỉ kiểm tra `IsDir`; validation mới chỉ bao phủ staging. `AcquireReader`/`ReleaseReader` chưa có caller production.
- Fantasy authoring manifest: `not-submitted`, `release_eligible: false`; ignored engine dùng `charm.land/fantasy v0.41.3`. Không suy ra release binary đã chứa patch từ việc file patch tồn tại.
- Môi trường kiểm tra: Windows amd64, process mặc định CGO=0, không thấy gcc/clang/cl trên PATH; cache có Go 1.27.0/1.27.1. Không gắn nhãn thiếu Go chỉ từ phiên bản launcher ngoài repo.
- GitHub open issues/PRs: không có tại thời điểm kiểm tra. Đây không phải bằng chứng mọi defect đã được liệt kê.

## 3. Phương án đã chọn — không mở lại lựa chọn kiến trúc

1. **Correctness trước performance.** Sửa CI, đo đúng provider wire, đóng snapshot lifecycle, tích hợp Fantasy, rồi mới nghiệm thu và baseline.
2. **PR0 ở đúng layer.** Actual serializer/HTTP/SSE decoder instrumentation thuộc Fantasy/provider boundary và Crush-owned model calls. Desktop chỉ đọc/validate/ghi telemetry; không dùng workspace SSE connection làm proxy cho provider latency.
3. **Một registry observation theo model call và HTTP attempt.** Context truyền identity rõ ràng; root run là parent. `purpose` mô tả loại model call; retry là attempt/reason, prep error và queued cancellation là terminal outcomes, không tạo model call giả. Sửa contract/versioning additive trước khi đổi enum hiện có.
4. **Snapshot có manifest validation và explicit generation leases.** Giữ content-addressed HMAC identity; validate cả new/reused/adopted generation. Mỗi holder tham chiếu generation cụ thể, release theo lifecycle; không dùng cap hai generation để thay actual reader set, cũng không tắt prune vô hạn.
5. **Dependency phát hành phải tái lập được.** Gửi bounded Fantasy fixes theo upstream workflow khi có quyền; chọn publicly fetchable, reviewed upstream commit chứa fix và pin qua tracked Crush recipe. Không tạo private/permanent fork, không local replace, không sửa module cache. Có thể author/test patch offline trước khi được phép publish upstream.
6. **Sửa nested findings thay vì xin miễn trừ mặc định.** Reproduce trước; fix tối thiểu bằng tracked patch. Chỉ chủ repo mới có thể chấp nhận ngoại lệ có scope/evidence cụ thể nếu không sửa được.
7. **Cache: NO-ROLLOUT cho milestone này.** Không triển khai thêm prompt-cache-key experiment. Baseline thật vẫn bắt buộc để hiểu latency; không có claim cải thiện performance. A/B experiment là workstream sau, chỉ mở lại bằng yêu cầu riêng.
8. **Windows-only, isolated profiles.** Dùng Windows runner/toolchain sẵn có được phép dùng; không cài compiler hoặc đổi global configuration để vượt blocker. Không dùng profile, DB, credentials dump hoặc app instance thật của chủ repo làm fixture.
9. **Một người/agent tích hợp.** Không cho nhiều agent cùng ghi một working tree. Có thể review/author offline trong checkout riêng; tích hợp tuần tự, contract-first, mỗi WP có evidence và rollback.

## 4. Scope và giới hạn quyền

In scope: IP-01 đến IP-08 còn thiếu; CI/code hygiene; existing PR1/PR2/PR4 regression; Fantasy PR5; Windows race/ACL/portable acceptance; Hermes packaged-app smoke; live baseline; consolidated evidence và release-readiness decision.

Out of scope: Linux/macOS/WSL acceptance; hybrid/local compaction algorithm; đổi summary role/threshold; cache rollout; tự tối ưu skill/model/tool rebuild chưa có số đo; viết lại Office/Zalo/terminal/memory/skills/recall; website hoặc phục hồi WebPlan; đổi branch protection/hooks; tự chọn LICENSE thay chủ repo; tự publish release.

Các thao tác external phải có authorization riêng, không suy ra từ plan:
- Publish upstream PR/pin hoặc push/tag/release nếu yêu cầu thực thi chưa cho phép.
- Live provider calls: đúng endpoint/model/account và request/cost cap đã được chủ repo duyệt. Không tự đổi model hay xin token trong chat. Existing credentials được dùng qua cấu hình bảo mật hiện hữu.
- Cài compiler, bật service hoặc đổi môi trường toàn máy.

Không cần hỏi lại về lựa chọn kỹ thuật đã khóa ở mục 3. Với quyền/budget còn thiếu, chỉ hỏi đúng quyền thiếu và tiếp tục các WP offline độc lập. Không đổi BLOCKED thành PASS.

## 5. Dependency graph và phân chia công việc

```text
WP0 baseline/readiness -> WP1 CI/hygiene
WP1 -> WP2 provider telemetry ----+-> WP6 Windows acceptance -> WP7 live/Hermes -> WP8 baseline
WP1 -> WP3 snapshot lifecycle ---+
WP1 -> WP4 Fantasy integration --+
WP1 -> WP5 nested upstream fixes-+
WP2..WP8 -> WP9 consolidated candidate proof, docs, release-readiness
```

WP2/WP4 có thể cùng cần Fantasy upstream pin: author các thay đổi độc lập, review cùng dependency candidate và chỉ chạy final acceptance khi binary chứa cả hai. Không thay pin hai lần chỉ để chia checklist. Thiếu upstream quyền không chặn WP1/WP3/WP5 offline.

### WP0 — Revalidate và dựng nơi làm việc an toàn

Deliverables:
- Ghi HEAD, parent diff, `.tack-pin`, patch manifest/hashes, dependency graph và toolchain thực dùng. Bảo toàn owner edits.
- Xác nhận root/data/profile/pipe/single-instance isolation và đường dẫn toolchain trước khi chạy app.
- Chọn checkout/temp build duy nhất thuộc lượt chạy này; không dùng ignored engine của chủ repo.
- Lập permission ledger: upstream, live acceptance, baseline budget, compiler/runner. Trạng thái ban đầu là chưa xác minh cho lượt implementation mới.

Gate: baseline và phạm vi viết rõ, existing work không bị ghi đè; read-only readiness không phát sinh live traffic/download toolchain ngầm.

### WP1 — CI/hygiene, không đổi hành vi sản phẩm

Files: hai unused symbols nêu ở baseline; các file gofmt; owning tests và CI diagnostics.
- Xóa test hook thực sự thừa hoặc dùng trong regression có ý nghĩa; không thêm dummy reference để né Staticcheck.
- Chọn một owner cho purpose validation khi WP2 sửa contract; không giữ `validPurposes` chết chỉ vì export API dự kiến.
- Format tracked Go files. Chạy tests/vet/staticcheck/deadcode; phân biệt hàm thực sự chết với integration còn thiếu của WP2/WP3, không xóa API cần thiết để lấp gap.
- Kiểm tra generated events và binding drift. Không sửa generated output bằng tay.

Gate: không còn U1000/gofmt issues của baseline; full CI hygiene được chạy, không còn skip do failure trước đó. Rerun sau WP2/WP3 nếu wiring thay đổi.

### WP2 — IP-01: provider telemetry end-to-end

Owners: Fantasy serializer/transport/decoder seams; tracked Crush patches (`internal/agent`, provider call boundaries, RunTrace/event types); host `internal/crushapi`, `internal/runmetrics`, `internal/engineobserver`, `bind_engine.go`; owning REST/SSE contract.

Implementation:
- Loại bỏ phép đo host workspace-stream bị gắn sai tên provider span. Không giữ tên cũ cho metric khác nghĩa. Giữ compatibility cho consumer cũ bằng fields additive/omittable.
- Instrument actual request encode/write và response receive/complete-SSE-frame. `GotFirstResponseByte` không phải first decoded frame; không dùng token/semantic callback giả làm transport event.
- Truyền run -> model-call -> HTTP-attempt identity xuyên title, main/tool steps, summarize, retries/auth retries. Mỗi observation chỉ thuộc một attempt; scope counters và request-purpose contract rõ ràng.
- First semantic reasoning/tool/nonempty-text có offset riêng. Không đo được thì absent; số 0 hợp lệ chỉ khi thực sự đo và làm tròn. Không fallback wall-clock giả, không dùng timestamp stream cũ cho run mới.
- Một root run có đúng terminal event kể cả chuẩn bị thất bại, queued cancel và requeue/compaction. Cleanup mọi outcome; reconnect không dùng observer của client cũ, không data race ngoài connection ownership.
- Không cộng parent/child overlapping spans hoặc usage retries hai lần. HTTP TTFB không được quảng bá thành provider-queue-only; engine text-available không được gọi UI-visible.
- Fingerprint sau mọi transformations/options/tools; loại secrets/opaque reasoning khỏi HMAC projection. Giữ cache presence tri-state và sanitized/bounded sinks.

Proof bắt buộc:
- Negative tests tách HTTP headers, first byte, partial frame, complete SSE; EOF/5xx before frame; multi-frame reads; cancellation và missing observations.
- Concurrent runs, reused long-lived workspace stream, nhiều calls trong một root run và nhiều attempts không lẫn timestamp/identity.
- Controlled-delay fixture đi qua actual provider HTTP -> real engine -> REST/SSE -> host merge/validator/JSONL sink. Không chỉ test Registry trực tiếp; harness chỉ đọc terminal payload không tự chứng minh host integration.
- Title/tool/summarize/retry/prep-error/queued-cancel matrix; verify no fabricated zero, no double counting, no sensitive output in every diagnostic sink.
- Preserve remaining PR0 contract: final transformed request shape, generation reasons/precedence, per-kind timings và bounded writer.

Gate: fail-before/pass-after evidence từ đúng candidate executable; required gate phải gọi populated-span assertion. Comment/helper signature hoặc default fixture không có span không đủ đóng WP2.

### WP3 — IP-02: snapshot integrity và lifecycle wiring

Owners: `internal/contextseed/{seed,snapshot,snapshot_identity}.go`, regression tests; `context_seed.go`; workspace/run/connection lifecycle callsites xác minh bằng reference trace; owning context contract.

Implementation:
- Reuse một manifest validator cho staged, existing committed và concurrent-winner adoption paths: exact allowed file set, canonical paths, content digests và supported link/path semantics. `IsDir` không phải integrity check.
- Invalid committed generation: fail closed, giữ generation hợp lệ đang phục vụ; không âm thầm adopt, không overwrite/delete directory đang có reader. Không regenerate identity key để che corruption.
- Tạo explicit lease token với generation; acquire atomically với việc chọn generation để tránh khoảng trống select-then-prune. Release token idempotent; không overwrite lease của reader khác chỉ vì trùng workspace/run key.
- Workspace đã đăng ký config giữ lease cho generation cho đến khi engine xác nhận refresh/replacement hoặc teardown. Run giữ generation immutable nó thực sự dùng cho đến terminal/cancel cleanup. Nếu engine eager-load toàn bộ bytes, chứng minh boundary đó bằng code/test và vẫn giữ workspace config references cho lazy load/restart.
- Refresh build -> validate -> pin new generation -> set config/engine refresh -> commit active reference; chỉ release old reference sau acknowledgement. Failure phải giữ prior acknowledged snapshot/config; rollback/reconcile không xóa edit mới của người dùng.
- Multi-process acceptance: chọn cross-process generation leases có OS lifetime lock trong metadata ngoài prompt tree, kèm registry/prune lock; crashed holder nhả OS lock, không dựa vào TTL/PID đơn thuần. New reader registration và prune kiểm tra/xóa cùng exclusion boundary. Không giữ registry chỉ trong một Seeder rồi tuyên bố bảo vệ nhiều process.
- Retention = active leases + current acknowledged generations + bounded previous recovery floor. Prune khi không còn holder; không tắt prune, không xóa key, không đưa lease/backup/staging metadata vào prompt.
- Giữ HMAC identity domain/layout compatibility, fail-closed key publication và existing memory sanitization policy; không expand read scope hoặc đổi symlink/junction policy ngoài accepted contract.

Proof:
- Tamper committed generation rồi request cùng logical content: modified/missing/extra file bị từ chối; test cả reuse và rename-loser adoption, không chỉ staging hooks.
- Per-file read error, simultaneous publishers, two Seeder instances/processes, acquire/prune race, refresh failure và engine config failure.
- Workspace A/run chậm giữ gen1 qua gen2/gen3/gen4 do B refresh; gen1 không bị prune; sau release/crash recovery thì prune được, bounded retention không leak.
- Same bytes sau restart giữ identity; same-size edit đổi identity; key corruption không overwrite; profile path aliases không tạo holder giả hoặc bỏ sót protection.

Gate: production reference trace có actual acquire/release lifecycle; tests qua host/engine config boundary và Windows multi-process proof, không chỉ direct Seeder unit calls.

### WP4 — IP-03: Fantasy fix đi vào released dependency

- Author/review transport hooks WP2 và reasoning converter fixes trong isolated checkout từ version/pin đã ghi. Full patch phải có new files/tests, UTF-8/LF và provenance.
- Khi được phép, submit upstream và pin publicly fetchable reviewed upstream commit chứa các fixes qua tracked Crush dependency patch. Dùng exact module version/checksum; không author-only artifact làm release input.
- Replay theo `.tack-pin` -> compatibility patches -> hardening -> input_pipeline patches, đúng manifest order. Verify module graph VÀ binary build info/hash cùng chứa accepted Fantasy revision, không `replace`.
- Preserve ordered reasoning parts từ stream start/delta/end qua message DB đến replay: nhiều encrypted-only items xen tool calls/results, deep-copy metadata, retry bỏ failed partial attempts nhưng không re-execute tool side effects.
- Wire `{type,id,encrypted_content,summary:[]}` đúng SDK schema; không convert summary/thinking thành assistant text; duplicate IDs fail trước network.
- Compatibility fingerprint gồm endpoint/API mode/model và opaque account scope, không token. Switch/legacy ambiguous metadata phải explicit incompatible outcome; không silently drop reasoning cần cho cùng-provider continuity.
- Local replay `store=false`; reject incompatible `previous_response_id`/store configuration trước dispatch, không overwrite user config để che lỗi.
- Giữ latest COMPLETE anchor group + associated call/results, committed summary pointer recovery và token/accounting contract. Không đổi compaction algorithm/threshold/summary role. Rerun Anthropic/Google signature regressions.

Gate: full Fantasy suite + focused/full nested tests liên quan + actual streamed multi-item replay E2E qua restart/retry/switch/compaction; canary safe tại mọi sink; manifest release eligibility phải có evidence của pin/binary, không chỉ sửa boolean. Live acceptance còn WP7.

### WP5 — IP-06: bounded upstream defect fixes

- Reproduce `internal/fsext/TestGlobWithDoubleStar` và `internal/csync/maps.go` lock-by-value trên pristine pin và clean patched candidate, với exact OS/toolchain.
- Sửa nguyên nhân đúng trong tracked patch. Không đổi test expectation/assertion, xóa test, ignore vet hoặc viện lý do pre-existing để bỏ gate.
- Rebase recipe ở checkout thứ hai rồi chạy focused + owning package/full nested checks. Tách known upstream finding với regression mới.

Gate: pass trên candidate, hoặc release vẫn BLOCKED cho đến owner exception minh thị nêu residual risk/scope. Mặc định của plan là sửa, không miễn trừ.

### WP6 — IP-05: Windows runtime và candidate portable

- Chọn approved Windows runner có CGO-compatible C compiler; xác minh toolchain thực dùng trong repo. Cài/enable gì chưa được phép thì BLOCKED_ENVIRONMENT, không thay global env.
- Chạy race trên parent và nested scope liên quan. NTFS thật: aliases/casing/separators/relative-absolute/overlapping roots cho cả rendered bytes và snapshot ownership.
- Chứng minh key/lease/data-directory ACL với principal/access checks; simultaneous startup, creation failure/crash, named-pipe isolation và no orphan engine sau exit. Không dùng `0600` như proof Windows privacy.
- Build release-equivalent candidate từ isolated clean checkout: Wails host + verified clean-pin engine + guard/memory/skills/recall + OfficeCLI/Python/uv/libraries + bundled skills/context/legacy stock bases.
- Không chạy `scripts/build.ps1` trong owner checkout: script hiện gọi `update-crush.ps1` và có thể đụng ignored engine. Dùng isolated checkout với native Wails/resource entrypoints và receipt-verified engine; scripts downloads cần tuân quyền môi trường.
- CI portable artifact hiện chỉ chứng minh lane đó; kiểm tra actual file inventory/runtime imports/context trong gói sẽ giao, không suy ra đầy đủ từ tên ZIP.
- Dùng app instance/profile/pipe/single-instance namespace cách ly. New/stock/modified/unknown legacy; preview/cancel/accept/restart/rollback; edit-after-preview/accept; stock-base three-way merge; kill giữa staging/commit; failed refresh giữ last valid context. Capture through UI và provider fixture để policy/user content xuất hiện đúng một lần.
- Regression core workflows: startup/shutdown/reconnect, foreground chat/tool/permission/cancel, workspace/session switch, Office runtime discovery; Zalo không gọi account thật khi chưa có phạm vi test được phép.

Gate: app-level proof từ chính candidate, safe artifacts và cleanup chỉ process/temp thuộc lượt chạy. Không dùng profile chủ repo; không thay runtime proof bằng cross-compile hoặc engine-only tests.

### WP7 — IP-04 và Hermes credentialed smoke

Precondition: WP2-WP6 candidate đủ điều kiện, actual provider configuration và cap đã được duyệt. Không tự gửi request trả phí khi thiếu cap.
- Run live Responses synthetic suite đúng endpoint/model/account: encrypted-only multi-item continuity, tool loop, DB restart, compaction continuation, incompatible config rejected trước network. Fake ciphertext không thay live acceptance.
- Run Hermes trên chính packaged candidate: accepted foreground turn -> due memory/skill review theo cadence hiện có -> guard-enforced allowlist/tool action -> detached-session cleanup -> learned skill discovery -> bounded recall.
- Kiểm tra live-turn cancellation của review, không có second agent loop, không dùng dữ liệu thật làm training fixture; respect existing 10/10 cadence và 16-iteration bound.
- Evidence chỉ giữ sanitized verdict/count/timings/opaque test labels, cấu hình không bí mật và artifact hash; không logs/profile dump, không raw session IDs/token/ciphertext.
- Dừng trước khi vượt bất kỳ request/cost cap; exhausted budget là BLOCKED/inconclusive, không PASS. Nếu provider không expose chi phí tin cậy, không tự ước lượng rồi vượt cap; dùng quota có thể cưỡng chế đã được duyệt.

Gate: real provider acceptance và Hermes interaction path có observable proof. Tài khoản đã có không miễn trừ permission/budget. Historical Hermes implementation được giữ, chỉ bổ sung final acceptance.

### WP8 — IP-07: real baseline, cache giữ OFF

- Bổ sung explicit live-baseline mode/entrypoint fail-closed nếu chưa có. `scripts/bench-input-pipeline.ps1` hiện synthetic-only và workload aggregation là fresh; KHÔNG chạy nó rồi gọi kết quả là live/năm-workload baseline.
- Live entrypoint phải tách khỏi default fake mode, validate exact allowlisted endpoint/model/account, approved budget, candidate receipt và output-redaction trước network. Thiếu input/zero captures phải fail, không tự fallback provider hay giả lập latency.
- Đăng ký trước workload fixtures, warm-up, seed, concurrency, sample counts và missing/censoring policy. Thu đủ fresh, warm turns 2-10, 30-turn, near-compaction, synthetic-MCP/large-history qua real engine và approved real provider.
- Không cố định một sample count tùy ý rồi gọi đủ precision; plan chạy cụ thể phải vừa budget vừa mục tiêu mô tả baseline. Không có A/B claim trong milestone này.
- Report exclusive local prep, HTTP TTFB, first reasoning/tool/nonempty-text, full root turn, summarize; cache hit/miss/unreported; errors/retries/timeouts/tool-only/cancel/missing-text. Không loại outlier/lỗi âm thầm, không giả cold provider cache.
- Decision cố định `no-rollout`, default cache OFF; preserve user-configured key nếu có, không xóa config. Đóng optional PR3 bằng quyết định không rollout, không cần code experiment.

Nếu owner mở riêng experiment sau milestone: giữ gate gốc AB/BA >=30 independent session-pairs/workload, nearest-rank, seeded cluster bootstrap >=10000, warm text TTFT p50 >=10% improvement với CI95% >0, p95/full-turn degradation <=5% với đủ precision, không tăng error/retry hoặc history contamination. Không dùng baseline đơn lẻ để mở gate này.

### WP9 — IP-08: consolidated acceptance và handback

- Freeze integrated source/recipe candidate theo quy trình commit được phép. Gate clean-pin yêu cầu committed inputs; nếu chưa được phép commit thì báo NEEDS_COMMITTED_CANDIDATE, không lách verifier.
- Chạy đủ validation ở mục 6 trên candidate cuối, không cộng các PASS từ nhiều binary/revision khác nhau thành release PASS.
- Assert tất cả 14 required tests hiện có RUN/PASS; thêm các regression mới WP2-WP4 vào gate-owned required inventory hoặc assertions của test bắt buộc. Không drop test để giữ số 14; không unexpected skip.
- Test gate negative controls vẫn reject malformed JSON, build-fail, skipped/missing/duplicate required tests, wrong commit/recipe/hash/dependency, synthetic-as-live và leaked diagnostic canaries. Gate xanh có giới hạn coverage, không tự chứng nhận external pin/live/race.
- Rerun all-repo reference/unused/format checks; reconcile docs/contracts/ADR với final implemented behavior. Đặc biệt sửa mô tả host-SSE timing, purpose/attempt, refcount, summary:[] schema và feature-flag prose không được biến PR5 correctness thành optional.
- Historical reports giữ dấu vết, thêm correction rõ thay vì sửa lịch sử thành PASS. Active indexes trỏ một plan; không tái tạo `ImplementPlan.md` root.
- Actual CI và Windows gate xanh trên final source HEAD; record run URLs. Branch protection kiểm tra riêng nếu cần claim merge-blocking, không tự sửa settings.
- Final release-equivalent ZIP: versions/tag intent consistent, runtime inventory, hashes, last smoke. Return READY_FOR_OWNER_RELEASE chỉ khi mandatory gates PASS. Push/tag/publish cần quyền riêng; chọn LICENSE và screenshot README là owner/polish follow-up, không tự bịa legal decision.

## 6. Validation commands và cách chạy

Các lệnh dưới đây dùng PowerShell 7 từ checkout đã được xác minh. Trước mỗi lệnh phải xác minh dependencies/toolchain có sẵn; không tự cài compiler hoặc gọi live. Mỗi native command phải kiểm tra `$LASTEXITCODE`, không chỉ dựa vào `$ErrorActionPreference`. Không chạy các lệnh ghi generated files trên owner edits chưa bảo toàn.

Parent scope (Gotack module):

```powershell
go test -count=1 ./...
go vet ./...
staticcheck ./...
deadcode -test ./...
gofmt -l (git ls-files '*.go')
node scripts/check-repository-invariants.mjs
node --test scripts/input-pipeline/gate.test.mjs scripts/input-pipeline/benchmark.test.mjs
pnpm --dir frontend check
pnpm --dir frontend test
pnpm --dir frontend build
```

`deadcode` output phải rỗng, không chỉ exit 0. gofmt output phải rỗng. Tool versions theo CI/module pin, không tự `@latest`. Generated drift: dùng native event generator và `wails generate module` trong isolated checkout; compare tracked output theo contract, không chỉ generation exit status.

```powershell
go run ./internal/uievents/gen/main.go
git diff --exit-code -- frontend/src/platform/events.generated.ts
wails generate module
# Inspect generated binding changes against owning contract and tracked baseline.
./scripts/test-input-pipeline-e2e.ps1
```

E2E script tự clean-pin replay vào temp, không dùng owner ignored engine. Hai bước BuildOnly/SkipBuild chỉ dùng absolute binary/provenance paths do build thành công trả về, không PATH fallback. Receipt phải khớp current commit, recipe và binary. Không dùng build-only success làm E2E PASS.

Windows race: chỉ trong process có approved compiler; bật CGO riêng process, lưu/restore environment sau khi chạy. Chạy `go test -race -count=1 ./...` ở parent và đúng nested packages/modules trong isolated candidate. Parent `./...` không bao gồm toàn module Crush/Fantasy.

Nested scope: trong clean replay checkout, chạy focused reproduction `go test -count=1 ./internal/fsext` và `go vet ./internal/csync`, rồi full `go test -count=1 ./...`, `go vet ./...` cùng relevant static/race checks. Trong isolated pinned Fantasy checkout, chạy full suite và OpenAI/other-provider regression suites. Record mọi platform/dependency limitation, không thay full-suite gate bằng focused PASS.

Synthetic benchmark sanity (không phải live baseline): dùng `./scripts/bench-input-pipeline.ps1 -Pairs 3 -Seed 42 -EngineBinary '<verified absolute path>' -Provenance '<matching absolute path>'`. Paths ở đây là placeholders, không paste nguyên. Live runner CLI chưa có ở baseline: WP8 phải implement/document/test trước, không invent command đã tồn tại.

Các test/server/app process lâu phải có timeout, cancellation và owner-scoped cleanup. Không để shell foreground chạy vô hạn; poll finite logs đã redacted. Không ghi raw provider/test output chứa canary/secrets vào published artifacts.

## 7. Evidence contract

Mỗi WP ghi một entry gồm:
- Source HEAD + candidate changes/commit; OS/toolchain; exact commands, exit codes và timestamps.
- Relevant pin, ordered patch/manifest/hardening hashes, module version/checksum, binary build info và SHA-256; portable ZIP hash nếu có.
- Status: PASS / FAIL / BLOCKED / NOT_RUN. Negative-control expected failure phải ghi EXPECTED_FAIL với đúng invariant diagnostic; không gom vào success count.
- Test names/scenarios, actual RUN/PASS inventory, unexpected skips, coverage limits; app/provider observations tách khỏi unit tests.
- Artifact safe path/CI URL, privacy verdict, permissions/budget usage theo cap đã duyệt; không raw prompts/ciphertext/token/session IDs.
- Remaining blocker, next exact action, rollback và resources đã cleanup.

| WP | Candidate/commands/artifacts | Status ban đầu | Next action |
| --- | --- | --- | --- |
| WP0 | Baseline source/CI ở mục 2 | NOT_RUN cho implementation mới | Revalidate và permission ledger |
| WP1 | Known CI U1000 + gofmt inventory | OPEN | Fix và rerun hygiene |
| WP2 | Source-level telemetry gaps | OPEN | Provider-boundary implementation và populated-path E2E |
| WP3 | Existing-generation validation + unused reader API | OPEN | Validate reuse/adopt và wire lifecycle |
| WP4 | Fantasy manifest not-submitted/release-ineligible | BLOCKED_EXTERNAL_PIN | Offline author/review; xin đúng quyền upstream còn thiếu |
| WP5 | Historical fsext/csync findings | OPEN | Reproduce trên candidate, tracked fix |
| WP6 | CGO/compiler chưa có proof | BLOCKED_ENVIRONMENT | Approved Windows runner/toolchain và isolated portable |
| WP7 | Provider có sẵn, chưa có cap cho lượt mới | BLOCKED_LIVE_AUTH | Approved endpoint/model/account/caps rồi chạy |
| WP8 | Runner hiện synthetic-only | OPEN; live execution blocked budget | Implement explicit live baseline; no-rollout |
| WP9 | Tổng gate chưa đóng | OPEN | Consolidate sau WP2-WP8 |

## 8. Progress

- [x] WP0: baseline/readiness/permission ledger đã cập nhật (local execution dưới đây).
- [x] WP1: CI/hygiene đã sửa và đủ kiểm tra local; remote CI chưa chạy trên candidate.
- [ ] WP2 / IP-01: provider-wire telemetry đúng và real executable/host sink proof.
- [ ] WP3 / IP-02: committed integrity + reader lifecycle + concurrency proof.
- [ ] WP4 / IP-03: approved Fantasy revision thực sự trong candidate binary.
- [ ] WP5 / IP-06: nested findings đã xử lý với executable evidence.
- [ ] WP6 / IP-05: Windows race/NTFS/ACL/isolation/portable acceptance.
- [ ] WP7 / IP-04 + Hermes: live continuity và credentialed packaged smoke.
- [ ] WP8 / IP-07: năm workload baseline thật, cache OFF/no-rollout.
- [ ] WP9 / IP-08: consolidated final candidate proof, docs và release-readiness handback.

Không tính tỷ lệ hoàn thành bằng checkbox. Tick chỉ sau gate của WP; một implementation patch hoặc focused green không được gọi release-ready.

### Execution evidence — 2026-09-06, Windows amd64 (UTC+07)

**Result: IMPLEMENTATION_PARTIAL / NOT_READY.** Đây là execution thật, không
phải PLAN_ONLY. WP2, WP3 và WP8 còn implementation offline chưa xong; không
gọi các phần đó là chỉ chờ quyền. WP4/6/7 có blocker dependency/môi trường/
authorization riêng. Không chuyển plan sang completed.

Candidate và phạm vi:

- Owner HEAD vẫn `8511143de90d0ae13d3eb307fca692d7bf7e210d`. Khi bắt đầu có sẵn
  12 tracked document changes (kể cả xóa `ImplementPlan.md`) và plan untracked;
  đã bảo toàn. Không sửa ignored `third_party/crush`, module cache, profile,
  DB hay app instance thật. Không chạy owner `scripts/build.ps1`.
- Temp root của lượt này:
  `C:/Users/Admin/AppData/Local/Temp/gotack-release-56459ee393174823bb021c8f546481fd`.
  `host` là local clone + exact owner overlay + implementation changes;
  `crush`, `replay`, `final-replay` là isolated upstream checkouts;
  `fantasy` là isolated authoring checkout. Không dùng owner ignored engine.
- Local-only validation commit trong `host`:
  `32444f812d07fe38be318e0de5de768026867ebb`. Không push, tag, publish hay
  thay HEAD/index owner. Evidence entry này được thêm sau freeze; source/recipe
  của binary là commit nói trên, không nhận evidence prose là binary input.
- Parent Go `go1.27.0 windows/amd64`, CGO=0; staticcheck 2026.2.1 / v0.8.1,
  deadcode module v0.49.0, Wails v2.15.0. Toolchain Go đã có trong cache;
  Fantasy test dùng cached Go1.26.6. Không cài C compiler/global config.
- Permission ledger: local source/test/isolated fixture/build YES; upstream
  PR/push/pin publication NO; live endpoint/model/account/caps UNAPPROVED;
  compiler installation NO; external CI dispatch/push NO. Không chạy live
  provider acceptance hoặc baseline. Existing provider credentials không được
  lấy từ profile chủ repo để làm fixture.
- Compiler readiness: không có gcc/clang/cl trên PATH; bốn đường dẫn phổ biến
  `C:/msys64/{ucrt64,mingw64}/bin/gcc.exe`, LLVM clang và Git mingw gcc đều
  không tồn tại. Đây không phải exhaustive inventory mọi compiler trên máy.
- Graph `gotack` đã indexed nhưng thiếu symbols mới và trả snapshot source
  range cũ; đã dùng filesystem fallback theo AGENTS. Không sửa harness vì
  friction này.

| WP | Implementation / actual proof | Trạng thái còn lại |
| --- | --- | --- |
| WP0 | HEAD/diff/toolchain kiểm tra; temp clone riêng; permission ledger ở trên; executable harness tạo isolated fixtures | PASS readiness; chưa chứng nhận portable app isolation |
| WP1 | Xóa `withCollectHook`, `validPurposes` thừa; format bảy file baseline. Staticcheck trước sửa exit 1 đúng hai U1000; sau sửa tests/vet/staticcheck/deadcode exit 0, deadcode/gofmt output rỗng | PASS local hygiene; remote CI NOT_RUN |
| WP2 | Host callback không merge workspace timing vào provider span; callback giữ đúng client qua reconnect. Test `TestWorkspaceStreamCannotFabricateProviderSpanInHostSink` qua HTTP SSE → forwarder → production callback → validator/JSONL: trước sửa exit 1 đúng diagnostic, sau sửa exit 0; hai root runs trên cùng stream, engine span được giữ, session canary không vào sink | PARTIAL: serializer/transport/complete-frame instrumentation, model-call/attempt identity, populated-provider E2E và terminal matrix chưa xong. Legacy registry còn trong client; không coi tests của registry là provider proof |
| WP3 | Reuse/adopt dùng chung manifest validator với staging, reject links/nonregular/duplicate canonical paths. `TestSnapshotRejectsCorruptCommittedGeneration`: modified/missing/extra × reuse/adopt đều EXPECTED_FAIL trước sửa, PASS sau sửa; giữ prior bytes và không xóa corrupt directory. Full contextseed suite exit 0 | PARTIAL: explicit atomic generation leases, cross-process registry/prune lock, host/run lifecycle và acknowledgement/rollback chưa implement |
| WP4 | Fantasy patch apply --check/apply exit 0 ở exact base `f06034c7824ffddc4394d4cefa5ed5132a186b1b`. Package suites gồm OpenAI/Anthropic/Google pass; full suite FAIL do recorded Responses request mismatch rồi providertests timeout 180s | FAIL_OFFLINE + BLOCKED_EXTERNAL_PIN. Không đổi release_eligible; candidate binary vẫn Fantasy v0.41.3, không chứa author-only patch |
| WP5 | Pristine và patched `internal/fsext` pass; lock-by-value vet fail trước sửa ở cả hai. Thêm `csync-schema-lock.patch` với schema/JSON regression, vet pass. Full suite phát hiện glob Windows path regression; `glob-windows-paths.patch` sửa output mà không đổi test assertion; glob tests pass. Final recipe replay từ pin mới pass | OPEN: một full nested test/vet run pass nhưng rerun có 1 fail/21 skip; nested staticcheck còn 19 diagnostics. Không dùng pass trước để đóng gate |
| WP6 | Verified clean-pin engine build và existing 14-test executable gate pass trên Windows/CGO=0 | BLOCKED_ENVIRONMENT + upstream dependencies; race/ACL/portable app/UI/Hermes không được thay bằng engine-only proof |
| WP7 | Không gửi lượt live; không dùng dữ liệu thật | BLOCKED_LIVE_AUTH + candidate preconditions; endpoint/model/account/request-cost cap chưa duyệt |
| WP8 | Chặn caller đổi `synthetic:false` thành live claim trong report API; regression EXPECTED_FAIL trước sửa, PASS sau sửa. Existing synthetic benchmark 3 pairs/seed42 pass với receipt-verified binary | PARTIAL: dedicated live runner và năm workload live chưa implement/run; cache OFF/no-rollout |
| WP9 | Freeze isolated source; actual existing gate 14 RUN/PASS, zero unexpected skips; source/recipe/binary receipt khớp | OPEN / NOT_READY: mandatory WP2-WP8 chưa đóng, không có remote CI/portable ZIP acceptance |

Commands và kết quả (ngày 2026-09-06, lượt làm việc khoảng 19:10–19:35
UTC+07; đầu lượt chưa thu timestamp riêng từng unit command, không bịa
timestamp chính xác):

- Parent `go test -timeout 180s -count=1 ./...`: exit 0, sau host/snapshot fixes.
  Đây là default build tags, không tự thay executable gate.
- Parent `go vet ./...`, `staticcheck ./...`, `deadcode -test ./...`: exit 0;
  deadcode không có output. `gofmt -l (git ls-files '*.go')` output rỗng;
  new Go test files cũng đã gofmt.
- `node scripts/check-repository-invariants.mjs`: exit 0.
  `node --test scripts/input-pipeline/gate.test.mjs scripts/input-pipeline/benchmark.test.mjs`:
  exit 0, 23 tests pass, zero skip sau synthetic-as-live guard.
- `pnpm --dir frontend check`: exit 0, zero errors/warnings;
  `pnpm --dir frontend test`: exit 0, 39 tests/7 files pass;
  `pnpm --dir frontend build`: exit 0. Không đổi frontend source; đây không
  phải UI/portable workflow verification.
- Trong frozen `host`: `go run ./internal/uievents/gen/main.go`,
  `wails generate module`, `git diff --exit-code`, `git status --short`:
  generation exit 0, tracked drift rỗng và status rỗng.
- Isolated Crush pristine/patched: `go test -timeout 120s -count=1 ./internal/fsext`
  exit 0; `go vet ./internal/csync` exit 1 trước fix. Sau fix:
  `go test -timeout 120s -count=1 ./internal/csync ./internal/fsext` và
  `go vet ./internal/csync` exit 0 ở authoring checkout và second replay.
- `go test -timeout 120s -count=1 ./internal/agent/tools -run TestGlob -v`:
  exit 0 sau glob fix, gồm scoped prefix, symlink escape và result cap.
  Trước fix full suite fail `TestGlobFilesScopedPrefixMatchesUnscoped`.
- `./scripts/apply-crush-patches.ps1 -CrushDir <final-replay>` exit 0 từ clean
  pin với đầy đủ ordered recipe; full `go test -timeout 180s -count=1 ./...`
  và `go vet ./...` exit 0 một lượt. Lượt JSON inventory tiếp theo exit 1:
  2806 RUN, 2784 PASS, 21 SKIP, 1 FAIL; đang điều tra tên/lý do, không bỏ qua.
  Rerun để lấy safe test-name inventory: exit 0, 2806 RUN, 2785 PASS,
  21 SKIP. Lần fail trước chưa giữ tên test trong bộ đếm nên chưa xác định
  root cause; rerun PASS không đóng flake. Skip inventory gồm coder live
  fixture, Docker, symlink/permissions và platform/shell cases; chưa phân
  loại đủ tất cả 21 lý do, không claim zero unexpected skips cho full module.
  Nested `staticcheck ./...` exit 1: U1000, ST1005 và hai deprecated APIs;
  diagnostics gồm channelGate.isOpen, unused Question handlers sau hardening,
  createDotCrushDir, dialog/diffview helpers, MCP logging và shell ExecHandler.
- Fantasy `go test -timeout 180s -count=1 ./...`: exit 1. Cassette matcher
  diagnostic có raw recorded reasoning bytes; không lưu/upload stdout này
  vào evidence artifacts. Full-suite PASS chưa có; cần sửa fixture/serializer
  compatibility ở owning layer và kiểm tra diagnostic redaction trước rerun.
- Trong frozen `host`, `./scripts/test-input-pipeline-e2e.ps1`: exit 0.
  `provenance.json` mtime `2026-09-06T12:29:35Z`; `result.json` mtime
  `2026-09-06T12:30:04Z`, result `{status:PASS, required_tests:14, unexpected_skips:0}`.
- Synthetic command (dùng chính xác backslash paths trong receipt):
  `./scripts/bench-input-pipeline.ps1 -Pairs 3 -Seed 42 -EngineBinary 'C:\Users\Admin\AppData\Local\Temp\gotack-input-pipeline-NGuDFq\tack-engine-e2e.exe' -Provenance 'C:\Users\Admin\AppData\Local\Temp\gotack-input-pipeline-NGuDFq\provenance.json'`:
  exit 0, `BenchmarkPairedTurns`, 3 independent pairs, synthetic=true,
  no-rollout/cache OFF. Trước đó invocation dùng slash alias bị verifier
  reject `build_command_mismatch` (exit 1, chưa network); dùng lại exact
  printed paths pass, không đổi/bypass verifier.

Artifacts an toàn và provenance:

- `C:/Users/Admin/AppData/Local/Temp/gotack-input-pipeline-NGuDFq/` chứa
  `provenance.json`, `tests.jsonl`, `result.json`, `tack-engine-e2e.exe`.
  `tests.jsonl` chỉ required lifecycle/elapsed; đã đọc và xác nhận đủ 14
  RUN/PASS. Không có raw prompt, token, ciphertext hay session ID.
- Binary SHA256:
  `99d2a921cf65bdb1927d9d7b303c5d88e7a0da16322cf42fa10d6fd3e532733a`.
  Build info: Windows/amd64, CGO=0, trimpath=true;
  `charm.land/fantasy v0.41.3`, checksum
  `h1:MuzL/7iSF1M9LiB+mHQHFYL9oXyYxI5yWQxOu2PaAX4=`; no replace.
- Ordered manifest SHA256:
  `99dc387ab5351c74323fa9c5a612a7178acdc969cfb006704ae9f4ef6b1ea6d2`.
  Full pin/ordered patch/hardening/input hashes ở receipt.
- Synthetic report:
  `<temp-root>/host/tmp/bench-input-pipeline/bench-report-20260906-193054.json`;
  không gọi đây là five-workload live baseline hay performance improvement.
- Không có remote CI URL cho candidate mới; không claim branch protection.
  Không có portable ZIP/hash hay release publication.
- Enforcement: Go regressions nằm trong `go test ./...` mà CI hiện gọi.
  Workflow input-pipeline chỉ gọi `gate.test.mjs`, chưa gọi
  `benchmark.test.mjs`; synthetic-as-live regression mới hiện được chứng minh
  bằng local command, không claim CI enforcement cho test đó. Không cài hook
  hay sửa workflow/branch protection.
- Temp checkouts/build artifacts được giữ để review/recovery; không xóa
  dữ liệu owner. Engine gate và benchmark đã exit; kiểm tra process sau gate
  không thấy tack-engine-e2e/tack/gotack. Không có app thật được launch.
  Rollback implementation bằng diff chọn đúng file/hunk của lượt này;
  không reset owner tree và không rollback user data.

Next execution: hoàn thiện WP2 provider boundary + WP3 leases trước runtime
acceptance; xử lý Fantasy full-suite/cassette và nested findings; sau đó
implement live runner offline. Các phần này vẫn là công việc implementation,
không được đổi thành PASS bằng quyền upstream/budget được cấp sau này.

## 9. Risks, recovery và stop rules

- Sai layer telemetry tạo benchmark giả: gate controlled-delay/attempt correlation phải FAIL trước fix; không publish performance claim trước WP8.
- Snapshot integrity/lease lỗi có thể mất context: fail closed, giữ last acknowledged generation, không sửa/xóa user customization; rollback cần đồng bộ engine config/reference trước prune.
- Race/restart có thể làm reuse observation hoặc generation đã chết: explicit ownership, process-lifetime locks và Windows crash tests; không TTL-only lease.
- Dependency drift: build từ clean pin + ordered tracked recipe, kiểm tra binary metadata/hash; không dựa vào thư mục tmp cũ hoặc tên exe.
- Migration rollback phải qua existing transaction/CAS và conflict behavior. Không `git reset --hard`, `git clean -fdx`, rm owner profile hoặc sửa ignored engine để recovery.
- Mỗi WP nên là change set nhỏ có thể revert bằng commit được phép. Revert source không tự rollback user data; không downgrade migration/DB state ngoài contract.
- Nếu phát hiện regression mới hoặc approved gate thất bại: bổ sung reproducer, sửa đúng owning layer rồi rerun affected/downstream gates. Không giảm tests để tránh báo cáo failure.
- Chỉ stop WP có materially missing product authority, remote-write permission, budget hoặc recovery chưa an toàn; báo cụ thể và chuyển sang independent offline WP.

## 10. Definition of Done và Result

DONE chỉ khi: WP1-WP9 mandatory acceptance đều PASS trên integrated candidate; no unexpected skips; release uses accepted Fantasy fix; Windows race/ACL/runtime proof có thật; live continuity/Hermes và năm-workload baseline có evidence; cache vẫn OFF; docs nhất quán; không mất/rò user data; final source/recipe/binary/ZIP provenance đầy đủ.

Result hiện tại: IMPLEMENTATION_PARTIAL / NOT_READY. Evidence execution ngày
2026-09-06 ở mục 8 thay trạng thái PLAN_ONLY ban đầu. Existing engine gate
14/14 đã pass trên isolated candidate, nhưng mandatory acceptance của plan
chưa hoàn tất. Root `ImplementPlan.md` được thay bằng plan này, không phải
chứng nhận backlog cũ đã hoàn tất.

Handback của AI thực thi phải nêu: source candidate, từng WP status, tests thực chạy, artifact/CI URLs, blocker còn lại, cleanup, rollback và READY_FOR_OWNER_RELEASE hoặc NOT_READY. Không viết “đang chạy nền” khi không có process còn chạy; không tuyên bố đã phát hành nếu chỉ build local.
