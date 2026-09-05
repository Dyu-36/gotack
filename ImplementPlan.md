# ImplementPlan — Gotack input pipeline upgrade

Date: 2026-09-04; revalidated: **2026-09-05 (Windows)**.
Status: **Reviewed with blocking findings — đủ làm hướng dẫn triển khai theo phase/gate, CHƯA đủ điều kiện release hoặc tuyên bố cải thiện performance.**
Product scope (chủ repo xác nhận 2026-09-05): **Windows-only**. Không yêu cầu Linux build, Linux runner/WSL hoặc Linux case-sensitive filesystem tests cho milestone này. Windows race, NTFS/path, named-pipe, ACL và portable-app tests vẫn là các gate liên quan. Chủ repo đã có provider thật; đây không phải blocker thiếu provider, nhưng live acceptance/performance vẫn phải chạy và có evidence trước khi đưa ra claim.
Authority: yêu cầu chủ repo ngày 2026-09-05 cho phép kiểm chứng/sửa Plan; `AGENTS.md`, `docs/WORKFLOW.md` và các contract đã đối chiếu với code. `Upgrade.md` không tồn tại tại thời điểm audit, nên không dùng làm authority đã xác minh; nếu có lại phải đối chiếu trước khi đổi product scope.
Baseline: Gotack `d07c8929b8737fedcc7e2584e78f6e9c7d662bc3`, Crush pin `6d14dd93a9e526505f7de54ae5999431bc32a793`, **kèm working tree đang dirty**. Phân biệt pin sạch, bốn compatibility patches + hardening, và code thử nghiệm trong `tmp/crush-input-pipeline` / `tmp/fantasy-input-pipeline`.

Bản review này chỉ sửa Plan, không sửa implementation, không commit và không chứng nhận các checkbox trong execution plan là hoàn tất. Mục 0 là điều kiện thực thi bắt buộc; mục 10 ghi bằng chứng, lệnh tái hiện và phần chưa kiểm chứng. Các mục 1–9 mô tả target, không phải tính năng đã được chứng minh. Không có cam kết tuyệt đối “không lỗi”: release phải dựa vào positive/negative tests, recovery và provider gate.

## Kết quả bắt buộc

Triển khai một pipeline prompt/request có tính xác định, không làm mất cấu hình hoặc reasoning state, có telemetry đủ để chứng minh nguyên nhân latency, và có E2E black-box qua executable/REST/SSE/provider giả. Kế hoạch này là quyết định cuối cùng; coder không tự chọn lại phương án kiến trúc.

## Quyết định cuối cùng của Leader

1. Làm **PR0 observability trước mọi thay đổi performance**.
2. Duyệt bounded Crush patches cho ordering, provider-option merge và todo correctness.
3. Tách prompt thành **stable prefix đứng trước** và **dynamic suffix đứng sau**; không đưa turn loop sang desktop.
4. `skills.Manager` là nguồn dữ liệu skill duy nhất; prompt renderer không tự scan lần thứ hai.
5. Windows được coi là filesystem **case-insensitive** trong contract v1; thư mục Windows bật per-directory case sensitivity chưa được hỗ trợ.
6. `prompt_cache_key` chỉ là experiment sau benchmark gate; mặc định vẫn tắt.
7. Reasoning continuity là **release requirement P0** cho OpenAI Responses; hỗ trợ đầy đủ nhiều reasoning item, không chỉ item cuối.
8. Dùng explicit local-history replay với `store=false`; không dùng đồng thời `previous_response_id`.
9. Hoãn hybrid compaction code; chỉ được bắt đầu sau baseline, contract và invariant tests.

## Ranh giới không được vi phạm

- Không import `third_party/crush/internal/...`; Gotack chỉ giao tiếp qua REST + SSE.
- Không sửa checkout ignored đang dùng của người khác. Có thể phát triển trong checkout cách ly; mọi thay đổi engine cuối cùng phải thành patch tracked có phase/provenance, replay được từ pin sạch theo mục 0.1.
- Fantasy reasoning/transport mapping phải upstream trước. Nếu release bị chặn, pin tạm đúng commit chứa upstream PR qua Crush patch; không tạo fork vô thời hạn.
- Không log prompt, ciphertext reasoning, token OAuth, authorization header, raw tool output hoặc raw session UUID.
- Contract wire thay đổi thì cập nhật `docs/contracts/crush-rest-sse.md` cùng PR.
- Không tối ưu skill scan/model rebuild/tool rebuild khi telemetry chưa chứng minh chúng đáng kể.

## Thứ tự giao việc theo phase và gate

Giữ tên PR0–PR5 để đối chiếu scope bên dưới, nhưng **không giao một lượt A–Z**. Mỗi phase có đầu ra và review riêng; PR3 là experiment tùy chọn, được xếp sau các hạng mục correctness/data safety bắt buộc.

1. **Phase 0 — Harness + PR0 observability:** khóa baseline/provenance trong checkout cách ly; sửa replay/build/E2E chống pass giả và trace wire thật. Không tối ưu performance, không sửa migration hoặc reasoning ở phase này. Sau fake-provider gate và kiểm soát secrets/cost, chạy baseline thật trên năm workload; lưu evidence redacted. Có thể tách 0A harness/provenance và 0B telemetry/baseline thành hai checkpoint để diff dễ review.
2. **Phase 1 — PR1 bounded correctness:** ordering, canonical rendered paths, option merge/validation và todo state. Đưa regression cases tương ứng vào owning package và chứng minh fail-before/pass-after.
3. **Phase 2 — PR2 prompt architecture:** stable/dynamic split, immutable snapshots và một skill builder. Chạy lại baseline/benchmark để kiểm tra ảnh hưởng, chưa bật cache experiment.
4. **Phase 3 — PR4 TACK ownership/migration:** transaction, crash recovery, durable rollback và Windows UI/portable flow. Không chạy migration thử trên profile/dữ liệu thật của user.
5. **Phase 4 — PR5 reasoning continuity:** durable patch/upstream pin, ordered replay qua DB/restart/tool loop và bounded compaction anchor contract. Live acceptance đúng endpoint/model là gate bắt buộc trước release claim.
6. **Phase 5 — PR3 cache experiment (conditional):** chỉ triển khai khi telemetry chỉ ra cơ hội đáng đo; benchmark paired với candidate đã tích hợp các phase trước. Nếu PR4/PR5 đổi request/prefix thì số đo cũ không mở gate cho candidate mới. Không có lợi ích đủ bằng chứng thì giữ off và đóng phase bằng quyết định không rollout, không ép bật để đủ checklist.
7. **Release gate — Windows:** replay toàn bộ patch từ pin sạch, build đúng candidate, repository/nested/focused tests, race, fake E2E, named-pipe/ACL/portable migration và live Responses acceptance tương ứng đều có evidence. Hybrid compaction vẫn là workstream riêng, blocked theo mục 9; không nhân tiện implement.

### Cách điều phối agent

- Một agent triển khai chính chịu trách nhiệm tích hợp; giao **một phase/checkpoint mỗi lần**, dừng bàn giao sau gate. Không cho nhiều agent cùng ghi một working tree. Nếu dùng agent khác cho phase kế tiếp, bàn giao bằng Plan + provenance + evidence, không dựa vào trí nhớ chat.
- Một reviewer độc lập, mặc định read-only, đọc diff và tự chạy các lệnh/negative controls liên quan trong checkout cách ly. Reviewer không vừa sửa code vừa tự duyệt cùng thay đổi; feedback quay lại agent triển khai. Có thể review/thiết kế fixtures song song; coding song song chỉ khi đã khóa contract, có ownership file rõ và checkout riêng. Không chạy nhiều bộ test live tốn phí song song ngoài budget đã duyệt.
- Kết thúc mỗi phase báo: scope đã làm/chưa làm, file/patch thay đổi, exact commands + exit/status PASS/FAIL/BLOCKED/SKIP, artifact/provenance, negative-test proof, rollback và rủi ro còn lại. Không gọi BLOCKED hoặc unexpected SKIP là PASS; không giảm assertion, xóa test hoặc đổi gate để làm xanh.
- Chỉ nhận phase khi reviewer xác nhận acceptance trong phạm vi đó; completion của một phase không phải release approval. Các lỗi migration/reasoning chưa xử lý có thể tồn tại ở phase trước nhưng phải ghi còn mở và không thử trên dữ liệu thật. Nếu gate cùng phase bị chặn bởi môi trường, dừng tại checkpoint và báo đúng hỗ trợ cần thiết.
- Phase 0 phải kiểm tra readiness Windows race toolchain và live provider config/budget sớm. Chủ repo đã có provider thật, không yêu cầu mua/kết nối lại hoặc dán token vào chat. Không tự đổi endpoint/model, phát sinh chi phí chưa duyệt, cài compiler hay thay Windows service.

