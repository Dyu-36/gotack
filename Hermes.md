# Hermes.md — Port cơ chế học của Hermes Agent vào gotack

Ngày: 2026-09-01 · Loại: khung sườn thi công · Phạm vi: `D:\gotack`

> Bản này thay thế bản nháp trước. Bản trước tự thiết kế một vòng học mới
> (tasklog/outcome/lesson/curator) — sai đề bài. Nhiệm vụ đúng: **copy cơ chế
> của Hermes**, giữ nguyên tên gọi, hằng số và thứ tự của nó, chỉ đổi ngôn ngữ
> và đường ống (Python in-process → Go + REST/SSE + MCP).

## 0. Nguồn tham chiếu — đọc trước khi viết dòng code đầu tiên

Hermes Agent — Nous Research, repo `NousResearch/hermes-agent`.

| Cơ chế cần copy | File trong Hermes | Tài liệu |
| --- | --- | --- |
| Nudge (memory + skill) | `run_agent.go`→`run_agent.py` `run_conversation()` ~L5227–5249 | issue #2227 |
| Đếm turn / đầu mối nudge | `agent/conversation_loop.py` (~L470–478 gọi `_spawn_background_review`) | issue #8506 |
| Sync memory quanh turn | `agent/memory_manager.py` | — |
| Kho memory + cap + scan | `tools/memory_tool.py` (`MemoryStore`) | docs · features/memory |
| Chèn memory vào prompt | `agent/system_prompt.py`, `agent/prompt_builder.py` | docs · features/memory |
| Nạp cấu hình, provider | `agent/agent_init.py` | — |
| Background review fork | `_spawn_background_review`, `agent/auxiliary_client.py` | docs · features/memory |
| Skills + tự vá skill | `skill_manage` tool, `agent/skill_utils.py` | docs · features/skills |
| Recall FTS5 | `session_search` tool, `~/.hermes/state.db` | docs · user-guide/sessions |
| Nén ngữ cảnh | `agent/context_engine.py`, `agent/context_compressor.py`, `agent/prompt_caching.py` | developer-guide |
| Journey UI | `hermes journey` / `/journey` | docs · features/memory |

Repo này đã có sẵn bản khảo sát Hermes: `docs/plans/completed/hermes-parity-harness.md`
(Phase 0–6 đóng 2026-09-01, Phase 7 hoãn). Đọc nó trước, đừng làm lại.

## 1. Cơ chế học của Hermes — đúng như nó chạy

Hermes không có "hệ thống học" riêng. Nó có **4 lớp bộ nhứ** + **2 cú đẩy**:

```
L0  Context files       SOUL.md / AGENTS.md          → nhân cách, luôn trong prompt
L1  Curated memory      MEMORY.md + USER.md          → frozen snapshot đầu session
L2  Session search      state.db (SQLite FTS5)       → gọi khi cần, không tốn prompt
L3  Skills              ~/.hermes/skills/*/SKILL.md  → tri thức thủ tục, tự sinh/tự vá

Đẩy 1: NUDGE          chèn thêm vào user message ngay trong session đang chạy
Đẩy 2: BACKGROUND      fork sau mỗi turn, replay hội thoại, ghi memory/vá skill
        REVIEW
```

Vòng đời một lần học, theo đúng trật tự Hermes:

1. **observe** — turn chạy bình thường, đếm user-turn và đếm tool-iteration.
2. **nudge** — tới ngưỡng thì **dán thêm một câu vào cuối user message** để mô hình
   tự quyết định lưu gì. Không phải một phiên bản hệ thống, chỉ là một câu.
3. **background review** — sau khi turn xong, fork nền `bg-review` replay hội thoại
   với toolset hẹp (memory + skill + đọc file) và tự ghi.
4. **distill** — sau **3+ lần hoàn thành thành công cùng một dạng nhiệm vụ**, sinh
   `SKILL.md` gồm procedure + pitfalls + verification steps (chuẩn agentskills.io).
5. **reuse** — skill hiện diện dạng description nhỏ (~3k token cho **toàn bộ** kho),
   chỉ nạp thân khi khớp việc (progressive disclosure).
6. **refine** — dùng rồi thấy thiếu thì **vá ngay giữa session** bằng `skill_manage`,
   không viết lại cả file.
7. **prune** — `/journey` cho người xem timeline đã học được gì, sửa hoặc xoá.
   Skill bị xoá thì **archive** (phục hồi được), memory chunk thì xoá hẳn.

Hai nguyên tắc thiết kế phải copy nguyên, vì mọi thứ khác dựa vào:

- **Memory không tự nén.** Vượt cap thì **trả lỗi kèm danh sách entry hiện có**
  và bắt mô hệ hợp nhất **ngay trong turn đó** rồi thử lại. Tuyệt đối không
  âm thầm bỏ entry cũ (gotack đang làm sai điểm này — xem §3).
- **Tách "tìm" và "hiểu".** FTS5 tìm rộng và rẻ; LLM chỉ tinh lọc khi cần.
  Không nhét vector DB, không nhét dịch vụ ngoài.

## 2. Hằng số và chuỗi phải copy đúng — không được "cải tiến"

| Thành phần | Giá trị Hermes | Ghi chú |
| --- | --- | --- |
| Cap `MEMORY.md` | 2.200 ký tự (~800 token) | 8–15 entry điển hình |
| Cap `USER.md` | 1.375 ký tự (~500 token) | 5–10 entry điển hình |
| Phân cách entry | `§` | entry được phép nhiều dòng |
| Header khối memory | `MEMORY (your personal notes) [67% — 1,474/2,200 chars]` | mô hình phải thấy % đầy |
| Ngưỡng tự hợp nhất | >80% cap | hợp nhất trước khi add |
| Nudge memory | mỗi **10 user turn** | dán vào user message |
| Nudge skill | sau **10+ tool iteration** trong turn trước | dán vào user message |
| Gợi ý lưu skill (system prompt) | tác vụ **5+ tool call**, lỗi khó, workflow phi tầm thường | `SKILLS_GUIDANCE` |
| Ngưỡng distill thành skill | **3+ lần** hoàn thành thành công cùng pattern | — |
| Ngân sách prompt cho index skill | ~3.000 token cho toàn kho | progressive disclosure |
| Tên thread review | `bg-review` | dùng để chặn đệ quy |
| Dòng log kết thúc review | `Background review complete: thread=bg-review calls=… in=… out=… result=…` | ghi vào log app |
| Ghi nhận chi phí | bảng `session_model_usage`, `task='background_review'` | đo được chi phí học |
| Thông báo chat | `💾 Memory updated`, `💾 Skill 'foo' patched` | mức `off` / `on` / `verbose` |
| Toolset của review | memory + skill-management + đọc file (read-only) | thêm ngoài phải khai báo tên |

Hai chuỗi nudge — copy nguyên văn rồi dịch sang tiếng Việt trong
`internal/userstrings` nếu muốn, nhưng giữ đúng ý và định dạng `[System: …]`:

```text
[System: You've had several exchanges. Consider: has the user shared
preferences, corrected you, or revealed something about their workflow worth
remembering for future sessions?]

[System: The previous task involved many tool calls. Save the approach as a
skill if it's reusable, or update any existing skill you used if it was wrong
or incomplete.]
```

