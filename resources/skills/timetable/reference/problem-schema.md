# Schema `problem.json` và API mở rộng

Tài liệu này mô tả đầu vào duy nhất của `runtime/solver.py`. Khi điền, đối chiếu
từng yêu cầu của thầy/cô với đúng một mục dưới đây. Nếu một yêu cầu **không**
diễn đạt được bằng bất kỳ loại nào → viết hàm plugin trong
`constraints_extra.py` (mục "API mở rộng" cuối tài liệu). Tuyệt đối không bỏ
 silently yêu cầu nào: solver kiểm tra chặt key lạ và loại lạ, thiếu gì báo đó.

## Quy ước

- **Slot** là một ô lịch: `{day, session, period}` với `day` là số 2–8
  (2 = Thứ 2 … 8 = Chủ nhật), `session` là tên buổi ("Sáng", "Chiều"…),
  `period` là tiết thứ mấy trong buổi (bắt đầu từ 1).
- Mọi khóa lạ đều bị từ chối với thông báo rõ ràng — dùng để tự bắt lỗi điền.

## Cấu trúc gốc

```json
{
  "frame":        { "days": [ { "day": 2, "sessions": [ { "name": "Sáng", "periods": 5 } ] } ] },
  "assignments":  [ { "id": "a1", "teacher": "Nguyễn Văn A", "class": "6A", "subject": "Toán", "periods": 4 } ],
  "requirements": [ ... ],
  "preferences":  [ ... ],
  "solver":       { "phase_a_seconds": 20, "phase_b_seconds": 15, "workers": 8, "random_seed": 7 },
  "compact": false
}
```

- `assignments`: mỗi dòng là một phân công giáo viên–lớp–môn với tổng số
  tiết/tuần. `id` tùy chọn (tự sinh nếu thiếu).
- `compact` (mặc định `false`): nếu thầy/cô yêu cầu **lớp học liền mạch từ
  tiết 1, không trống tiết giữa buổi** thì đặt `true`. Lưu ý: nhiều trường
  thực tế chấp nhận trống giữa buổi — chỉ bật khi được yêu cầu, bật oan sẽ
  làm bài toán chật hoặc vô nghiệm.
- `solver`: cấu hình tùy chọn cho bộ giải:
  - `phase_a_seconds` (mặc định `20`s, trần cứng `60`s): thời gian tối đa cho pha A (tìm nghiệm đầu).
  - `phase_b_seconds` (mặc định `15`s, trần cứng `15`s): thời gian tối đa cho pha B (tối ưu hóa "đẹp", có dừng sớm).
  - `workers` (mặc định `min(os.cpu_count(), 8)`): số luồng giải song song.
  - `random_seed` (mặc định `7`): seed ngẫu nhiên để tái lặp kết quả.
- Ràng buộc lõi luôn bật (không cần khai báo): đúng & đủ số tiết theo phân
  công; mỗi lớp / mỗi giáo viên tối đa 1 tiết trong một slot.

## Tùy chọn dòng lệnh (CLI options)

Solver hỗ trợ các cờ bổ trợ khi chạy từ dòng lệnh:
```bash
python -X utf8 runtime/solver.py <problem.json> <schedule.json> [options]
```
- `--phase-a-only`: Chỉ chạy Pha A tìm nghiệm hợp lệ rồi lưu kết quả ngay (bỏ qua Pha B).
- `--time-budget <seconds>`: Chia tổng ngân sách thời gian (70% pha A, 30% pha B; nếu đi kèm `--phase-a-only` thì dồn 100% cho pha A). Cờ này **không bị áp trần** 60s/15s như giá trị khai trong `problem.json`.
- `--diagnose`: Chạy chẩn đoán nhóm yêu cầu mâu thuẫn (conflict assumptions) khi bài toán vô nghiệm.
- `--verbose`: In chi tiết log tìm kiếm của solver và hiển thị toàn bộ bảng kiểm PASS/FAIL.

## Bộ chọn

**LessonSelector** — chọn tập phân công (`selector`):

| Khóa | Kiểu | Ghi chú |
|---|---|---|
| `teachers` | [tên] | so khớp chính xác tên giáo viên |
| `subjects` | [môn] | so khớp chính xác tên môn |
| `classes` | [lớp] | so khớp chính xác tên lớp |

Bỏ trống khóa nào = không lọc theo khóa đó; cả ba bỏ trống = tất cả.

**SlotSelector** — chọn tập slot (`slot_selector`):

| Khóa | Kiểu | Ví dụ |
|---|---|---|
| `days` | [số] | `[3, 4, 5]` = T3–T5 |
| `sessions` | [buổi] | `["Chiều"]` |
| `periods` | [tiết] | `[1]` = tiết đầu buổi |
| `periods_from` | n | từ tiết n trở đi trong buổi |
| `from_start` | n | n tiết đầu buổi |
| `from_end` | n | n tiết cuối buổi |

Các điều kiện trong một dict kết hợp giao nhau (AND). Cần **hợp (OR)** — ví dụ
"chiều hoặc sáng từ tiết 3" — truyền `slot_selector` là **danh sách các dict**:

```json
"slot_selector": [
  {"sessions": ["Chiều"]},
  {"sessions": ["Sáng"], "periods_from": 3}
]
```

## 12 loại yêu cầu (`requirements`)

Mỗi yêu cầu có `id`, `name` (hiển thị), `type`, và tham số theo loại. Mọi yêu
cầu đều có công tắc bật/tắt bên trong solver: khi bài toán vô nghiệm và bật cờ
`--diagnose`, solver in ra đúng nhóm yêu cầu mâu thuẫn theo `id`/`name` để đọc cho thầy/cô hiểu.

### 1. `pin` — ghim vào slot cụ thể

```json
{ "id": "H04", "name": "SHL toàn trường T6-Sáng-5", "type": "pin",
  "selector": { "subjects": ["SHL"] },
  "slots": [{ "day": 6, "session": "Sáng", "period": 5 }], "count": 1 }
```

Mỗi phân công khớp selector đặt đúng `count` tiết vào danh sách `slots`.

### 2. `forbid_slots` / 3. `allow_slots` — cấm / chỉ cho phép

```json
{ "id": "H08a", "name": "Tiếng Anh không vào tiết 1 sáng", "type": "forbid_slots",
  "selector": { "subjects": ["Tiếng Anh"] },
  "slot_selector": { "sessions": ["Sáng"], "periods": [1] } }
```

`allow_slots` = mọi tiết của selection phải nằm trong tập cho phép (dùng cho
lịch bận giáo viên, cửa sổ phòng học…). Có thể truyền `slots` liệt kê thay cho
`slot_selector`.

### 4. `per_day_limit` — giới hạn số tiết mỗi ngày

```json
{ "id": "H16a", "name": "Mỗi GV tối đa 5 tiết/ngày", "type": "per_day_limit",
  "per": "teacher", "max": 5 }
```

`per`: `"class"` hoặc `"teacher"`. Một trong `max` / `min` / `exactly`.
Ví dụ "GDTC + Nghệ thuật không cùng ngày": selector hai môn, `per: "class"`,
`max: 1`.

### 5. `spread_days` — trải đều ra số ngày

```json
{ "id": "H06", "name": "Toán 4 tiết trải đúng 3 ngày", "type": "spread_days",
  "selector": { "subjects": ["Toán"] }, "exactly_days": 3 }
```

`min_days` hoặc `exactly_days`; tùy chọn `slot_selector` để chỉ đếm ngày theo
cửa sổ (ví dụ HĐTN: 2 tiết chiều ở 2 ngày khác nhau →
`{"sessions": ["Chiều"]}, "min_days": 2`).

### 6. `same_day_adjacent` — cùng ngày phải liền kề cùng buổi

```json
{ "id": "H06b", "name": "Nếu Toán có 2 tiết/ngày thì phải là tiết đôi", "type": "same_day_adjacent",
  "selector": { "subjects": ["Toán"] } }
```

### 7. `shared_days` — hai lớp trùng nhịp học môn

```json
{ "id": "H17", "name": "Toán A-B cùng khối trùng ≥ 2 ngày", "type": "shared_days",
  "selector": { "subjects": ["Toán"] },
  "pairs": [["6A","6B"], ["7A","7B"], ["8A","8B"], ["9A","9B"]], "min_days": 2 }
```

### 8. `pair_days_disjoint` — ngày tiết đôi của hai nhóm không trùng

```json
{ "id": "H18a", "name": "Ngày tiết đôi Toán ≠ ngày tiết đôi Văn", "type": "pair_days_disjoint",
  "selector": { "subjects": ["Toán"] }, "selector_b": { "subjects": ["Ngữ văn"] } }
```

### 9. `no_k_consecutive` — không dạy/học k tiết liền trong một buổi

```json
{ "id": "H16b", "name": "GV không dạy 5 tiết liền một buổi", "type": "no_k_consecutive",
  "per": "teacher", "k": 5 }
```

Ví dụ khác: "tối đa 3 tiết môn chính liền" → selector nhóm môn chính,
`per: "class"`, `k: 4`.

### 10. `min_total_in_slots` — tối thiểu N tiết toàn trường trong cửa sổ

```json
{ "id": "H14b", "name": "≥ 6 tiết GDTC cuối buổi sáng", "type": "min_total_in_slots",
  "selector": { "subjects": ["GDTC"] },
  "slot_selector": { "sessions": ["Sáng"], "from_end": 1 }, "min": 6 }
```

### 11. `class_slot_used` — lớp bắt buộc có tiết tại một slot

```json
{ "id": "H19a", "name": "Lớp tải 30 không trống T6-Sáng-5", "type": "class_slot_used",
  "classes": ["6A", "6B"], "slot": { "day": 6, "session": "Sáng", "period": 5 } }
```

### 12. `resource` — phòng/tài nguyên hạn chế (Lab, phòng máy…)