## 0. Gate nền tảng và các quyết định được khóa sau audit

### 0.1. Provenance, patch và thứ tự thực thi

- **P0 trước hết:** sửa harness chống pass giả và hoàn thiện producer PR0; optional struct/JSONL consumer không đủ để đóng PR0. Phần PR4 đã có trong working tree vẫn phải coi là chưa an toàn cho dữ liệu thật.
- Chỉ tiếp tục benchmark/performance sau khi trace wire thật và E2E thật pass. PR1 correctness không được giấu sau cache flag; chỉ PR3 cache experiment mặc định off. PR5 là release gate bắt buộc cho phạm vi Responses đã công bố.
- Không reset/clean checkout đang dirty, không tự sinh patch từ cây người khác đang sửa. Dùng checkout cách ly, ghi pin, ordered patch SHA256, hardening-script SHA256, Fantasy version/commit và binary SHA256. Không suy ra nguồn binary từ tên file/PATH.
- Bốn patch hiện có + hardening tạo **base đã harden**. Code ở `tmp/crush-input-pipeline` dựa trên base này; incremental patch phải có phase **sau hardening**, không chỉ thêm prefix `zz-` vào vòng glob vốn chạy trước hardening. Khi implement, sửa replay script để tách rõ compatibility patches → hardening → input-pipeline patches; record phase trong provenance. Một cách generate khác phải chứng minh tương đương từ pin sạch, không áp trùng bốn patch cũ.
- Diff phải bao gồm new files/tests: stage đúng tập file được duyệt trong checkout cách ly trước `git diff --cached --binary <verified-base>`. Không dùng plain `git diff` rồi bỏ sót untracked tests. Xuất patch UTF-8/LF; không dùng PowerShell 5.1 `>` tạo UTF-16. Replay từ checkout thứ hai phải không phụ thuộc temp commit chỉ tồn tại trên máy này.
- Local `replace charm.land/fantasy => ./third_party/fantasy` chỉ dùng thử nghiệm, **cấm trong release**. Phải có upstream commit truy cập được, version/go.sum chính xác, không sửa Go module cache. Nếu upstream/pin chưa sẵn sàng, PR5 blocked chứ không coi local tests là upstream approval.
- `$SkipInputPipeline` đang được khai báo nhưng chưa được dùng: phải implement và có positive/negative proof hoặc bỏ tham số; không chấp nhận flag no-op. Không sửa CI/branch protection trong đợt audit Plan này; implementation phải có PR riêng chứng minh job thật gọi gate, không chỉ ghi lệnh vào docs.

### 0.2. Trace và bảo mật: không suy luận dữ liệu chưa đo

- Root run, model call/step, HTTP attempt và retry là các cấp khác nhau. Title generation, tool-loop requests, summarize và auth retry cần `purpose`/correlator riêng; một root run có đúng terminal event theo contract, kể cả lỗi chuẩn bị, cancel khi queued và requeue sau compaction. Không cộng chồng duration của span cha/con hoặc token của retry.
- Lưu monotonic **offset** của từng mốc và duration; unavailable/không xảy ra = absent, không phải 0. Đo riêng first reasoning, first tool và first non-empty text, không để reasoning đầu tiên che mất visible-text TTFT. Đây là engine text-available TTFT; chỉ gọi là UI-visible khi có timestamp ở UI.
- `GotFirstResponseByte` không phải first SSE event; observer decoder phải xác nhận một SSE frame hoàn chỉnh. HTTP TTFB gồm network/provider wait, không thể tự tách provider queue bằng client clock. Callback encode phải ở actual serializer. Test delay kiểm tra ordering và tolerance, không khẳng định delay injection là latency internet.
- `model_refresh` hiện bao hàm skill/tool work: chỉ báo exclusive local preparation hoặc union intervals; không cộng p95 của các span chồng nhau. Mọi lần gọi provider có attempt riêng và usage presence thật; SDK default zero không chứng minh cache miss.
- Fingerprint ở **sau tất cả OnPrepare/provider adapters/options/tool transforms**, có history, attachment metadata và canonical tool schema chứ không chỉ tool count. `request_shape_hmac` là HMAC của projection đã loại secrets/opaque reasoning, không quảng bá là full-wire hash. Không đưa ciphertext/signatures/OAuth/header vào bất kỳ digest nào. Hash domain-separated, encode length/fields rõ ràng; count bytes tách riêng.
- Giữ `prefix_changed_reason` để tương thích; thêm optional sorted `change_reasons` cho nhiều thay đổi đồng thời, gồm todo/provider-options khi liên quan. Stable-only generation không tăng vì Git/date/todo/MCP dynamic. Primary stable reason precedence: model_switch > compaction > context > skills > tool_set > none; base-template/policy đổi map vào context. Dynamic-only reasons nằm trong change_reasons, primary là none. Ghi initial trong change_reasons cho first observation; enum/precedence này phải được cập nhật vào contract và tests. Không suy ra reason chỉ từ hash.
- Validate bằng allowlist/length limits cho labels, span names, counters, enum và digest shape; error/warning chỉ ghi field/reason code, **không echo giá trị bị reject**. Sanitize trước mọi sink, kể cả engine slog/error paths, không chỉ JSONL. Nếu HMAC key không dùng được, telemetry báo unavailable, không fallback sang SHA/plaintext.
- Key publication phải chịu concurrent startup/crash; `O_EXCL` rồi write vẫn có cửa sổ file rỗng. Test reader/writer concurrency và failed creation. Windows cần kiểm chứng ACL thực tế; `0600` và test bỏ qua Windows không chứng minh private. Writer bounded/rotation và không chặn SSE dispatch vì disk chậm.
- Phân loại sink: prompt/history và encrypted reasoning được phép trong DB phiên riêng và fake-provider capture **trong bộ nhớ** để test replay; không được trong diagnostic logs/telemetry/published artifacts. Canary scan không scan cả fixture/DB rồi đòi chúng rỗng. Persisted evidence chỉ chứa verdict/count/redacted shape, không raw capture; canary cố ý của negative test chỉ nằm trong audit temp log.

### 0.3. Deterministic không chỉ là sort map

- Tách read path khỏi canonical identity/render path. Windows alias casing, relative/absolute và separator tương đương phải tạo **cùng bytes của path trong prompt**, không chỉ cùng dedupe key. Dedupe file lặp do roots chồng nhau trong cùng lane; giữ precedence project/global và không tự merge hai lane. Không thay symlink/junction semantics hoặc expand phạm vi đọc bằng `EvalSymlinks`.
- `context-prompt/snapshot-<timestamp>` hiện bị render vào stable prefix. Chốt generation content-addressed bằng install-key HMAC của canonical manifest (source-relative paths + bytes + policy version), reuse immutable physical snapshot path khi content không đổi. Không đưa temporary staging path vào rendered prefix. Không dùng timestamp/mtime làm cache invalidation authority. Cùng bytes sau restart/refresh phải giữ stable hash.
- Context/skills/model/base template/options ảnh hưởng prompt phải nằm trong một immutable run snapshot. Copy slice chứa pointers không đủ nếu pointee còn mutate. Refresh đọc dữ liệu một lần, atomically publish cả revision; không mix model mới với prompt/tools cũ. Rebuild khi context sửa cùng size hoặc model/template/skills đổi, không chỉ khi skills thay đổi. Failed refresh giữ committed snapshot trước, không âm thầm xóa context config.
- Test trên **Windows/NTFS thật** cho contract case-insensitive v1: alias casing, separator, relative/absolute, overlapping roots và bytes của rendered paths. Truyền platform giả chỉ chứng minh nhánh xử lý chuỗi, không thay Windows filesystem tests. Linux và Windows per-directory case-sensitive mode nằm ngoài scope milestone này, không phải release gate. Không gọi hash-stable là chứng minh provider cache hit; phải kiểm tra final wire ordering/tool schemas riêng.

