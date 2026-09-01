# Execution Plan: Port cơ chế học của Hermes Agent vào gotack

Date: 2026-09-01

## Status

Active

## Outcome

Khi plan này đóng, các hành vi sau phải **quan sát được** qua giao diện thật
(app đang chạy, file trên đĩa, log runtime) — không phải qua mô tả trong tài liệu:

1. Turn thứ 10 của một session nhận đúng chuỗi nhắc memory của Hermes; turn
   ngay sau một turn có ≥10 tool call nhận đúng chuỗi nhắc skill. Cả hai chuỗi
   **không** xuất hiện trong lịch sử hiển thị cho người dùng.
2. Tool `memory` có đúng 3 action `add` / `replace` / `remove`, khớp `old_text`
   theo chuỗi con duy nhất, không còn action `view`.
3. Memory tràn cap → tool trả lỗi kèm `current_entries` và số liệu dung lượng;
   **không** entry nào bị xoá tự động.
4. Mỗi turn kết thúc bình thường sinh đúng một background review, ghi log
   `Background review complete: thread=bg-review calls=... in=... out=... result=...`
   và ghi chi phí dưới nhãn `background_review`.
5. Agent tự tạo được skill mới tại
   `%AppData%\gotack\skills\<category>\<name>\SKILL.md`, engine thấy skill đó ở
   turn sau, và vá được skill giữa session bằng action `patch`.
6. Cùng một vân tay task thành công 3 lần → review được yêu cầu distill đúng
   một lần; lần thứ 4, 5 không bắn lại.
7. `session_search` chạy được cả 4 hình thái gọi của Hermes kèm `sort`.
8. Bật `memory.write_approval` / `skills.write_approval` → mọi lần ghi vào hàng
   đợi pending, sống qua restart, duyệt / từ chối được từ UI.
9. Màn Journey liệt kê, sửa, xoá được thứ đã học; skill xoá đi phục hồi được;
   memory chunk xoá hẳn.

## Context

### Thẩm quyền cho plan này

- Chỉ thị của người dùng: "copy cơ chế của Hermes", "đừng có sáng tạo". Vì vậy
  **mọi mặc định, hằng số, tên action và chuỗi văn bản đều lấy nguyên từ Hermes
  Agent**. Thứ gì Hermes không có thì không đưa vào plan này (xem Out of scope).
- `Hermes.md` ở gốc repo: bản đối chiếu Hermes → gotack, kèm §0 liệt kê từng cơ
  chế ứng với file nào của Hermes. Plan này là bản thi công của tài liệu đó.
- `docs/plans/completed/hermes-parity-harness.md`: Phase 0–6 đã đóng 2026-09-01,
  Phase 7 hoãn. Decisions WP8 ở plan đó (hoãn skill generation, hoãn `/learn`,
  reflection dùng model đã cấu hình) là tiền đề trực tiếp của plan này.
- `docs/decisions/0001-narrow-no-agent-logic-rule.md`: gotack không viết logic
  agent; chỉ điều phối qua context file, MCP tool, hook, REST.
- `docs/decisions/0002-approval-posture-pretooluse-hook.md`: đường duyệt đã có.
- `docs/decisions/0003-memory-writes-constrained-by-construction.md`: **plan này
  đảo một điểm của nó** (evict oldest-first) nên phải mở ADR 0004.
- Contract liên quan: `docs/contracts/gotack-memory-mcp.md`,
  `gotack-recall-mcp.md`, `gotack-reflection.md`, `gotack-approvals.md`,
  `crush-rest-sse.md`, `wails-bindings.md`.
- `AGENTS.md`: 8 hard rule áp cho mọi phase (nhắc lại ở phần Validation).

### Nguồn Hermes được copy (phải đọc lại trước khi thi công từng phase)