```json
{ "id": "H10b", "name": "Mỗi lớp 1 tiết KHTN thực hành ở Lab (T3-T5), không kề GDTC/Nghệ thuật",
  "type": "resource", "resource": "Lab",
  "selector": { "subjects": ["KHTN"] }, "per_class_count": 1,
  "slot_selector": { "days": [3, 4, 5] }, "capacity": 1,
  "exclude_adjacent": { "subjects": ["GDTC", "Nghệ thuật"] } }
```

- `per_class_count`: mỗi lớp dùng đúng n tiết; bỏ qua và đặt
  `"per_class_all": true`-style bằng cách chỉ định `per_class_count` bằng số
  tiết của môn khi cả môn phải ở phòng (ví dụ toàn bộ Tin học ở phòng máy).
- `capacity`: tối đa bao nhiêu lớp cùng dùng tài nguyên trong một slot.
- `exclude_adjacent`: tiết dùng tài nguyên không được kề môn thuộc selection
  này của cùng lớp.
- Tiết được đánh dấu ghi vào `labels` của assignment trong schedule.json.

## Ưu tiên mềm (`preferences`) — chỉ ảnh hưởng pha tinh chỉnh

```json
{ "id": "S02", "name": "Tránh Toán tiết cuối buổi", "selector": { "subjects": ["Toán"] },
  "slot_selector": { "from_end": 1 }, "weight": 2, "avoid": true }
```

`avoid: true` → phạt khi rơi vào cửa sổ; mặc định (`false`) → ưu tiên nằm trong
cửa sổ. Solver luôn tự tối ưu sẵn hai tiêu chí: không dồn một môn nhiều tiết
vào một ngày (trọng số 3), giảm giờ trống giữa các tiết của giáo viên trong
cùng buổi (trọng số 2). Pha tinh chỉnh có trần thời gian ngắn và tự dừng sớm khi không cải thiện — lịch hợp lệ từ pha 1 luôn được ghi nhận an toàn ngay sau khi tìm thấy.

## API mở rộng: `constraints_extra.py`

Đặt file này **cạnh problem.json**. Solver tự nạp nếu tồn tại. Mỗi yêu cầu
đặc thù là một hàm dựng (chạy trong solver) + một hàm kiểm tra (chấm trên lịch
kết quả), được gắn công tắc như yêu cầu thường:

```python
# -*- coding: utf-8 -*-
"""Yêu cầu ngoài schema: Toán 6B không ở tiết cuối ngày T4."""


def register(requirement):
    def build(api):
        last_t4 = api.slot_keys({"days": [4], "from_end": 1})
        api.add_under(api.occ({"classes": ["6B"], "subjects": ["Toán"]}, last_t4) == 0)

    def check(capi):
        last_t4 = set(capi.slot_keys({"days": [4], "from_end": 1}))
        bad = capi.placed_of({"classes": ["6B"], "subjects": ["Toán"]}, last_t4)
        return (len(bad) == 0, f"còn {len(bad)} tiết Toán 6B ở tiết cuối T4")

    requirement(name="Toán 6B không tiết cuối T4", build=build, check=check)
```

API cho `build(api)`:

| Hàng mục | Dùng |
|---|---|
| `api.model` | mô hình CP-SAT gốc (ortools) |
| `api.rows(selector)` | danh sách phân công khớp selector |
| `api.slot_keys(slot_selector)` | danh sách slot khớp bộ chọn |
| `api.x(row_id, slot)` | biến nhị phân "phân công này ngồi slot này" |
| `api.occ(selector, slots)` | tổng số tiết của selection trong các slot |
| `api.add_hard(expr)` | ràng buộc bắt buộc (không kèm công tắc) |
| `api.add_under(expr)` | ràng buộc bắt buộc **kèm công tắc** của yêu cầu này (nên dùng) |
| `api.add_penalty(expr, weight)` | thêm hạng phạt cho pha tinh chỉnh |
| `api.new_bool(name)` | tạo biến nhị phân phụ |

API cho `check(capi)`: `capi.placed` (mọi tiết đã xếp), `capi.rows(selector)`,
`capi.slot_keys(selector)`, `capi.placed_of(selector, slots)`. Trả về
`(bool, chi_tiết_lỗi_không_pass)` — chi tiết chỉ hiển thị khi FAIL.

## Đọc kết quả

- Mã thoát `0`: thành công; tổng kê in ra màn hình gồm số GV, số lớp, số tiết, số yêu cầu PASS và thời gian từng pha; lịch ghi vào file out theo cấu trúc
  `classes` / `days` / `assignments` (đúng đầu vào của `runtime/exporter.py`).
- Mã `2`: vô nghiệm hoặc quá trần thời gian — khi chạy kèm `--diagnose`, màn hình chỉ rõ **nhóm yêu cầu
  mâu thuẫn** (kèm `id`/`name`) để đọc cho thầy/cô và đề nghị nới yêu cầu.
- Mã `3`: problem.json lỗi (key lạ, thiếu trường) — sửa theo thông báo.
- Mã `4`: thiếu thư viện ortools → `pip install ortools`.
- Mã `5`: plugin `constraints_extra.py` lỗi — xem traceback.
- Mã `6`: lịch tìm được nhưng tự kiểm tra FAIL (không nên xảy ra; báo lại).