### 0.4. Todo và options: giữ semantics, fail rõ ràng

- Todo là dữ liệu task do user/model tạo, không được nâng thành system policy. Stable/dynamic split **không đổi role** của reminder: giữ ephemeral user-role context sau system prefix, không persist. Snapshot todo lại ở mỗi model-call boundary sau tool update, không chỉ lần bắt đầu root run. Source label không biến task text thành trusted instructions.
- Thứ tự todo bám thứ tự task trong session (deterministic slice), không tự sort alphabet/status rồi làm mất priority. Caps phải tính cả XML escaping, state/wrapper/truncation marker và UTF-8 boundaries; báo total/omitted status counts để task active không bị che bởi completed tasks. Không có task thực sự mới được nói empty.
- Mọi invalid Responses options phải fail model/request preparation **trước network** và giữ config gốc. `slog.Error` rồi trả empty options vẫn là clobber, không phải fail model build. Propagate error qua tất cả callers, title/summarize/run; deep-copy maps/slices/pointers trước merge.
- SDK hiện dùng reasoning summary `auto|concise|detailed`, không có bằng chứng `none` hợp lệ. `reasoning_summary: null` là explicit omission, khác absent/default, cần test giữ nguyên. Assert field wire `reasoning.summary`, không nhầm với config key `reasoning_summary`; khóa precedence thực tế model/provider/workspace bằng config integration fixtures.
- Capability routing phải dựa trên model/endpoint đã kiểm chứng; không tự suy ra mọi model chứa `gpt-4`/`gpt-5` đều hỗ trợ cùng Responses reasoning fields. Unknown/custom aliases phải có explicit capability contract/test, không gửi field đoán mò.

### 0.5. TACK migration là transaction, không phải chuỗi rename rời

- Chốt state machine có version/generation: legacy, pending, staged, committed-layered, rolled-back. Stage core/USER/state/backup đầy đủ; validate và sync trước commit marker. Snapshot chỉ lấy **committed generation**. Per-file rename không làm cả nhóm file atomic; journal/recovery phải hoàn tất trước seeding/render lần tiếp theo.
- Không xóa legacy active trước khi generation mới sẵn sàng; malformed seed report, disk failure hoặc crash phải còn owner cũ dùng được. Reseed idempotent giữ backup token; explicit rollback phải tồn tại qua lần khởi động kế tiếp, không bị auto-migrate lại. Giữ backup/state đến hết retention một release, không reset chúng ở mỗi `Seed`.
- Preview/accept là compare-and-swap trên hash/generation của legacy + USER + core đã preview. Concurrent edit phải báo conflict; không overwrite edit mới hơn. Backup cả USER/state trước accept, rollback không xóa edit mới phát sinh sau accept; hiển thị conflict thay vì silent overwrite. Token rollback phải thuộc manifest/generation, không chỉ basename hợp lệ.
- Hash stock không thay thế base bytes cho 3-way merge: ship/retrieve được exact stock version. Không biết base thì manual review/merge; không invent base hoặc heuristic trích rules. Preserve existing root `USER.md`, `memory/USER.md`, MEMORY và auxiliary user files theo ownership riêng; pending legacy không được làm mất USER có từ trước.
- Đọc migration status một lần cho snapshot dưới lock, không một lần mỗi file. Snapshot immutable cần retention cho mọi workspace/run đang dùng; không prune global generation chỉ vì workspace khác vừa refresh. Core/USER không được double-loaded, file backup/report/staging không lọt prompt.
- `bundleseed` hiện đã hash nội dung: phải cập nhật docs size-only đã lỗi thời, không phát minh lại hệ seeding chung. Wails preview/accept/rollback cần đủ bound API, `desktop.ts`, generated bindings và UI thực sự; contract stubs và allowlist trong test không thay thế luồng UI.
- Chỉ de-duplicate rules do sản phẩm ship; không xóa customization của user vì user cố ý lặp rule. Portable test phải dùng đúng artifact `tack.exe`, isolated profile/pipe/single-instance namespace đã chứng minh. Chỉ đổi `%APPDATA%` chưa chứng minh không chạm engine/app thật.

### 0.6. Reasoning và compaction: xác định rõ cái phải chứng minh

- Full pipeline phải giữ ordered output items từ stream → message → DB → replay, không chỉ unit test dựng sẵn parts. Key event phải phân biệt request/step/output index; merge partial/end metadata không làm rơi field cũ, retry bỏ partial failed attempt nhưng không nhân đôi tool đã chạy. Clone phải deep-copy nested metadata.
- JSON replay gồm `type`, `id`, `encrypted_content` và **`summary: []`** theo SDK; không đổi summary display thành assistant text. Strict fake server validate schema, stream lifecycle và relative function-call/result order. Duplicate item ID là deterministic pre-network validation error (không silently dedupe).
- Provider/model fingerprint bao gồm endpoint/API mode và credential/account scope không bí mật (opaque hash), không chỉ string `openai`; không đưa token vào fingerprint. Legacy row thiếu fingerprint chỉ được replay khi compatibility suy ra chắc chắn từ metadata gốc, ngược lại emit explicit unsupported reason. Anthropic/Google signatures phải có regression tests riêng.
- Contract chọn local replay `store=false`; reject `previous_response_id` hoặc incompatible explicit store configuration trước khi dispatch, không âm thầm override user. Fake ciphertext chỉ chứng minh forwarding; **live synthetic test ở đúng endpoint/model** mới chứng minh opaque item được provider chấp nhận sau tool loop/restart. Chưa có proof thì không release continuity claim.
- Mục 9 block **hybrid compaction algorithm**, không block bounded PR5 fix của history selection. `getSessionMessages` hiện cắt từ summary ID nên làm mất mọi pre-summary anchor. PR5 phải giữ một complete latest valid assistant anchor group gồm ordered reasoning parts và associated call/results cần thiết, không orphan và không duplicate. Freeze exact boundary/ID/token-budget policy trong contract; không đổi LLM summary algorithm/threshold/summary role trong PR này. Recovery test phải kiểm tra committed summary pointer, không half-summary.

## E2E harness dùng chung — fail closed, build có provenance

`e2e/inputpipeline/e2e_test.go` và hai scripts hiện có chỉ là scaffold, **không phải gate đạt**. Thay bằng black-box runner thật; không import Crush internals trong Gotack tests.

1. Resolve repo root từ script, `Push-Location`/`finally Pop-Location`; temp root dùng RUNNER_TEMP khi có, fallback OS temp khi chạy local, tạo subdir unique. Check exit code của **từng** native command; failure/timeout/missing artifacts phải nonzero. Không áp patch vào checkout user đang dirty.
2. Clone pin → compatibility patches → hardening → input-pipeline patch phase → `go build -mod=readonly -trimpath -o <absolute-engine> .`. Entrypoint là root `.`, **không phải `./cmd/crush`**. Assert pin/patch manifest/Fantasy dependency/binary hash; `SkipBuild` chỉ nhận explicit absolute binary + matching provenance, không fallback tùy ý PATH.
3. Dùng `server --host <unique-npipe-or-isolated-loopback> --data-dir <temp>` theo CLI thật; cô lập global config/cache/home/workspace và provider credentials trong child environment. Không đoán endpoint từ stdout, không dùng biến `TACK_DATA_DIR` chưa được contract chứng minh. Windows có test named pipe qua `crushapi` transport; TCP test chỉ khi --host được chỉ rõ. Health readiness có deadline là bootstrap probe, không phải polling state thay SSE.
4. Fake Responses provider phải stream SSE hợp lệ với flush/terminal events; fake MCP nói JSON-RPC stdio thật, stdout chỉ protocol. Cấm fallback internet, clear live credentials trong child; không dump env. Tạo workspace thật qua REST, dùng ID trả về, cấu hình provider/model/permission fixture, init agent; không dùng workspace ID `test` tự đoán.
5. Subscribe workspace SSE bằng đúng client ID và chờ attached trước gửi prompt; submit REST rồi đọc tới matching run_complete, count provider requests và check captured body. Test tool-loop/retry phải assert request/call counts **lớn hơn 0** cùng expected outcomes, không chỉ khởi động fake server. Correlate qua harness transport metadata/in-memory mapping; không nhét run ID vào prompt cho vừa test.
6. Run root timeout, SSE read deadline, bounded stderr drain và cleanup `Wait`/terminate đúng process tree mình tạo trong mọi đường lỗi. Không kill app/engine của user. Capture body chỉ in-memory, artifact redacted. Restart dùng cùng isolated session DB.
7. Missing binary, endpoint not ready, zero captures, schema violation, dropped terminal event, unexpected SKIP hoặc unsupported platform là FAIL trong required CI lane. Negative controls phải cố ý gây từng lỗi đó và assert gate fail. Không đặt `skipIfNoEngine` trước test vốn không cần engine rồi đếm nó là E2E.
8. Windows CI phải build/pin explicit binary, export `TACK_ENGINE_BINARY`, chạy `go test -json -tags=e2e ./e2e/inputpipeline -count=1 -timeout=10m`; post-check required test names thực sự RUN/PASS, zero unexpected skips. Workflow hiện tại chưa gọi lane này. Race chạy trên Windows với CGO/C compiler phù hợp; named-pipe, ACL và portable tests cũng phải chạy trên Windows. Không thêm Linux/WSL lane để đáp ứng milestone Windows-only.