| Cơ chế | Tài liệu / file Hermes |
| --- | --- |
| Nudge memory + nudge skill | `run_agent.py` (vòng `run_conversation`, khối nudge ở khoảng L5227–5249); test `tests/run_agent/test_memory_nudge_counter_hydration.py`; issue #2227 (nudge bị lưu vào lịch sử), #8506 (nudge không bắn) |
| Background review | `agent/conversation_loop.py` `_spawn_background_review` (khoảng L470–478); `agent/auxiliary_client.py`; config `auxiliary.background_review.{enabled,provider,model,extra_tools}` |
| Memory store + tool | `agent/memory_manager.py`, `tools/memory_tool.py` (`MemoryStore`); docs `user-guide/features/memory` |
| Skills + `skill_manage` | docs `user-guide/features/skills`; `agent/skill_utils.py`; `SKILLS_GUIDANCE` trong `agent/system_prompt.py` (issue #429) |
| Session search | docs `user-guide/sessions` (`~/.hermes/state.db`, FTS5) |
| Journey | `hermes journey list|delete|edit` (docs + dashboard Star Map) |

Sự thật cần ghi rõ: **mã nguồn Hermes không có trên máy này** (đã quét `C:\` và
`D:\`, không có checkout). Mọi hằng số trong plan lấy từ tài liệu chính thức của
Hermes và từ §0–§2 của `Hermes.md`. Trước khi code mỗi phase, phải đối chiếu lại
với upstream (`github.com/NousResearch/hermes-agent`) và sửa plan nếu lệch —
không được đoán bù vào chỗ trống.

### Điểm chạm trong gotack

- Đã có, sẽ sửa: `internal/memory`, `internal/recall`, `internal/reflection`,
  `internal/permission`, `internal/appconfig`, `internal/uievents`,
  `internal/userstrings`, `cmd/memory`, `cmd/recall`, `bind_session.go`,
  `events.go`, `memory_seed.go`, `reflection_host.go`, `settings_crush.go`,
  `frontend/src/platform/desktop.ts`.
- Thêm mới: `internal/nudge`, `internal/skillmanage`, `internal/skillpattern`,
  `internal/bgreview`, `internal/staging`, `internal/journey`, `cmd/skills`,
  `skills_seed.go`, `bind_journey.go`, `frontend/src/features/journey/`.
- Seam của Crush được dựa vào (không sửa `third_party/crush/`):
  `options.context_paths`, `options.skills_paths`, `mcp_servers`,
  `hooks.PreToolUse`, REST + SSE, `internal/agent/prompt/prompt.go` (`Build`
  đọc lại mọi context file mỗi lần dựng prompt), `internal/skills`
  (progressive disclosure có sẵn).

## Scope

In scope:

- H1 nudge: hai chuỗi nhắc, hai bộ đếm, bền hóa bộ đếm.
- H2 memory parity: từ chối thay vì evict, `old_text` chuỗi con, chống trùng,
  quét injection, header phần trăm, bỏ action `view`.
- H3 background review: chạy sau mọi turn, ngân sách, chống đệ quy, log, chi phí.
- H4 `skill_manage`: đường ghi skill đầy đủ + MCP server `gotack-skills`.
- H5 distill sau 3 lần thành công cùng vân tay.
- H6 `session_search` đủ 4 hình thái + `sort`.
- H7 duyệt trước khi ghi + thông báo mức `off|on|verbose`.
- H8 màn Journey để xem / sửa / tỉa thứ đã học.
- Tài liệu đi kèm: 4 contract mới, 6 contract sửa, ADR 0004.

Out of scope (Hermes có nhưng **không** copy ở plan này):

- Atropos / xuất dữ liệu RL (ShareGPT, DPO, RLHF) — huấn luyện mô hình.
- Honcho và các memory provider ngoài (Mem0, Supermemory, ByteRover, …).
- Skills Hub, `hermes skills browse|install|publish`, skill bundle, trust
  project-dir, quarantine theo content hash.
- `/learn`, voice mode, profile, multi-backend terminal, `hermes claw migrate`.
- Context compression kiểu Hermes (`SUMMARY_PREFIX`, tail cut) — Crush tự nén;
  làm thêm là vi phạm ADR 0001.
- Frozen memory snapshot + tinh chỉnh prefix cache — không chạm được `prompt.Build`.

## Approach

8 phase, làm tuần tự. Mỗi phase là một lần giao độc lập: code + test + contract
trong cùng commit, build xanh, không để nợ tài liệu. H4 và H6 có thể chạy song
song vì không chạm file chung. Không phase nào được định nghĩa lại hằng số của
Hermes; mọi hằng số đặt một chỗ duy nhất và có trích dẫn nguồn trong `doc.go`.

### H1 — `internal/nudge`: hai cú đẩy của Hermes

Hermes không có bộ điều phối học nào. Toàn bộ phần "tự động" là hai đoạn văn
bản chèn vào prompt theo nhịp đếm. Làm trước vì rẻ nhất và phơi bày ngay
các vấn đề wiring.

File và trách nhiệm: xem `Hermes.md` §5.1. Chỉ nhắc lại phần thi công ở đây.

- `internal/nudge/counter.go`: trạng thái theo session `UserTurns`,
  `ToolIterationsLastTurn`, `LastMemoryNudgeAt`, `LastSkillNudgeAt`.
- `internal/nudge/rule.go`: nudge memory khi `UserTurns % 10 == 0`; nudge skill
  khi turn trước có `ToolIterationsLastTurn >= 10`. Khi cả hai cùng đúng, chỉ
  trả nudge skill (một turn không bao giờ mang hai nudge).
- `internal/nudge/text.go`: hai chuỗi `[System: ...]` đặt nguyên văn, lấy qua
  `internal/userstrings` để không rải chuỗi khắp code.
- `internal/nudge/state.go`: `%AppData%\gotack\learning\nudge.json`, ghi atomic
  (temp + rename), **hydrate khi mở lại session** — đây chính là lỗi Hermes từng
  mắc (#8506) và đã có test riêng ở upstream.
- `bind_session.go`: nối chuỗi nudge vào **cuối nội dung gửi cho engine**, không
  ghi vào bản hiển thị / lịch sử UI (lỗi #2227).
- `events.go`: đếm tool call của turn từ SSE đang có, không thêm đường polling.

Proof: bảng test cho đúng nhịp 10, không bắn trùng trong cùng turn, sống qua
restart, turn lỗi / bị huỷ không tính vào bộ đếm; một lần chạy thật đếm đủ 10
turn và xác nhận UI không hiện chuỗi nudge.

### H2 — `internal/memory`: đưa L1 về đúng hình dạng Hermes

gotack đã có cap đúng (2.200 / 1.375) và dấu `§` đúng, nhưng ba hành vi đang
ngược Hermes. Đây là phase sửa hành vi, không phải thêm tính năng.

1. **Từ chối thay vì evict.** Hermes không bao giờ tự xoá memory của người dùng:
   tràn cap thì tool trả lỗi, kèm phần trăm đang dùng và toàn bộ entry hiện có,
   và yêu cầu mô hình hợp nhất rồi thử lại **trong cùng turn**. gotack hiện
   evict oldest-first — mất dữ liệu âm thầm. Đổi `store.go`, thêm `errors.go`.
2. **`old_text` là chuỗi con duy nhất.** Hermes không có khái niệm "section";
   `replace`/`remove` định vị bằng đoạn văn bản. Khớp 0 hoặc ≥2 → lỗi đòi đoạn
   cụ thể hơn. Thêm `match.go`, đổi schema trong `cmd/memory`.
3. **Chống trùng + quét nội dung.** `dedupe.go`: entry trùng hệt trả thành công
   kèm ghi chú không ghi gì. `scan.go`: chặn prompt injection, lệnh lấy cắp dữ
   liệu, Unicode tàng hình — bắt buộc với gotack hơn cả Hermes vì
   `prompt.Build` của Crush **đọc lại context file mỗi lần dựng prompt**, nên
   memory ở gotack là chỉ thị sống, không phải snapshot đóng băng đầu session.

Thêm `render.go` để file memory mang đúng header
`MEMORY (your personal notes) [67% — 1,474/2,200 chars]` — không có header này
thì mô hình không biết sắp đầy và sẽ không chủ động hợp nhất.

Bỏ action `view` khỏi tool: Hermes không có đường đọc memory vì memory luôn ở
trong prompt; để `view` là mời mô hình đọc lại thứ nó đã thấy.

Proof: đổi `TestCapEnforcementEvictsOldest` thành
`TestCapEnforcementRefusesAndReportsUsage`; thêm test khớp đa vị trí, trùng,
injection bị chặn có tên luật; chạy app thật và xác nhận header % xuất hiện
trong prompt qua `TestPromptRoutesMemoryThroughD3` còn xanh.

### H3 — `internal/bgreview`: cú đẩy thứ hai, hấp thu `internal/reflection`

Hermes chạy một lượt review **sau mỗi turn**, trong một nhánh riêng tên
`bg-review`, với bộ tool bị thu hạp còn memory + skill + đọc file, trên model
rẻ hơn, và chỉ replay một digest chứ không replay cả transcript.

gotack đã có gần đủ hạ tầng cho việc này trong `internal/reflection`
(gate theo ngưỡng turn, ngân sách giờ, chống đệ quy, đặt tiêu đề tiền tố). Ba
điểm phải đổi:

1. **Nhịp**: từ `DefaultTurnThreshold = 8` sang **mọi turn kết thúc bình thường**.
   Ngân sách giờ giữ nguyên làm phảnh an toàn, không phải công tắt.
2. **Hợp đồng đầu ra**: review không "suy ngẫm" chung chung. Nó chỉ được làm
   một trong ba việc: ghi memory, vá/tạo skill, hoặc không làm gì — và luôn báo
   lại đã chọn gì.
3. **Bền hóa trạng thái**: trạng thái reflection hiện nằm trong bộ nhớ, mất
   sạch sau restart. Chuyển sang file dưới `learning/`.

File: `trigger.go`, `digest.go`, `prompt.go`, `budget.go`, `guard.go`,
`usage.go`, `config.go` (chi tiết ở `Hermes.md` §5.5). `reflection_host.go` đổi
tên thành `learning_host.go` và trở thành nơi duy nhất wire nudge + bgreview +
skillpattern.

Hai ràng buộc của Crush phải lách (đã phân tích ở `Hermes.md` §4):

- Không fork được nhánh trong một session → review là **một session riêng** replay
  digest, đặt tiêu đề tiền tố cố định để guard nhận ra.
- REST không cho chọn model theo từng session, và `disabled_tools` /
  `allowed_tools` là toàn cục → không thu hạp được toolset riêng cho review.
  Thay vào đó: prompt nói rõ chỉ dùng `memory` và `skill_manage`, posture duyệt
  chặn phần còn lại (ADR 0002), ngân sách chặn độ dài. Ghi rõ đây là **sai lệch
có chủ đích** so với Hermes, không phải thiếu sót.

Proof: một turn → đúng một review; review không sinh review
(`TestRecursionGuardIgnoresReflectionCompletions` giữ xanh, đổi tên theo package
mới); `TestBudgetCapsFiringsPerHour` giữ xanh; log đúng định dạng một dòng; bản
ghi chi phí có nhãn `background_review` tách khỏi chi phí turn người dùng.

### H4 — `internal/skillmanage` + `cmd/skills`: đường ghi skill

Đây là lỗ trống lớn nhất. Hermes nói thẳng: memory giữ **sự thật nhỏ, luôn
trong context**; skill giữ **quy trình dài, chỉ nạp khi cần**; hai thứ hợp thành
vòng tự cải thiện. gotack đọc được skill nhưng không viết được — tức là thiếu
nửa phần còn lại.

Bộ action phải copy **đúng tên và đúng tham số** của Hermes:

| Action | Dùng khi | Tham số |
| --- | --- | --- |
| `create` | skill mới | `name`, `content` (toàn bộ SKILL.md), `category` (tùy chọn) |
| `patch` | sửa nhỏ — **ưu tiên dùng** | `name`, `old_string`, `new_string` |
| `edit` | viết lại lớn | `name`, `content` |
| `delete` | bỏ hẳn một skill | `name` |
| `write_file` | thêm/sửa file phụ trợ | `name`, `file_path`, `file_content` |
| `remove_file` | bỏ file phụ trợ | `name`, `file_path` |

`patch` được Hermes gọi là lựa chọn mặc định vì chỉ đoạn thay đổi đi vào tool
call — rẻ token hơn `edit`. Không được đổi tên tham số thành `old_text` cho
"giống memory": memory dùng `old_text`, skill dùng `old_string`.

Đường đọc skill thì **không viết lại**: Crush đã có progressive disclosure đúng
ba mức như Hermes (`skills_list()` trả `{name, description, category}` — khoảng
3.000 token; `skill_view(name)` trả toàn bộ; `skill_view(name, path)` trả một
file tham chiếu). `internal/skills` của Crush phụ trách phần này — ADR 0001 cấm
gotack làm trùng.

Bố cục trên đĩa copy Hermes: `<skills-root>/<category>/<name>/SKILL.md`, kèm
các thư mục phụ được phép `references/`, `templates/`, `scripts/`, `examples/`,
`assets/`. Frontmatter theo agentskills.io: `name`, `description`, `version`,
`platforms` (tùy chọn), `metadata.hermes.*` → ở gotack đổi thành
`metadata.gotack.{tags,category}`. Thân bài đúng bốn mục của Hermes:
`## When to Use`, `## Procedure`, `## Pitfalls`, `## Verification`.
Chuẩn viết của Hermes: `description` **≤60 ký tự** — giữ nguyên, vì index
skill nằm trong system prompt.

Viết ở đâu: skill do agent tạo **luôn** đi vào thư mục skill của người dùng
(`%AppData%\gotack\skills\`). Skill nằm trong repo của người dùng
(`<workspace>\.agents\skills`) là **repo-owned**: bảo trì tự động không được
sửa chúng — đúng quy định của Hermes cho project skill và external dir. Sửa
skill đang có thì sửa tại nơi tìm thấy, **trừ** hai vùng repo-owned ở trên.

File: `skill.go`, `store.go`, `patch.go`, `template.go`, `archive.go`,
`validate.go`, `cmd/skills/main.go` (MCP stdio, dùng `internal/mcp`),
`skills_seed.go` đăng ký `mcp_servers.gotack-skills` và gỡ khoá config khi
thiếu binary (hard rule 8, copy nguyên mẫu `memory_seed.go`).

Proof: tạo skill từ một turn thật → file xuất hiện đúng đường dẫn → turn sau
engine liệt kê được skill đó; `patch` đổi đúng một đoạn giữa session;
`delete` đưa vào `archived/` và phục hồi được; xóa binary `skills.exe` rồi mở
app → khoá config bị gỡ, engine vẫn chạy;
`TestBundledSkillsFrontmatterMatchesEngineContract` và
`TestRegisterOfficeToolsAppendsUserAndProjectSkillsDirs` giữ xanh.

### H5 — `internal/skillpattern`: distill sau 3 lần thành công

Hermes không distill ngay lần đầu. Nó theo dấu nhiệm vụ nhiều bước ở tầng
episodic, và khi **cùng một pattern thành công từ 3 lần trở lên** thì mới chứng
cất thành skill. Ba dấu hiệu Hermes dùng để quyết định có đáng ghi không, copy
nguyên vào prompt distill:

- quy trình nhiều bước đáng làm lại;
- đã gặp lỗi / đường cụt rồi tìm ra đường đi được;
- người dùng đã sửa cách làm của agent.

File: `signature.go` (vân tay = chuỗi tên tool đã chuẩn hoá + workspace +
trạng thái kết thúc), `store.go` (SQLite `learning/patterns.db`),
`threshold.go` (đúng 3), `success.go` (turn không lỗi, không huỷ, ≥5 tool call —
đúng ngưỡng `SKILLS_GUIDANCE` của Hermes), `prune.go` (giữ 90 ngày).

Nối vào H3: khi `ShouldDistill` đúng, `bgreview/prompt.go` thêm một đoạn nói rõ
pattern nào đã lặp 3 lần và yêu cầu viết skill theo bốn mục ở H4. **Không**
tự sinh SKILL.md bằng code — vi phạm ADR 0001.

Proof: 3 lần thành công cùng vân tay → đúng một lần yêu cầu distill; lần 4 và 5
không bắn lại; đổi một tool trong chuỗi → vân tay khác, đếm lại từ đầu.

### H6 — `internal/recall`: đủ 4 hình thái gọi của `session_search`

Hermes lưu mọi hội thoại trong `~/.hermes/state.db` (SQLite + FTS5) và mở bốn
cách tra: tìm theo từ khoá, cuộn trong một session theo `session_id`, xem danh
sách session gần đây khi không truyền tham số, và `sort` để đảo thứ tự thời
gian. gotack mới có cách thứ nhất.

- `query.go`: nhận nguyên cú pháp FTS5 — nhiều từ là AND, `"cụm chính xác"`,
  `OR`, `NOT`, tiền tố `deploy*`.
- `scroll.go`: `session_id` + con trỏ → đọc tiến/lùi trong một session.
- `browse.go`: không tham số → danh sách session gần đây kèm tiêu đề, preview,
  thời điểm hoạt động cuối.
- `sort.go`: `newest` / `oldest`, mặc định vẫn là xếp theo độ liên quan.

Phần Hermes có mà gotack **chưa cần**: tiêu đề tự sinh 3–7 từ bằng model phụ,
tiêu đề duy nhất ≤100 ký tự, `parent_session_id` cho chuỗi session sinh ra do
nén context. Ghi vào Out of scope ở trên; chỉ mở khi có yêu cầu riêng.

Proof: bảng test cho bốn hình thái; test cú pháp FTS5;
`TestIncrementalSyncByWatermark`, `TestCrossSessionPersistence`,
`TestSchemaMismatchSurfacesAsError` giữ xanh.

### H7 — `internal/staging`: duyệt trước khi ghi

Hermes mặc định cho agent ghi tự do (`write_approval: false`), nhưng có cỏng
duyệt đầy đủ cho ai muốn: mọi lần ghi bị **stage** thay vì commit, nằm trong
`~/.hermes/pending/skills/`, sống qua restart, và duyệt bằng cùng luồng
approve/deny như lệnh nguy hiểm. Bộ lệnh: `pending`, `diff <id>`,
`approve <id>|all`, `reject <id>|all`, `approval on|off`.

gotack đã có `internal/permission` + `cmd/guard` + ADR 0002 — tái dùng đường đó,
không dựng đường duyệt thứ hai. File: `queue.go` (`pending\memory\`,
`pending\skills\`), `diff.go` (diff hợp nhất để xem trước khi duyệt),
`decide.go`, `notify.go`.

Lưu ý copy đúng: Hermes stage **mọi** lần ghi skill khi cỏng bật, kể cả ghi từ
background review, vì một SKILL.md quá dài để duyệt inline. Còn `guard` nội dung
(quét mẫu nguy hiểm) là cơ chế **độc lập** với cỏng duyệt — hai thứ không thay
nhau, gotack cũng phải giữ tách bạch như vậy.

Thông báo: `display.memory_notifications` với ba mức `off` / `on` / `verbose`;
mức `on` hiện một dòng `💾 Memory updated`.

Proof: bật cỏng → ghi không vào file thật mà vào pending; restart → pending
còn; duyệt → file thật đổi; từ chối → không đổi gì và có lý do;
`TestEvaluateTierMatrix` và `TestDenyReasonNamesTheRule` giữ xanh.

### H8 — `internal/journey` + màn Journey

Không có màn này thì không ai kiểm soát được thứ agent đã học. Hermes mở
`journey list|delete|edit`: skill xoá thì **archive, phục hồi được**, memory
chunk thì **xoá hẳn**. Copy đúng sự bất đối xứng đó.

File: `internal/journey/timeline.go`, `internal/journey/mutate.go`,
`bind_journey.go`, `frontend/src/features/journey/`. Binding khai báo ở
`frontend/src/platform/desktop.ts` — của duy nhất (hard rule 4).

Proof: timeline hiện đủ ba loại mục (memory entry, skill, pattern đã distill);
sửa/xoá có hiệu lực trên đĩa; restore skill được;
`pnpm --dir frontend check`, `test`, `build` xanh.

## Risks And Recovery

- **Mất memory của người dùng khi đổi hành vi cap (H2).** Hạ tầng hiện evict
  oldest-first; đổi sang từ chối mà làm sai có thể cắt entry. Giảm thiểu: sao
  lưu `context\memory\` sang `context\memory\.bak-<ts>\` trước khi đổi, test
  trên bản copy trước. Recovery: khôi phục từ `.bak-<ts>`; hai file là text
  thuần nên chỉnh tay được.
- **Prompt injection qua memory (H2).** `prompt.Build` của Crush đọc lại context
  file mỗi lần dựng prompt, nên memory là chỉ thị sống. Nếu review tự động ghi
  nội dung đọc từ web/repo vào memory thì lần sau nó thành lệnh. Giảm thiểu:
  `scan.go` phải xong **cùng phase** với đường ghi, không hoãn sang sau.
- **Bão chi phí do review mọi turn (H3).** Giảm thiểu: giữ ngân sách giờ đã có,
  thêm trần ngày, digest có giới hạn token, một review in-flight tại một thọi
  điểm. Recovery: `learning.bgreview.enabled: false` tắt ngay, không cần build.
- **Đệ quy review sinh review (H3).** Giảm thiểu: giữ nguyên recursion guard đã
  có test; review chạy dưới tiêu đề tiền tố cố định và cờ unattended.
- **Model yếu ghi rác vào skill (H4, H5).** Giảm thiểu: `skills.write_approval`
  bật được từ H7; `validate.go` từ chối skill thiếu description hoặc
  description quá 60 ký tự; `delete` luôn là archive nên hoàn nguyên được.
- **Làm bẩn repo người dùng (H4).** Giảm thiểu: skill do agent tạo luôn vào
  `%AppData%\gotack\skills\`; `<workspace>\.agents\skills` là repo-owned,
  bảo trì tự động không được chạm.
- **Lệch so với upstream Hermes.** Mã nguồn Hermes không có trên máy; hằng số
  lấy từ tài liệu. Giảm thiểu: mỗi phase mở đầu bằng một lượt đối chiếu
  upstream và cập nhật plan trước khi viết code. Không được đoán bù.
- **Test hiện có pin hành vi cũ.** `TestCapEnforcementEvictsOldest` sẽ đỏ khi đổi
  H2. Đó là thay đổi có chủ đích, phải đi kèm ADR 0004; không được xoá test mà
  không thay bằng test mới kiểm đúng hành vi từ chối.

## Progress

- [ ] H1 `internal/nudge` + wire `bind_session.go` / `events.go`; contract
      `docs/contracts/gotack-nudge.md`.
- [ ] H2 memory parity (`store.go`, `errors.go`, `match.go`, `dedupe.go`,
      `scan.go`, `render.go`, `cmd/memory` schema); ADR 0004; cập nhật
      `gotack-memory-mcp.md`.
- [ ] H3 `internal/bgreview` hấp thu `internal/reflection`; `learning_host.go`;
      contract `gotack-bgreview.md` thay `gotack-reflection.md`.
- [ ] H4 `internal/skillmanage` + `cmd/skills` + `skills_seed.go`; contract
      `gotack-skills-mcp.md`; CI/release đóng gói `skills.exe`.
- [ ] H5 `internal/skillpattern` + đoạn distill trong `bgreview/prompt.go`.
- [ ] H6 `internal/recall` 4 hình thái + `sort`; cập nhật `gotack-recall-mcp.md`.
- [ ] H7 `internal/staging` + notify; cập nhật `gotack-approvals.md`.
- [ ] H8 `internal/journey` + `bind_journey.go` + màn Journey; cập nhật
      `wails-bindings.md` và contract `gotack-journey.md`.
- [ ] Dọn tài liệu cuối: `AGENTS.md`, `docs/README.md`, `Hermes.md` §3 (bảng
      trạng thái), rồi chuyển plan sang `docs/plans/completed/`.

## Decisions

Tất cả lấy mặc định của Hermes. Thứ gì Hermes không nói rõ thì không chốt ở đây —
để lại cho phase tương ứng đối chiếu upstream rồi mới ghi.

- 2026-09-01: Thẩm quyền cho plan này là chỉ thị "copy cơ chế Hermes, không sáng
  tạo". Mọi hằng số và tên action đều là hằng số của Hermes, không phải lụa
  chọn thiết kế của gotack; vì vậy không dừng để hỏi policy.
- 2026-09-01: Nhịp nudge giữ nguyên 10 user turn (memory) và 10+ tool iteration
  (skill).
- 2026-09-01: Background review chạy **sau mỗi turn**, thay ngưỡng 8 turn của
  `internal/reflection`. Ngân sách giờ là phảnh an toàn, không phải nhịp.
- 2026-09-01: Review dùng model đã cấu hình (`model: "auto"`), đúng decision WP8
  của plan parity. Chỉ trỏ sang model rẻ khi defect A3 (`SettingsInfo.SmallModel`
  nhận rồi bỏ, `settings_crush.go` pin `models.large` = `models.small`) được sửa.
- 2026-09-01: Không thu hạp toolset riêng cho review vì REST chỉ có
  `disabled_tools`/`allowed_tools` toàn cục. Chặn bằng prompt + posture duyệt +
  ngân sách. Đây là sai lệch có chủ đích so với Hermes, phải ghi trong
  `bgreview/doc.go`.
- 2026-09-01: Memory ở gotack là **live**, không phải frozen snapshot như Hermes,
  vì `prompt.Build` đọc lại context file mỗi lần. Chấp nhận mất prefix cache;
  đánh đổi là `scan.go` thành bắt buộc.
- 2026-09-01: Memory tràn cap thì từ chối kèm số liệu, không tự xoá. Đảo một
  điểm của ADR 0003 → phải mở `0004-memory-refuses-instead-of-evicting.md`.
- 2026-09-01: `memory` dùng `old_text`; `skill_manage` dùng
  `old_string`/`new_string`. Không hợp nhất hai tên cho "đỡ lạ" — giữ đúng
  upstream.
- 2026-09-01: Skill do agent tạo đặt ở user scope `%AppData%\gotack\skills\`;
  `<workspace>\.agents\skills` là repo-owned và bảo trì tự động không sửa.
- 2026-09-01: Ngưỡng distill là 3 lần thành công cùng vân tay, mỗi lần ≥5 tool
  call.
- 2026-09-01: `memory.write_approval` và `skills.write_approval` mặc định
  **tắt** (như Hermes); `display.memory_notifications` mặc định `on`.
- 2026-09-01: Quét nội dung nguy hiểm và cỏng duyệt là hai cơ chế độc lập, không
  thứ nào thay thứ nào.

Promote khi đóng plan: ADR 0004 (memory từ chối thay vì evict) và một ADR cho
ranh giới "agent tự ghi skill" nếu H4 làm thay đổi posture duyệt mặc định.

## Validation

- Focused proof: bảng test cho `nudge` (nhịp, không trùng, hydrate),
  `memory` (từ chối cap, `old_text` đa vị trí, trùng, injection),
  `skillpattern` (ngưỡng 3, đổi vân tay), `recall` (4 hình thái, cú pháp FTS5),
  `skillmanage` (6 action, archive/restore), `staging` (stage, restart, duyệt).
- Integration hoặc end-to-end proof: chạy app thật theo
  `docs/templates/application-runbook.md`; đếm đủ 10 turn để thấy nudge; gây một
  turn ≥10 tool call; để review chạy và đọc log + bản ghi chi phí; tạo và vá
  một skill rồi xác nhận engine thấy ở turn sau; xoá `skills.exe` để chứng minh
  nhánh gỡ config; bật cả hai cỏng duyệt rồi restart.
- Repository-required checks: `gofmt -l` rỗng, `go build ./...`, `go vet ./...`,
  `go test ./...`, `staticcheck ./...`, `deadcode -test ./...`,
  `node scripts/check-repository-invariants.mjs`, `pnpm --dir frontend check`,
  `pnpm --dir frontend test`, `pnpm --dir frontend build`,
  `wails build -platform windows/amd64`, `actionlint`.
- Mọi file mới phải dưới 1000 dòng và dùng LF.

## Result

Chưa có — plan mới mở. Ghi kết quả đã kiểm chứng, giới hạn và việc còn lại ở đây
trước khi chuyển plan sang `docs/plans/completed/`.