Bài học miễn phí từ lỗi của Hermes (issue #2227): nudge bị **nướng cứng vào
lịch sử hội thoại** và tranh giành chú ý với việc người dùng vừa yêu cầu.
gotack phải copy cả cách tránh: nudge **không được lưu vào lịch sử hiển thị**,
và khi đã có background review thì **không bắn cả hai nudge trong cùng một turn**.

## 3. Đối chiếu Hermes ↔ gotack (tính tới 2026-09-01)

Đã kiểm tra trực tiếp trên cây mã: `internal/memory`, `internal/recall`,
`internal/reflection`, `internal/schedule`, `internal/guard`, `cmd/*`,
`docs/contracts/*`, `resources/skills/*`.

| Cơ chế Hermes | gotack hiện tại | Trạng thái |
| --- | --- | --- |
| L0 context file (SOUL.md) | `resources/context/TACK.md` + `internal/contextseed` | **Xong** |
| L1 memory 2 file + `§` + cap | `internal/memory`, `cmd/memory` | **Xong nhưng lệch** (xem dưới) |
| L2 session search FTS5 | `internal/recall`, `cmd/recall`, `recall.db` | **Xong 1/4 hình thái gọi** |
| L3 skills + progressive disclosure | 12 skill đóng gói, `options.skills_paths` đã merge | **Xong phần đọc** |
| Nudge memory (10 user turn) | không có | **Thiếu** |
| Nudge skill (10+ tool iteration) | không có | **Thiếu** |
| Background review sau turn | `internal/reflection` bắn 1 session riêng, 8 turn/1 lần/giờ | **Gần đúng, sai hình dạng** |
| Toolset hẹp cho review | không có (chỉ ràng bằng prompt) | **Thiếu, Crush chặn** — §4 |
| `skill_manage` (tạo/sửa/vá skill) | không có đường ghi skill nào | **Thiếu — đây là lỗ trống lớn nhất** |
| Đếm pattern → distill sau 3 lần | không có lớp episodic | **Thiếu** |
| `write_approval` + hàng đợi pending | không có | **Thiếu** |
| Thông báo `💾 Memory updated` | không có | **Thiếu** |
| `/journey` timeline + prune | không có UI nào cho phần học | **Thiếu** |
| Nén ngữ cảnh + prompt caching | không có | Hoãn (Phase 7, cần fork) |
| Atropos RL, Honcho, profiles | không có | Không copy — §7 |

Ba chỗ gotack **lệch khỏi Hermes** và phải sửa, không phải thiếu:

1. **Cap xử lý sai.** `internal/memory/store.go` đang **evict oldest-first** khi tràn.
   Hermes **không bao giờ tự bỏ**; nó trả lỗi có `current_entries` + `usage` và bắt
   mô hình hợp nhất ngay trong turn. Evict âm thầm = mất tri thức không dấu vết.
2. **Tool sai hình dạng.** gotack dùng `action` + `section`; Hermes dùng
   `action` + `target` + **`old_text` khớp chuỗi con duy nhất** + `content`,
   **không có action đọc** (memory đã nằm trong prompt). Bỏ `view`, thêm `old_text`.
3. **Thiếu hai hàng rào đầu vào.** Hermes chặn **entry trùng hệt** (trả thành công,
   không ghi) và **quét injection/exfiltration + ký tự Unicode tàng hình** trước khi
   nhận. gotack chưa có cả hai, mà memory ở gotack vào prompt **mọi turn** — rủi ro
   cao hơn Hermes (§4.3).

Kết luận tiến độ: gotack đã có **hạ tầng** (L0–L2 + một nửa L3) nhưng **chưa có
vòng học**. Hai cú đẩy (nudge, background review) và đường ghi skill — tức toàn
bộ phần "auto-enhance" — chưa tồn tại. Ưu tiên theo đúng thứ tự §6.

## 4. Ba ràng buộc của Crush và cách lách (bắt buộc đọc trước khi thiết kế)

gotack không sở hữu vòng lặp agent — Crush sở hữu. Ba điểm sau quyết định hình
dạng của bản port, đều đã xác minh trong `docs/plans/completed/hermes-parity-harness.md`.

**4.1 Không có fork trong session.** Hermes fork luồng `bg-review` ngay trong
process, dùng lại prompt cache nóng. Crush chỉ cho tạo **session mới** qua REST.
→ `bgreview` tạo session riêng, đánh dấu unattended, gửi **digest** (turn gần nhất
nguyên văn + tóm tắt phần cũ) — đúng cách Hermes làm khi review chạy mô hình khác.
Tên session phải mang tiền tố nhận diện để recursion guard bỏ qua `run_complete`
của chính nó (đã có sẵn: `reflection.TitlePrefix`).

**4.2 Không chỉ định được model/toolset theo từng session.** REST chỉ có
`disabled_tools`/`allowed_tools` toàn cục → không thể copy `extra_tools` whitelist
của Hermes theo đúng nghĩa. Thay bằng ba lớp chặn:
prompt ràng buộc → `cmd/guard` chặn theo `CRUSH_SESSION_ID` nằm trong rosơ unattended
→ ngân sách giờ. Ghi chú vào `docs/contracts/gotack-learning.md` là sai lệch có chủ ý.

**4.3 Memory ở gotack KHÔNG phải frozen snapshot.** `prompt.Build` của Crush **đọc
lại mọi context file ở mọi lần dựng prompt**. Hệ quả:

- Ưu: entry ghi ở turn N có hiệu lực ngay turn N+1, không cần đợi session sau.
- Nhược: **vỡ prefix cache** mỗi lần ghi memory — chính thứ Hermes chọn frozen
  snapshot để tránh. Và memory trở thành instruction sống → injection nguy hiểm hơn.

Quyết định (tech lead chốt, không cần hỏi lại): **giữ live, không giả lập frozen.**
Bù hai việc: (a) `bgreview` chỉ ghi **sau khi turn đã xong**, gộp mọi thay đổi
thành **một lần ghi file duy nhất** để chỉ vỡ cache một lần; (b) bật quét injection
ở §5.2 vì đây là trạng thái rủi ro cao hơn Hermes.

## 5. Khung sườn file / folder — từng package làm gì, nối vào đâu

Ký hiệu: **[M]** file mới · **[S]** sửa file đang có. Mọi package đều phải có
`doc.go` một đoạn nói rõ nó copy cơ chế nào của Hermes và file Hermes tương ứng.

### 5.1 `internal/nudge/` — Đẩy 1, copy `run_conversation()` [M]

Rẻ nhất, hiệu lực cao nhất, làm trước tiên.

| File | Trách nhiệm |
| --- | --- |
| `doc.go` | Ghi rõ: copy nudge của Hermes `run_agent.py` L5227–5249. |
| `counter.go` | Theo session: `UserTurns`, `ToolIterationsLastTurn`, `LastMemoryNudgeAt`, `LastSkillNudgeAt`. |
| `rule.go` | `Decide(state) Kind` — `KindMemory` mỗi 10 user turn; `KindSkill` khi turn trước ≥10 tool call; **không bao giờ trả cả hai**, skill ưu tiên trước. |
| `text.go` | Hai chuỗi `[System: …]` ở §2 (lấy qua `internal/userstrings`). |
| `state.go` | Bền hóa `%AppData%\gotack\learning\nudge.json`, atomic temp+rename. **Phải hydrate lại sau restart** — Hermes từng bị bug #8506 vì mất bộ đếm. |
| `counter_test.go` | Bảng: đúng nhịp 10, không bắn trùng, sống qua restart, run lỗi/huỷ không tính. |

Nối dây: `bind_session.go` **[S]** — trước khi gửi prompt, gọi `nudge.Decide` và
nối chuỗi vào **cuối nội dung gửi cho engine**, nhưng **không** đưa vào bản
hiển thị trên UI (tránh lỗi #2227). Nguồn đếm tool call: `App.RunDone` /
sự kiện SSE đang có trong `events.go` **[S]**.

### 5.2 `internal/memory/` — đưa L1 về đúng hình dạng Hermes [S]

| File | Việc phải làm |
| --- | --- |
| `store.go` **[S]** | **Bỏ evict oldest-first.** Tràn cap → trả `ErrOverCap` mang `usage` + `current_entries`. Giữ atomic write + provenance. |
| `errors.go` **[M]** | Sinh đúng thông điệp Hermes: "Memory at X/Y chars… Consolidate now: use 'replace'… then retry this add — all in this turn." |
| `match.go` **[M]** | `old_text` khớp **chuỗi con duy nhất**; khớp 0 hoặc ≥2 → lỗi đòi chuỗi cụ thể hơn. Thay cho `section`. |
| `dedupe.go` **[M]** | Entry trùng hệt → trả `success` + "no duplicate added", không ghi. |
| `scan.go` **[M]** | Quét injection / exfiltration / Unicode tàng hình trước khi nhận; chặn thì nêu tên luật. |
| `render.go` **[M]** | Dựng header `MEMORY (your personal notes) [67% — 1,474/2,200 chars]` + phân cách `§` vào đúng file được seed → mô hình thấy % đầy. |
| `tool.go` **[S]** | Schema mới: `action` (`add`/`replace`/`remove`), `target` (`memory`/`user`), `old_text`, `content`. **Bỏ `view`** — memory đã trong prompt. |

Hệ quả tài liệu: cập nhật `docs/contracts/gotack-memory-mcp.md` **[S]** trong cùng
thay đổi (hard rule 7), và ghi một decision mới
`docs/decisions/0004-memory-refuses-instead-of-evicting.md` **[M]** — vì nó đảo
ngược hành vi đã chốt ở 0003.

### 5.3 `internal/skillmanage/` + `cmd/skills/` — L3 đường ghi skill [M]

Lỗ trống lớn nhất. Hermes coi **skill là cơ chế tự cải thiện chính**, memory chỉ
là nhạc việc. gotack hiện **đọc** được skill nhưng không **viết** được skill nào.

| File | Trách nhiệm |
| --- | --- |
| `internal/skillmanage/doc.go` | Copy tool `skill_manage` của Hermes. |
| `skill.go` | `Skill{Name, Description, Category, Platform, Body, Path, CreatedAt, UpdatedAt, Source}` — frontmatter chuẩn agentskills.io (đã được pin bởi `TestBundledSkillsFrontmatterMatchesEngineContract`). |
| `store.go` | Đọc/ghi `%AppData%\gotack\skills\<category>\<name>\SKILL.md` (đúng layout Hermes, **không** có tầng `learned/`), atomic temp+rename, cho phép thư mục phụ `references/`, `templates/`, `scripts/`, `examples/`, `assets/`. |
| `patch.go` | **Vá giữa session**: thay một đoạn theo `old_string`/`new_string` (đúng tên tham số Hermes; `memory` mới dùng `old_text`), không viết lại cả file. Hermes gọi `patch` là lựa chọn mặc định vì chỉ đoạn thay đổi đi vào tool call. |
| `template.go` | Khung `SKILL.md`: **`## When to Use` → `## Procedure` → `## Pitfalls` → `## Verification`** (đúng 4 mục của Hermes), frontmatter `name`/`description`/`version`/`platforms`/`metadata.gotack.{tags,category}`. |
| `archive.go` | Xoá = chuyển sang `skills\archived\<name>-<ts>\`, phục hồi được (Hermes archive skill, chỉ xoá hẳn memory chunk). |
| `validate.go` | Từ chối skill không có description, **description dài hơn 60 ký tự** (chuẩn viết của Hermes; index skill nằm trong system prompt, khoảng 3k token cho cả danh sách), trùng tên, hoặc chứa lệnh phá huỷ thuộc blocklist của `internal/guard`. |
| `cmd/skills/main.go` | MCP stdio server dùng `internal/mcp`, một tool `skill_manage` với đúng 6 action của Hermes: `create`(`name`,`content`,`category?`), `patch`(`name`,`old_string`,`new_string`) — ưu tiên, `edit`(`name`,`content`), `delete`(`name`), `write_file`(`name`,`file_path`,`file_content`), `remove_file`(`name`,`file_path`). **Không** có action đọc: Hermes đọc skill qua `skills_list()` / `skill_view(name)` / `skill_view(name, path)`, mà `internal/skills` của Crush đã lo (ADR 0001 cấm làm trùng). |

Nối dây: `skills_seed.go` **[M]** trong `package main` đăng ký
`mcp_servers.gotack-skills` (copy y nguyên mẫu `memory_seed.go`, kể cả nhánh
`RemoveConfigField` khi thiếu binary — hard rule 8), và thêm
`%AppData%\gotack\skills` vào `options.skills_paths` (đã có sẵn từ WP8).
`release.yml` / `ci.yml` **[S]**: build + đóng gói `skills.exe`.
Contract mới: `docs/contracts/gotack-skills-mcp.md` **[M]**.

### 5.4 `internal/skillpattern/` — lớp episodic, đếm 3 lần rồi distill [M]

Hermes "tracks multi-step tasks in its episodic memory layer" rồi mới distill.
Đây là phần duy nhất cần sổ ghi, và nó **chỉ** phục vụ ngưỡng 3 lần.

| File | Trách nhiệm |
| --- | --- |
| `signature.go` | Vân tay một nhiệm vụ: chuỗi tên tool đã gọi (đã chuẩn hoá, bỏ tham số) + workspace + trạng thái kết thúc → hash ổn định. |
| `store.go` | SQLite `%AppData%\gotack\learning\patterns.db`: `signature, successes, last_seen, sample_session_ids, distilled_at`. |
| `threshold.go` | `ShouldDistill(sig) bool` — đúng **3 lần thành công**; đã distill thì không bắn lại trừ khi có thêm 3 lần nữa **và** skill đó từng thất bại. |
| `success.go` | Định nghĩa "thành công": `run_complete` không lỗi, không bị huỷ, ≥5 tool call (đúng ngưỡng `SKILLS_GUIDANCE` của Hermes). |
| `prune.go` | Giữ 90 ngày; pattern không tái xuất thì rụng. |

Nguồn dữ liệu: `events.go` / `enginelink` **[S]** — gắn bộ thu tool call theo
session từ SSE đang có, **không thêm đường polling nào** (hard rule 5).

### 5.5 `internal/bgreview/` — Đẩy 2, copy `_spawn_background_review` [M]

Thay thế và hấp thu `internal/reflection` hiện tại. Reflection của gotack đúng
về hạ tầng (gate, ngân sách, chống đệ quy) nhưng sai về **mục đích**: nó "suy ngẫm"
chung chung; Hermes review có **hợp đồng rõ**: hoặc ghi memory, hoặc vá skill,
hoặc không làm gì — và luôn báo lại đã làm gì.

| File | Trách nhiệm |
| --- | --- |
| `doc.go` | Copy background review; nêu rõ sai lệch §4.1/§4.2. |
| `trigger.go` | Chạy sau **mỗi turn** (`run_complete` không lỗi/không huỷ), thay vì ngưỡng 8 turn. Giữ gate session-end. |
| `digest.go` | Dựng digest: turn gần nhất nguyên văn + tóm tắt phần cũ lấy qua `internal/recall`. Giới hạn theo ngân sách token, không dán cả transcript. |
| `prompt.go` | Prompt review: "đã học được gì bền vững?" + bảng quyết định lưu/không lưu của Hermes (Save These / Skip These) + bắt buộc chỉ dùng `memory` và `skill_manage`. |
| `budget.go` | Giữ `DefaultHourlyBudget`, thêm trần ngày và trần % token tháng; bỏ qua khi không có model (preflight, copy `internal/schedule`). |
| `guard.go` | Recursion guard theo tiền tố tiêu đề + rosơ unattended; single in-flight. |
| `usage.go` | Ghi chi phí tách riêng, nhãn `task="background_review"`; log đúng dòng ở §2. |
| `config.go` | `enabled` (mặc định bật), `model` (`auto` = model chính), `extra_tools` (mặc định rỗng). |

`reflection_host.go` **[S]** → đổi tên thành `learning_host.go`, wire
`nudge` + `bgreview` + `skillpattern` vào một chỗ. `internal/reflection` **[S]**:
giữ lại gate/budget/guard đã có test, chuyển phần còn lại sang `bgreview`,
**và bỏ state in-memory** (hiện tại "nothing persists" — mất sạch sau restart).

### 5.6 `internal/recall/` — đủ 4 hình thái gọi của `session_search` [S]

Hermes có 4 cách gọi; gotack mới có 1 (tìm theo từ khoá).

| File | Việc phải làm |
| --- | --- |
| `query.go` **[S]** | Hỗ trợ cú pháp FTS5 nguyên bản: AND mặc định, `"cụm chính xác"`, `OR`/`NOT`, tiền tố `deploy*`. |
| `scroll.go` **[M]** | Hình thái 2: `session_id` + con trỏ → cuộn tiến/lùi trong một session. |
| `browse.go` **[M]** | Hình thái 3: không tham số → danh sách session gần đây (tiêu đề, preview, thời điểm). |
| `sort.go` **[M]** | `sort` = `newest` / `oldest` đặt trên xếp hạng FTS5; mặc định là relevance. |
| `tool.go` **[S]** | Schema `session_search` nhận cả 4 hình thái; giữ `ErrSchemaMismatch`. |

Cập nhật `docs/contracts/gotack-recall-mcp.md` **[S]** trong cùng thay đổi.

### 5.7 `internal/staging/` — duyệt trước khi ghi + thông báo [M]

Hermes cho phép đặt mọi lần ghi memory/skill vào hàng đợi chờ người duyệt. gotack
đã có hạ tầng approval (`internal/permission` + `cmd/guard`, ADR 0002) — tái dùng,
không dựng đường thứ hai.

| File | Trách nhiệm |
| --- | --- |
| `queue.go` | Hàng đợi pending: `%AppData%\gotack\pending\memory\` và `pending\skills\`, mỗi mục một file JSON + diễn giải để hiển thị. |
| `diff.go` | Diff trước/sau cho cả memory entry và skill patch (Hermes có `/skills diff`). |
| `decide.go` | `Approve` → gọi đúng store tương ứng; `Reject` → xoá + ghi lý do. |
| `notify.go` | Phát sự kiện `💾 Memory updated` / `💾 Skill 'x' patched` theo mức `off` / `on` / `verbose`. |

Nối dây: `bind_permission.go` **[S]** mở hai hàm cho UI (`ListPending`,
`DecidePending`); `uievents` **[S]** thêm loại sự kiện thông báo học.
Cờo cấu hình: `memory.write_approval`, `skills.write_approval`,
`display.memory_notifications`.

### 5.8 `internal/journey/` + UI — xem và tỉa thứ đã học [M]

Không có màn này thì vòng học không kiểm soát được — Hermes gọi là `/journey`.

| File | Trách nhiệm |
| --- | --- |
| `internal/journey/timeline.go` | Hợp nhất một dòng thời gian: memory entry + skill đã tạo/vá + pattern đã distill, kèm session sinh ra nó. |
| `internal/journey/mutate.go` | Sửa / xoá từng mục. Memory chunk xoá hẳn; skill chuyển archive (đúng Hermes). |
| `bind_journey.go` **[M]** | Wails binding: `JourneyList`, `JourneyEdit`, `JourneyDelete`, `JourneyRestoreSkill`. |
| `frontend/src/features/journey/` **[M]** | Màn Journey: timeline, thanh dung lượng memory (%), hàng đợi pending, nút duyệt/từ chối. |
| `frontend/src/platform/desktop.ts` **[S]** | Khai báo binding mới — của duy nhất, không gọi `window.go` ở nơi khác (hard rule 4). |

### 5.9 Tài liệu, cấu hình, bố trí dữ liệu

Contract mới **[M]**: `docs/contracts/gotack-nudge.md`,
`gotack-skills-mcp.md`, `gotack-bgreview.md`, `gotack-journey.md`.
Sửa **[S]**: `gotack-memory-mcp.md`, `gotack-recall-mcp.md`,
`gotack-reflection.md` (đổi tên thành `gotack-bgreview.md`, giữ con trỏ chuyển hướng),
`gotack-approvals.md`, `docs/README.md`, `AGENTS.md` (mục package mới).

Khoá cấu hình (đặt trong `internal/appconfig` **[S]**, không nhét vào config Crush):
`learning.nudge.{enabled,memory_every_turns:10,skill_after_tool_calls:10}`,
`learning.bgreview.{enabled,model:"auto",per_turn:true,hourly_budget:1}`,
`learning.pattern.{enabled,distill_after_successes:3}`,
`memory.{char_limit:2200,user_char_limit:1375,write_approval:false}`,
`skills.write_approval:false`, `display.memory_notifications:"on"`.

Bố trí `%AppData%\gotack\` sau khi port:

```
context/TACK.md
context/memory/{MEMORY.md,USER.md}
skills/<category>/<name>/SKILL.md   (layout Hermes, không có tầng learned/)
skills/<category>/<name>/{references,templates,scripts,examples,assets}/
skills/archived/<name>-<ts>/SKILL.md
learning/{nudge.json,patterns.db,journey.json}
pending/{memory/*.json,skills/*.json}
recall/recall.db
```

## 6. Thứ tự thi công — work package cho các đội bên dưới

Mỗi WP là một lần giao độc lập: build xanh, test xanh, contract cập nhật cùng commit.
WP sau **không** được bắt đầu trước khi WP trước đóng, trừ cặp H4/H6 song song được.

| WP | Phạm vi | Phụ thuộc | Định nghĩa xong (DoD) |
| --- | --- | --- | --- |
| **H1** | `internal/nudge` + wire `bind_session.go` | — | Turn 10 có nudge memory; turn sau chuỗi 10+ tool call có nudge skill; nudge không hiện trên UI; sống qua restart; bảng test đủ 4 ca. |
| **H2** | `internal/memory` parity (§5.2) | — | Tràn cap → lỗi có `current_entries`, **không** mất entry cũ; `old_text` khớp chuỗi con duy nhất; trùng bị chặn; injection bị chặn có tên luật; header % xuất hiện trong prompt. Đổi `TestCapEnforcementEvictsOldest` → `TestCapEnforcementRefusesAndReportsUsage` + ADR 0004. |
| **H3** | `internal/bgreview` (hấp thu `reflection`) | H1, H2 | Mỗi turn xong sinh đúng 1 review; guard không đệ quy; ngân sách giớ chặn đúng; dòng log `Background review complete: …` đúng định dạng; chi phí ghi nhãn `background_review`; state còn sau restart. |
| **H4** | `internal/skillmanage` + `cmd/skills` + `skills_seed.go` | H3 | `skill_manage` đủ 6 action Hermes (`create`/`patch`/`edit`/`delete`/`write_file`/`remove_file`) tạo được skill ở `skills\<category>\<name>\SKILL.md`, engine thấy ngay trong turn sau; `patch` (`old_string`/`new_string`) sửa được giữa session; `delete` đưa vào archive và phục hồi được; thiếu binary → gỡ config không làm sập app. |
| **H5** | `internal/skillpattern` | H4 | 3 lần thành công cùng vân tay → bgreview được yêu cầu distill 1 lần duy nhất; lần 4,5 không bắn lại; skill sinh ra có đủ 4 mục `When to Use`/`Procedure`/`Pitfalls`/`Verification` và description ≤60 ký tự. |
| **H6** | `internal/recall` 4 hình thái + `sort` | — | 4 hình thái trả đúng; cú pháp FTS5 (cụm, OR, NOT, tiền tố) chạy; `TestIncrementalSyncByWatermark` và `TestSchemaMismatchSurfacesAsError` vẫn xanh. |
| **H7** | `internal/staging` + notify | H2, H4 | Bật `write_approval` → ghi vào pending chứ không vào file thật; duyệt/từ chối hoạt động; `off/on/verbose` đúng mức ồn. |
| **H8** | `internal/journey` + `bind_journey.go` + màn Journey | H5, H7 | Timeline thấy đủ 3 loại mục; sửa/xoá có hiệu lực; restore skill được; `pnpm check/test/build` xanh. |

Ràng buộc kiến trúc áp cho **mọi** WP (trích `AGENTS.md`, không thương lượng):

1. Không viết logic agent trong gotack — chỉ điều phối qua context file, MCP tool,
   hook, REST (ADR 0001). Nudge là văn bản; bgreview là session; không ai tự gọi LLM.
2. Không sửa `third_party/crush/`, không import `third_party/crush/internal/...`.
3. Không thêm đường polling; dùng SSE đã có.
4. Frontend chỉ đi qua `frontend/src/platform/desktop.ts`.
5. Mọi file < 1000 dòng (`scripts/check-repository-invariants.mjs` sẽ chặn).
6. LF toàn tree; `gofmt -l` rỗng; `go vet`, `staticcheck`, `deadcode -test` sạch.
7. Contract đi cùng commit với code, không để sau.
8. Binary MCP thiếu → gỡ khoá config, không để engine chết.

## 7. Cứ tính — KHÔNG copy

Hermes có nhiều thứ quanh vòng học nhưng không thuộc vòng học. Để ngoài phạm vi,
không bàn lại trong các WP H1–H8:

| Thứ | Lý do bỏ |
| --- | --- |
| Atropos / xuất dữ liệu RL (ShareGPT, RLHF, DPO) | Đó là huấn luyện mô hình, không phải học trong runtime. gotack không sở hữu mô hình. |
| Honcho và 7 memory provider ngoài | Thay thế tầng lưu trữ, không phải cơ chế học. `internal/memory` local đã là provider. |
| Voice mode, profiles, multi-backend terminal (docker/ssh/modal…) | Không liên quan vòng học. |
| `hermes claw migrate`, TUI gateway | Di cư/UI riêng của Hermes. |
| Context compression theo kiểu Hermes (`SUMMARY_PREFIX`, tail cut) | Crush **đã tự nén** trong vòng lặp của nó. Làm thêm là vi phạm ADR 0001. |
| Frozen memory snapshot + prefix cache tuning | Không chạm được `prompt.Build` (§4.3). Đã chốt chịu mất cache. |

## 8. Quyết định đã chốt (không cần hỏi lại)

Tất cả lấy mặc định của Hermes; ai muốn đổi thì mở ADR mới, không đổi trong WP.

1. **Nhịp nudge**: 10 user turn (memory) và 10+ tool iteration (skill) — y nguyên.
2. **Background review chạy sau mỗi turn**, không phải mỗi 8 turn. Ngưỡng 8 của
   `internal/reflection` bị thay; ngân sách giờ vẫn là phảnh an toàn.
3. **Model review**: dùng model đã cấu hình (`auto`), đúng decision WP8. Khi defect
   **A3** được sửa (`SettingsInfo.SmallModel` hiện bị nhận rồi bỏ) thì mở
   `learning.bgreview.model` trỏ sang model rẻ — đúng ý đồ Hermes (rẻ hơn 3–5×).
4. **Duyệt trước khi ghi**: mặc định **tắt** (Hermes cũng tắt), nhưng phải có đủ
   đường bật ở H7.
5. **Thông báo**: mặc định `on` (một dòng `💾 Memory updated`).
6. **Memory tràn cap**: từ chối + báo số liệu, **không** tự xoá. Đảo ngược hành vi
hiện tại, có ADR 0004 đi kèm.
7. **Skill học được đặt ở user scope** (`%AppData%\gotack\skills\<category>\<name>\`),
   không ghi vào `<workspace>/.agents/skills`. Đúng quy định Hermes: skill do agent
   tạo luôn vào thư mục skill của người dùng, project dir là repo-owned nên
   bảo trì tự động không được sửa.
8. **Ngưỡng distill**: 3 lần thành công cùng vân tay, mỗi lần ≥5 tool call.

---

*Tài liệu này là khung sườn giao việc, không phải exec plan. Exec plan đã mở tại*
*`docs/plans/active/hermes-learning-loop.md` (theo `docs/templates/exec-plan.md`),*
*nơi từng WP ở §6 được trích thành phase H1–H8 có proof và DoD riêng. Khi hai*
*tài liệu lệch nhau, plan là bản đang thực thi; sửa lại tài liệu này cho khớp.*