Các lệnh validation bên dưới chỉ là acceptance sau khi runner đã sửa; audit ngày 2026-09-05 không gọi scaffold pass là hoàn tất.

## 1. Observability phân rã latency và request shape (PR0)

### Vấn đề với code hiện tại

`coordinator.run` gộp ready wait, MCP wait, model refresh, skill refresh và tool build; `sessionAgent.Run` chỉ thấy semantic callbacks. `run_complete` hiện chỉ mang kết quả/cancel/error. Không có chuỗi thời gian để phân biệt local preparation, request encode/write, provider queue, first SSE, first reasoning/tool/text, retry và summarize. Cache token `0` cũng chưa phân biệt được miss với provider không báo.

### Phương án chỉnh sửa

- Tạo `RunTrace` theo `run_id`, dùng `time.Now()` chỉ làm mốc và `time.Since` monotonic để tính duration microsecond.
- Gắn spans: `ready_wait`, `mcp_wait`, `model_refresh`, `skill_scan`, `tool_build`, `history_load`, `prompt_prepare`, `request_encode`, `request_write_to_first_byte`, `first_byte_to_first_sse`, `first_sse_to_first_reasoning|tool|text`, `stream`, `summarize` và tổng run.
- Dùng `httptrace.ClientTrace` cho `WroteRequest` và `GotFirstResponseByte`; Fantasy upstream thêm hook cho `request_encoded` và response metadata nếu Crush không quan sát được. Không gán timestamp giả.
- Mở rộng `run_complete` bằng field optional `telemetry`; client cũ phải bỏ qua được.
- Cache status là enum `hit|miss|unreported`; cached/uncached token là pointer/optional để bảo toàn “absent”.
- Ghi effective reasoning effort, provider/model, attempt, retry count/delay, service tier, provider request ID, estimated usage và compaction flag.
- HMAC-SHA256 prompt/context/tool và sanitized request-shape projection bằng khóa 32-byte sinh một lần trong data dir; tuyệt đối loại ciphertext/credentials khỏi hash input theo mục 0.2; log chỉ digest base64url và byte count.
- `prefix_changed_reason` là một trong `git_status|date|mcp|skills|context|tool_set|compaction|model_switch|none`.

### Scope chỉnh sửa

- Crush patch: `internal/agent/coordinator.go`, `internal/agent/agent.go`, publisher/type của `run_complete`, provider call boundary và tests liên quan.
- Fantasy upstream: OpenAI Responses transport timing/header/usage presence; pin tạm reviewed commit nếu cần.
- Gotack: `internal/crushapi/contract.go`, `internal/uievents/forwarder.go`, package mới `internal/runmetrics/` để validate và ghi JSONL, cùng `docs/contracts/crush-rest-sse.md`.
- Không thêm biểu đồ/UI trong PR0; artifact chuẩn là JSONL redacted và report benchmark Markdown.

### Test yêu cầu

- Unit/contract: clock giả kiểm tra duration không âm, first-event chỉ ghi một lần, tri-state cache giữ absent, HMAC ổn định nhưng không lộ input.
- Integration: fake provider cố ý delay từng mốc; assert ordering `run_start <= request_written <= first_byte <= first_sse <= semantic <= run_complete` và tolerance phù hợp.
- E2E: gửi một turn qua executable/REST/SSE; đối chiếu cùng `run_id` trong provider capture, `run_complete.telemetry` và JSONL.
- E2E retry: fake provider trả 429 rồi thành công; assert attempt, retry delay và total không double-count token.
- Security E2E: seed synthetic canaries, chạy turn rồi scan diagnostic logs/telemetry/exported evidence theo sink policy mục 0.2, gồm rejected-input/error paths. DB/capture in-memory dùng kiểm tra replay được kiểm riêng, không áp điều kiện canary-absent sai scope.
- Backward compatibility: Gotack mới đọc được event cũ không có telemetry; payload mới không làm hỏng consumer UI/Zalo/scheduler.

## 2. Prompt assembly deterministic và path identity (PR1)

### Vấn đề với code hiện tại

`loadContextFiles` trả `map[string][]ContextFile`, rồi `promptData` range map nên thứ tự group không xác định. `sessionAgent.Run` range trực tiếp map từ `mcp.GetStates()`, nên nhiều MCP có instructions có thể đổi byte prompt. `strings.ToLower(expanded)` đang áp dụng trên mọi OS: sai trên filesystem case-sensitive, nhưng bỏ case-fold trên Windows lại có thể nạp trùng cùng path.

### Phương án chỉnh sửa

- Thay return type map bằng ordered `[]ContextGroup`; không giữ API dễ tái phạm nondeterminism.
- Canonical key: resolve theo working dir, `filepath.Abs` + `filepath.Clean`; Windows lower-case key, OS khác giữ nguyên case. Không `EvalSymlinks` bắt buộc vì có thể đổi semantics/permission.
- Dedupe bằng canonical key, sau đó sort lexical key trước khi flatten; `WalkDir` giữ thứ tự file lexical.
- Sort danh sách MCP server name trước khi lọc `connected && instructions != ""` và render.
- Ghi policy vào contract: Gotack v1 không hỗ trợ hai Windows directories chỉ khác casing hoặc per-directory case-sensitive semantics.
- Không đổi thứ tự tool/skill đã canonical; không tuyên bố đây là performance fix.

### Scope chỉnh sửa

Crush patch chạm `internal/agent/prompt/prompt.go`, `internal/agent/agent.go` và tests của hai package. Gotack chỉ cập nhật patch, `third_party/README.md` nếu cần và contract/path limitation; không thêm logic prompt vào desktop.

### Test yêu cầu

- Randomized test 100–1000 permutations của ba context roots và ba MCP; output byte-identical và hash-identical.
- Positive/negative path tests trên Windows/NTFS case-insensitive: `A` và `a` là cùng root nên dedupe thành một và render cùng canonical bytes dù đảo input order; relative/absolute và separator tương đương dedupe; path không tồn tại không crash. Không yêu cầu Linux hoặc Windows per-directory case-sensitive tests.
- E2E executable: ba fake MCP khởi động với delay/ngẫu nhiên khác nhau, hai context roots đảo config order; provider capture phải nhận một prompt canonical duy nhất qua ít nhất 20 process restarts.
- E2E built-in: Gotack memory/skills/recall không có instructions thì không sinh `<mcp-instructions>` rỗng.
- Patch replay: clone pin sạch, áp patch theo phase rồi filename order như mục 0.1, chạy focused Crush tests và `go test ./...`. Không áp incremental patch dựa trên hardened base trước hardening.

## 3. Provider options không được clobber (PR1)

### Vấn đề với code hiện tại

Trong `getProviderOptions`, Responses reasoning model luôn gán lại `mergedOptions["include"]` thành chỉ `reasoning.encrypted_content` và luôn gán `reasoning_summary="auto"`. Cấu hình user như file-search results hoặc logprobs bị mất dù merge trước đó thành công.

### Phương án chỉnh sửa

- Viết helper thuần `mergeResponsesIncludes(configured, required)` nhận `[]string`, `[]openai.IncludeType` hoặc JSON-decoded `[]any` sau normalize.
- Union option user với `reasoning.encrypted_content`, loại trùng và sort lexical để request bytes canonical.
- `reasoning_summary="auto"` chỉ là default khi key chưa tồn tại; giá trị user hợp lệ thắng.
- Mọi mutation chỉ diễn ra trên local `mergedOptions`; không mutate parsed options/pointer dùng chung.
- Parse/normalize lỗi phải propagate error qua getProviderOptions và mọi caller tới model/request preparation; không gọi provider. Diagnostic dùng field/reason code đã sanitize. Không được log rồi trả empty options, không mutate config user; thêm negative test chứng minh zero provider requests.

### Scope chỉnh sửa

Crush patch tại `internal/agent/coordinator.go`, focused tests trong `coordinator_test.go`, fixture config JSON và E2E provider capture. Không chạm Gotack provider overlay vì defect nằm sau merge, tại request option boundary.

### Test yêu cầu

- Unit table: nil, empty, duplicate, mixed representation, custom includes, user summary `auto|concise|detailed`, explicit null và invalid `none`; assert union/sort/default.
- Integration: provider options từ model + provider + workspace cùng tồn tại và precedence cũ không đổi ngoài required union.
- E2E: cấu hình `include=["file_search_call.results"]`, gửi turn; fake provider phải thấy đúng hai includes, mỗi giá trị một lần, và giá trị wire `reasoning.summary` do user chọn; explicit null phải omit summary chứ không khôi phục auto.

## 4. Todo reminder phản ánh state thật (PR1)

### Vấn đề với code hiện tại

`preparePrompt` luôn prepend câu “todo list is currently empty” và không nhận todo state. Khi session có todo, model nhận dữ liệu sai; reminder lại giả dạng user message dù không được user gửi.

### Phương án chỉnh sửa

- Truyền immutable todo snapshot vào model-call preparation; copy dưới lock cùng revision session, refresh sau tool updates trong cùng run. Không đọc shared mutable todos khi render.
- Tạo pure renderer cho ba trạng thái: empty, có pending/in-progress, và tất cả completed.
- Với non-empty, render status + nội dung task trong `<system_reminder><todos>...</todos></system_reminder>`; escape XML, đặt byte/task cap và ghi rõ đây là engine state.
- Reminder là ephemeral context, không persist thành message và không xuất hiện trong transcript.
- Giữ wrapper user-role hiện tại cả PR1 và PR2; reminder thuộc ephemeral dynamic user context sau system prefix, không chuyển task text vào system policy và không persist.

### Scope chỉnh sửa

Crush patch tại `internal/agent/agent.go`, todo/session type liên quan và tests. Nếu shape todo đi qua SSE không đổi thì Gotack không cần contract change.

### Test yêu cầu

- Unit: empty, mixed status, completed, ký tự XML, over-limit truncation và deterministic ordering.
- Integration: tạo/update/complete todo rồi gọi `preparePrompt`; nội dung phản ánh đúng snapshot, không có phrase “currently empty” khi có task.
- E2E: fake provider turn 1 gọi tool `todos` tạo hai task; turn 2 provider capture phải thấy cả hai và status; hoàn thành task rồi turn 3 thấy state mới.
- E2E persistence: restart engine giữa turn 1 và 2; reminder vẫn đúng từ session DB nhưng không có synthetic reminder trong API message history.

## 5. Stable prefix + dynamic suffix và một skill builder (PR2)

### Vấn đề với code hiện tại

`Prompt.Build` trộn policy với date, Git status, env, skill index và context trong một template. `promptData` tự discover skill trong khi `skills.Manager` cũng discover/refresh; hai pipeline có thể drift. `UpdateModels` rebuild prompt/tools mỗi run nên không biết thay đổi nào thật sự làm prefix đổi.

### Phương án chỉnh sửa

- Refactor thành collector + pure renderer BuildStable/BuildDynamic trên snapshot typed: stableSystem, dynamicSystem, ephemeralUserContext. Stable system bytes đứng trước dynamic system bytes; reminder giữ user role theo mục 0.4. Không cắt ghép bằng cách tìm marker trong arbitrary user/context content.
- Stable gồm base coder policy, ordered project/global context và canonical skill index. Dynamic system gồm working dir/platform/date, Git snapshot, MCP instructions và system run notes vốn được phép. Todo nằm riêng trong ephemeral user-role context; không thay đổi instruction hierarchy để tối ưu cache.
- `skills.Manager.ActiveSkills()` là input duy nhất cho skill XML. Xóa discovery khỏi `prompt.promptData`; initial build, refresh endpoint và pre-run refresh dùng chung một builder.
- Snapshot có immutable content/model/template/config generation. Rebuild stable khi bất kỳ stable input đổi, gồm context same-size edit và model/template switch; cùng content reuse generation/path. Git/date/MCP/todo chỉ đổi dynamic lanes; tất cả readers dùng một revision theo mục 0.3.
- Mỗi run tính riêng `stable_prefix_hmac`, `dynamic_suffix_hmac` và `request_shape_hmac`; reason được lấy từ diff generation, không đoán từ hash.
- Chưa cache model/tool object trong PR2. Telemetry quyết định workstream sau nếu exclusive/union local preparation vượt 50 ms p95 hoặc ngưỡng 5% được tính trên paired run measurements. Không cộng p95 hay parent/child spans chồng nhau.

### Scope chỉnh sửa

- Crush patch: `internal/agent/prompt/prompt.go`, `internal/agent/templates/coder.md.tpl`, `internal/agent/coordinator.go`, `internal/agent/agent.go`, `internal/skills/manager.go` và focused tests.
- Cập nhật patch `prompt-context-refresh.patch` hoặc thay bằng patch bounded tương đương; endpoint `/agent/refresh-prompt` vẫn giữ contract.
- Gotack cập nhật `docs/contracts/crush-rest-sse.md` cho telemetry hashes/reasons nếu payload đã mở rộng.

### Test yêu cầu

- Pure tests: cùng snapshot tạo cùng bytes; date/Git/MCP/todo đổi chỉ làm dynamic hash đổi; skill/context đổi làm stable generation đổi đúng một lần.
- Equivalence: initial builder output bằng refresh builder output với cùng input; chạy randomized order 100 lần.
- Concurrency/race: skill refresh đồng thời hai run không tạo mixed snapshot; chạy `go test -race` package liên quan.
- E2E: fake provider capture bốn turn: unchanged, Git đổi, skill thêm, MCP instruction đổi. Assert stable prefix giữ nguyên ở turn 2/4 theo đúng classification và chỉ đổi ở turn 3; dynamic suffix phản ánh từng thay đổi.
- E2E refresh: Gotack build snapshot mới rồi gọi `/agent/refresh-prompt`; session, queue và message history không reset.

## 6. Một nguồn sở hữu policy: TACK core + user context (PR4)

### Vấn đề với code hiện tại

`resources/context/TACK.md` lặp nhiều generic rule đã có trong `coder.md.tpl`. `contextseed` đang áp `UserEditableFiles` cho toàn cây, nên bundled TACK mới không thể nâng cấp file user cũ an toàn. Chỉ rút gọn file bundled sẽ tạo hành vi khác giữa install mới và cũ.

### Phương án chỉnh sửa

- Chọn mô hình hai lớp: `TACK_CORE.md` do sản phẩm quản lý và `USER.md` do user quản lý. `TACK_CORE` chỉ giữ identity/capability Gotack, Windows/Office/Zalo, guard, memory/recall/skills; generic coding/workflow rules chỉ thuộc coder template.
- New install: seed managed core và user file rỗng/có hướng dẫn; không seed legacy `TACK.md`.
- Dùng manifest versioned chứa hash các bản TACK stock. Legacy stock khớp hash được auto-migrate atomically, backup ra thư mục không nằm trong prompt snapshot.
- Legacy modified/unknown không được overwrite và không được load đồng thời với core. Giữ legacy mode, tạo trạng thái `migration_pending`, hiển thị diff/preview và chỉ chuyển sau explicit approval.
- Candidate `USER.md` được dựng bằng 3-way diff với base version đã ship; conflict bắt buộc user resolve. Không tự “trích rule” bằng heuristic/LLM.
- Sau accept: atomic rename/write, rebuild immutable prompt snapshot, refresh prompt; backup và rollback token được giữ một release.

### Scope chỉnh sửa

- `resources/context/TACK_CORE.md`, `resources/context/USER.md` và migration fixture/manifest cho legacy `TACK.md`.
- `internal/contextseed/seed.go`, `snapshot.go`, migration files/tests; mở rộng `bundleseed` chỉ bằng API policy theo file, không đổi semantics caller khác.
- `context_seed.go` giữ helper; Wails methods thuộc nhóm bind_*.go (ví dụ bind_context.go), đi qua desktop.ts, generated bindings và UI preview/accept/rollback thật. Cập nhật binding contract và bảng layout AGENTS.md/README.md trong implementation PR.
- `docs/contracts/crush-rest-sse.md` cập nhật marker/source layout; thêm ADR về ownership lâu dài trong `docs/decisions/`.

### Test yêu cầu

- Unit: new install, stock legacy, modified legacy, unknown hash, conflict, interrupted atomic write và rollback.
- Integration: snapshot ở mỗi mode chỉ chứa đúng một policy owner; managed core update được, `USER.md` luôn preserved.
- E2E real app: chạy portable build với temp `%APPDATA%`; kiểm tra new install, nâng từ stock và nâng từ modified. Với modified, agent-browser phải thấy preview, cancel giữ nguyên bytes, accept rồi restart vẫn giữ user text.
- E2E provider capture: generic rule chỉ xuất hiện một lần; Gotack-specific canaries và custom `USER.md` xuất hiện đúng một lần; legacy file không còn trong active snapshot sau accept.
- Recovery E2E: kill app giữa staging và rename; lần start sau chọn state cũ hoặc mới hoàn chỉnh, không có half-written prompt.

## 7. Reasoning continuity đầy đủ cho OpenAI Responses (PR5, P0)

### Vấn đề với code hiện tại

Crush lưu một `ResponsesReasoningMetadata` trên `ReasoningContent` tổng hợp; update/finish có nhánh làm rơi metadata, `ToAIMessage` chỉ emit reasoning khi `Thinking != ""`, nên encrypted-only item bị bỏ. Fantasy v0.41.3 skip toàn bộ `ContentTypeReasoning` khi dựng request kế tiếp và có thể drop assistant message chỉ chứa reasoning. Nhiều reasoning item bị collapse thành một.

### Phương án chỉnh sửa

- Đổi message model thành ordered reasoning parts, mỗi part có event ID, `item_id`, encrypted content, provider/model fingerprint, timestamps và optional display summary; không concatenate metadata khác item.
- Callback API nội bộ theo ID: start/append/finish phải update đúng part và preserve mọi field không đổi.
- `ToAIMessage` emit reasoning part khi có metadata dù thinking text rỗng; filtering dựa trên provider/model hiện tại.
- Fantasy upstream map reasoning part thành structured Responses input item `{type:"reasoning", id, encrypted_content, summary:[]}` tại đúng vị trí tương đối trước function call liên quan.
- Không replay `Thinking` hoặc `Summary[]` thành assistant text. Mỗi `item_id` tối đa một lần/request; duplicate là deterministic pre-network validation error; không silent dedupe.
- Chọn explicit replay với `store=false`; reject config kết hợp `previous_response_id` với replayed history.
- Khi đổi provider/model, drop anchor không tương thích và emit reason `model_switch`; tuyệt đối không log/hash ciphertext.
- Sau compaction giữ complete latest valid assistant anchor group cùng dependency call/results theo mục 0.6; loại các group cũ trong compacted range, không cắt đôi tool pairs. Bounded history-selection fix thuộc PR5, hybrid algorithm vẫn blocked.

### Scope chỉnh sửa

- Crush patch: `internal/message/content.go`, serialization/DB compatibility, `internal/agent/agent.go`, prompt preparation, summary/compaction selection và tests.
- Fantasy upstream OpenAI Responses converter/types/tests; Crush `go.mod` chỉ pin commit chứa fix tạm thời nếu chưa có release.
- Thêm contract `docs/contracts/openai-reasoning-continuity.md`; không đưa reasoning persistence vào desktop.

### Test yêu cầu

- Unit: metadata sống qua start/delta/end/finish/retry/JSON round-trip; encrypted-only và multi-item không collapse.
- Fantasy contract test: hai reasoning items xen với hai function calls được convert đúng schema/order, không text leakage và không duplicate.
- E2E two-turn: fake provider trả hai encrypted-only reasoning items + tool calls; restart engine; request turn sau phải replay đủ hai item đúng một lần và đúng vị trí.
- E2E model switch: tạo anchor ở model A, chuyển model B, request không chứa ciphertext A và telemetry ghi `model_switch`.
- E2E compaction: ép threshold thấp; sau summarize chỉ latest valid anchor được replay, không orphan tool call/result.
- Security E2E: canary ciphertext không xuất hiện trong slog, JSONL telemetry, SSE hoặc UI transcript; chỉ đi tới request body của đúng provider.

## 8. Benchmark causal và `prompt_cache_key` experiment (PR3)

### Vấn đề với code hiện tại

Chưa có baseline phân rã; matrix A→B→C cộng dồn không chỉ ra causal effect. Fantasy có option `prompt_cache_key`, nhưng bật đại trà trước khi có cache tri-state và stable prefix sẽ biến giả thuyết thành product behavior không chứng minh được.

### Phương án chỉnh sửa

- Runner bắt buộc gửi request và thu telemetry từ executable/REST/SSE/provider thật; không dùng `Random.Next` làm latency, không đo thời gian tạo object. FakeProvider chỉ test runner/correctness, report có `synthetic=true`; mất endpoint/zero captures phải FAIL. Mặc định không có credentials và không tự chuyển từ fake sang live.
- Workloads: fresh, warm turns 2–10, 30-turn, near-compaction, synthetic MCP/large history. Freeze account/endpoint/model snapshot/effort/tool schemas/prompt và history fixtures, engine/Fantasy revision, warm-up count, seed, concurrency và request budget trước khi chạy.
- Mỗi pair có cả control A và treatment B từ cùng logical starting state nhưng isolated session/DB/tool side effects, randomize AB/BA. Tối thiểu 30 **independent pairs/workload**, không phải 30 lần chọn ngẫu nhiên A hoặc B. Warm turns trong một session không phải independent samples; bootstrap theo session-pair clusters, không theo từng token/turn phụ thuộc.
- Cả hai arms có warm-up tương đương và thứ tự cân bằng để hạn chế account cache carryover/rate limits. Fresh session không đồng nghĩa provider cold cache. Record cache-state uncertainty thay vì tuyên bố reset cache mà provider không hỗ trợ; không đổi prompt riêng từng arm ngoài biến treatment.
- Đo riêng exclusive local prep, HTTP TTFB, first reasoning/tool/non-empty text, full root turn và summarize. Tool-only/cancel/no-text = TTFT absent, không ghi 0. Count toàn bộ errors/retries/timeouts trong mẫu; latency cho successful runs báo riêng kèm số censored/missing và attrition, không âm thầm bỏ outliers.
- Dùng percentile nearest-rank `sorted[ceil(p*n)-1]`, bootstrap 95% CI theo pairs với seed cố định và ít nhất 10,000 resamples. Unit fixtures `1..100` phải trả p50=50/p95=95; test empty/singleton/missing data. N=30 cho p95 rất ít tail observations: nếu CI quá rộng thì inconclusive, không tự tuning tới khi có significance.
- Control không key; treatment 1 là static opaque workspace HMAC qua config sẵn có. Chỉ sau gate preregistered có tín hiệu mới thử treatment 2: `base64url(HMAC(install_key, length-prefixed workspace/session/model-fingerprint/prefix-epoch))`. Options map mới mỗi model request; config rollback giữ nguyên key user vốn tự cấu hình.
- `prefix_epoch` chỉ tăng đúng một lần cho committed stable context/skills/TACK change, compaction commit hoặc model change; thất bại/retry không tăng. Git/date/todo/MCP dynamic không tăng. Scoped counter/digest phải ổn định qua restart; test đồng thời và hai session interleave không nhầm history. Prompt caching không phải conversation storage hay cơ chế cách ly bảo mật.
- Rollout chỉ khi warm text-available TTFT p50 cải thiện >=10% và 95% CI improvement >0; p95/full-turn không xấu hơn 5% với CI đủ precision, error/retry không tăng theo rule đã khóa, không history contamination. Nếu muốn claim UI-visible latency phải đo UI như mục 0.2. Không đủ mẫu/precision thì BLOCKED/off, không đổi threshold sau khi xem kết quả.
- Live run chỉ dùng synthetic prompts với endpoint/model/account và cost/request cap được chủ repo duyệt, credentials đặt trong môi trường riêng, không yêu cầu dán secrets vào chat. Fake endpoint pass không mở live performance gate. Nếu không đạt, giữ flag off và lưu kết luận/limitations.

### Scope chỉnh sửa

`scripts/bench-input-pipeline.ps1`, `e2e/inputpipeline` fixtures, report template dưới `docs/benchmarks/`, config feature flag Gotack và bounded request-option merge trong Crush chỉ cho treatment 2. Artifact chỉ giữ redacted metrics/opaque IDs, mặc định dưới tmp/bench-input-pipeline (đã ignored), không raw provider capture. Fake/synthetic results gắn nhãn rõ và không được dùng mở rollout gate.

### Test yêu cầu

- Deterministic harness test xác nhận randomization seed, pairing, sample count và percentile calculation.
- E2E fake provider kiểm tra key ổn định trong cùng session/epoch, khác giữa sessions, đổi khi model/compaction/stable prefix đổi, không đổi do date/Git/todo.
- E2E two-session interleave chứng minh không replay/cache state chéo session.
- Live benchmark là gate thủ công có credentials; CI chỉ chạy fake-provider correctness và không khẳng định latency internet.
- Report phải tách local preparation, TTFB, semantic/visible TTFT, full turn, summarize và `hit|miss|unreported`.

## 9. Hybrid compaction (blocked, không implement trong milestone hiện tại)

### Vấn đề với code hiện tại

Patch `proactive-auto-compact.patch` chỉ đổi ngưỡng và vẫn dùng summary LLM hiện hữu. Chưa có baseline hoặc contract cho file state, reasoning anchor, token accounting và orphan tool pairs; implement ngay có nguy cơ phá lịch sử session.

### Phương án chỉnh sửa

- Giữ patch ngưỡng 128K và LLM summary algorithm hiện tại trong PR0–PR5. Chỉ cho phép bounded PR5 history-selection/anchor preservation/recovery contract ở mục 0.6; không triển khai local/hybrid compactor.
- Entry gate: PR0 baseline hoàn tất, reasoning contract pass, và ADR/contract compaction được duyệt.
- Khi mở workstream: local deterministic pass chỉ dedupe/truncate tool output, reconcile file state last-write-wins và chọn boundary không cắt tool call/result; LLM chỉ tóm tắt narrative còn lại.
- Chốt summary role là **user** với wrapper `<conversation_summary>` vì đây là role hiện tại trong `getSessionMessages` và tương thích provider rộng nhất; không đổi role trong cùng PR với thuật toán.
- Giữ ID verbatim, order monotonic, idempotence; chuyển usage của range bị compact vào summary metadata và tách live-context khỏi lifetime tokens.
- Emit `tokens_before/after`, `messages_evicted`, `summary_source=local|llm`, duration và cache epoch; không emit nội dung.

### Scope chỉnh sửa

Hiện tại chỉ tạo ADR + contract + fixture design sau entry gate. Workstream sau mới chạm Crush summarization/getSessionMessages/filetracker/message DB và cập nhật `docs/contracts/provider-usage-and-compaction.md`. Desktop không được chứa compactor.

### Test yêu cầu trước khi bỏ trạng thái blocked

- Property tests: compaction chạy hai lần cho cùng input cho cùng output; không duplicate/drop ID ngoài policy.
- E2E: history có parallel tool calls, lỗi/cancel, file write nhiều lần, reasoning multi-item và attachment; compact qua executable rồi tiếp tục turn thành công.
- E2E recovery: kill trong lúc summarize; restart không chọn half-summary và có thể retry.
- Accounting E2E: live tokens giảm, lifetime tokens không giảm, cache epoch tăng đúng một lần.
- Golden semantic suite so sánh task completion trước/sau; local-only summary không được rollout nếu giảm chất lượng vượt ngưỡng đã duyệt.

## Acceptance và rollback theo PR

- **PR0:** optional telemetry field; rollback bỏ consumer/flag, event cũ vẫn đọc được. Không merge nếu mốc wire bị suy đoán hoặc canary leak.
- **PR1:** rollback từng bounded helper/patch độc lập; không rollback bằng cách khôi phục nondeterministic map iteration trong production.
- **PR2:** giữ legacy full prompt builder sau feature flag trong một release; shadow-build và compare hash/semantic fixture trước khi bật mặc định.
- **PR3:** kill switch mặc định off; rollback chỉ tắt key, không mutate config user.
- **PR4:** mọi migration có backup + atomic commit + explicit rollback; modified legacy không được auto-migrate.
- **PR5:** release gate P0 theo đúng endpoint/provider/model; upstream hoặc live acceptance chưa đạt thì BLOCKED, không cho warning-only hoặc default-off thay thế release requirement. Không silently discard reasoning.

## Lệnh validation bắt buộc

```powershell
node scripts/check-repository-invariants.mjs
go test ./...
go vet ./...
staticcheck ./...
pnpm --dir frontend check
pnpm --dir frontend test
pnpm --dir frontend build
./scripts/test-input-pipeline-e2e.ps1
```

Ngoài ra mỗi Crush patch phải được áp từ checkout đúng `.tack-pin`, chạy focused package tests, `go test ./...`, build `tack-engine.exe`, rồi chạy E2E black-box. PR có UI migration phải chạy portable app bằng agent-browser và lưu screenshot/log redacted làm artifact.

## Definition of Done

- Mỗi PR cập nhật contract/ADR và patch provenance tương ứng; không có direct edit phụ thuộc vào ignored vendor tree.
- Focused, integration, E2E và repository gates đều pass; E2E chạy qua executable + REST/SSE + fake provider, không chỉ gọi helper.
- Telemetry chứng minh mốc đo và không chứa dữ liệu nhạy cảm.
- Cùng logical input tạo bytes canonical; mọi thay đổi prefix có reason xác định.
- Config user, todo state, TACK customization và reasoning metadata không bị mất qua restart/refresh/compaction boundary được phép.
- Không còn quyết định kiến trúc mở trong scope PR; nếu evidence mới phủ định giả định, coder dừng và cập nhật file này/ADR thay vì tự đổi policy.

## 10. Evidence ledger — audit 2026-09-05

### Phạm vi và provenance đã xác minh

- Parent HEAD: `d07c8929b8737fedcc7e2584e78f6e9c7d662bc3`; `.tack-pin`: `6d14dd93a9e526505f7de54ae5999431bc32a793`. Working tree đã dirty trước audit; audit không sửa implementation, không stage/commit/reset các thay đổi đó.
- `Upgrade.md` absent. `ImplementPlan.md` vốn untracked; original backup tại `tmp/plan-audit-20260905/ImplementPlan.before.md`, SHA256 `6db066123fa76d3c94d94b201d8e18e4f7054f897288b463640914fc483fcbd5`.
- Scratch Crush `tmp/crush-input-pipeline` ở `40d74a1add85bb4a3d09db3d6959721ce600fbf0` + dirty edits; scratch Fantasy ở `f06034c7824ffddc4394d4cefa5ed5132a186b1b` + dirty edits. Scratch Crush có local Fantasy replace. **Không** coi hai thư mục này là patch/release/upstream acceptance.
- Tracked engine patch set hiện chỉ có bốn compatibility patches. `zz-input-pipeline-windows.patch` và Fantasy reasoning patch được docs nhắc tới nhưng không tồn tại tại thời điểm audit. Graph index không chứa những scratch/new functions; source và tests trực tiếp được dùng thay index cũ.

### Các kiểm tra đã chạy và kết quả thật

| Kiểm tra | Kết quả audit | Giới hạn |
| --- | --- | --- |
| `node scripts/check-repository-invariants.mjs` | PASS | Checker chủ yếu quét tracked files; không chứng minh untracked work đã được CI/merge gate kiểm |
| `go test -count=1 -timeout=180s ./...` trong Gotack | PASS | Không bao gồm nested Crush/Fantasy module hoặc E2E build tag |
| `go vet ./...`, `staticcheck ./...` | PASS | Không chứng minh crash safety / provider wire |
| `pnpm --dir frontend check` | PASS; 0 errors, 0 warnings | Không có migration UI flow để coi là đã E2E |
| `pnpm --dir frontend test` | PASS; 39 tests / 7 files | Existing suite, chưa chứa các acceptance tests mới |
| `pnpm --dir frontend build` | PASS | Build output trong ignored frontend/dist, không sửa UI source |
| Scratch Crush: agent/prompt, agent, message, skills focused tests | PASS | Local dirty prototype, không phải durable patch replay |
| Scratch Fantasy: selected Responses conversion/terminal-stream tests | PASS | Fake/local shape tests, không phải provider thật |
| Pin-clean clone + bốn patches + hardening | PASS | Chỉ baseline compatibility; input-pipeline patch chưa tồn tại |
| Clean baseline focused agent/message/skills + engine build root `.` | PASS | `internal/agent/prompt` trên baseline báo **no test files**, không tính như coverage pass |
| E2E entrypoint script nguyên trạng | FAIL trước build | RUNNER_TEMP rỗng; build target `./cmd/crush` cũng được xác minh không tồn tại |
| Race với default CGO=0; retry process-local CGO=1 | BLOCKED môi trường | Lần hai lỗi `gcc not found`; không cài compiler hay đổi global env |

Clean baseline binary: `tmp/plan-audit-20260905/tack-engine-audit.exe`, SHA256 `d0ada5ba58ff653688b38eb06098cf1ebf90d633d55da90437f642cee320a679`. `server --help` xác nhận `--host` và `--data-dir`. Đây **không** phải binary triển khai PR0–PR5.

### Negative controls và regression probes đã tái hiện

Các probes dùng Go `-overlay`, không thêm test file vào implementation tree. File/overlay/log chỉ ở `tmp/plan-audit-20260905/`; assertions dưới đây cố ý yêu cầu invariant đúng và FAIL trên code hiện tại.

| Probe | Bằng chứng quan sát | Gate cần thêm |
| --- | --- | --- |
| `TestAuditStockReseedKeepsRollbackToken` | FAIL: lần Seed kế tiếp mất backup token | Preserve migration manifest/retention qua startup |
| `TestAuditRollbackSurvivesReseed` | FAIL: explicit legacy rollback trở lại layered | Rollback là durable state, không tự migrate lại |
| `TestAuditFailedSeedKeepsLegacyOwner` | FAIL: malformed seed report làm Seed lỗi sau khi TACK.md active đã bị xóa | Stage/validate trước retirement; transaction recovery |
| `TestAuditInvalidTelemetryDoesNotLeakRejectedValue` | FAIL: warning log echo synthetic canary trong invalid cache status | Sanitize validation errors trước mọi sink |
| `TestAuditWindowsAliasOrderPreservesRenderedPaths` | FAIL: cùng dedupe key nhưng rendered file path khác casing theo input order | Canonical rendered identity, không chỉ canonical key |
| `TestAuditStablePrefixIgnoresPhysicalSnapshotGeneration` | FAIL: cùng file bytes nhưng snapshot-1 / snapshot-2 đổi prefix | Content-addressed stable generation/path |

`TestAuditSnapshotGenerationDoesNotChangeOnNoop` chỉ là observation: file bytes bằng nhau nhưng hai lần snapshot tạo đường dẫn khác nhau; **không** tính observation PASS là stable-prefix proof.

Hai negative controls khác:

- Đặt `TACK_ENGINE_BINARY` tới đường dẫn không tồn tại rồi chạy TelemetryPassthrough/FakeProviderCapture/RetryBehavior: **cả ba vẫn PASS**. Chúng không đưa request thật qua engine; RetryBehavior assert số attempts bằng 0. Không chạy FreshTurn với engine/PATH/user profile thật vì isolation của scaffold chưa đúng.
- `bench-input-pipeline.ps1 -Endpoint http://127.0.0.1:1 -Iterations 2 -Seed 42` vẫn hoàn tất dù FakeProvider=false. Source tạo Random telemetry và không gọi Endpoint. Report đặt trong `tmp/plan-audit-20260905/synthetic-benchmark-NOT-performance-evidence/`; **không dùng số liệu đó đánh giá latency/cache**.

### Lệnh tái hiện probes (dự kiến FAIL cho đến khi sửa implementation)

```powershell
Set-Location D:/gotack
$audit = 'D:/gotack/tmp/plan-audit-20260905'
go test "-overlay=$audit/host-overlay.json" -count=1 -run '^TestAudit' -v ./internal/contextseed ./internal/runmetrics
go -C D:/gotack/tmp/crush-input-pipeline test "-overlay=$audit/prompt-overlay.json" -count=1 -run '^TestAudit' -v ./internal/agent/prompt
```

Log: `host-probes.log`, `prompt-probes.log`, `clean-focused.log` tại audit dir. Khi implement, chuyển các regression cases vào đúng owning package và chứng minh chúng PASS sau fix; không xóa assertions để làm xanh. Audit temp là evidence local, không phải dependency của CI/release; durable case descriptions nằm ngay trong Plan này.

### Blockers cần hỗ trợ và phần chưa làm

- **Môi trường Windows:** audit xác minh thiếu GCC/MinGW cho race; đã thử bật CGO riêng process vẫn fail. Cần C compiler phù hợp cho Go race trên Windows trong PATH hoặc Windows runner có sẵn toolchain đó. Không cần Linux/WSL; lỗi WSL từng gặp chỉ là lịch sử thử nghiệm, không còn là blocker của milestone Windows-only. Không tự cài toolchain/bật Windows service trong audit.
- **Provider thật đã có:** chủ repo xác nhận ngày 2026-09-05. Audit chưa gọi API live, chưa kiểm chứng endpoint/model hoặc dùng account/token thật. Coder dùng cấu hình riêng trên máy, xác nhận request/cost cap trước live run (không dán secrets vào chat), rồi chạy synthetic reasoning replay + paired live benchmark sau khi harness/PR0 đã sửa. Không yêu cầu kết nối/mua provider lại; việc có provider không tự chứng nhận cache improvement hoặc encrypted-item acceptance.
- **Thiếu implementation, không phải thiếu máy:** full executable/REST/SSE/provider E2E, migration preview/accept UI, crash-injection recovery, full ordered reasoning replay/compaction và tracked input-pipeline/Fantasy patch chưa đạt. Không yêu cầu user giải quyết các lỗi code này thay coder; phải implement theo gate.
- Chưa chạy toàn bộ nested Crush/Fantasy `go test ./...`, Windows portable migration UI hoặc verified Windows race suite; focused passes không thay thế chúng. Linux case-sensitive tests nằm ngoài product scope, không cần thực hiện và không giữ release vì thiếu chúng. Không verify CI run/branch protection, không thay CI/settings trong audit.
- Toolchain thực tế trong Gotack là Go **1.27.0** qua auto-toolchain; `go version` ngoài repo báo 1.25.4 không có nghĩa repo không chạy được. SDK local xác nhận summary enum auto/concise/detailed; live model-specific capability vẫn cần gate.

### Điều kiện bàn giao

Plan đã được kiểm chứng ở mức source + existing gates + targeted negative probes; **implementation chưa đạt Definition of Done**. Giữ PR3 off và không phát hành migration/continuity trước khi các blockers tương ứng đóng bằng evidence. Người tiếp theo đọc mục 0 và ledger trước, không tiếp tục từ checkbox “PR0/PR4 done” chưa có proof trong execution plan cũ.
