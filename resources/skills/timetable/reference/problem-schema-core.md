# Cấu trúc `problem.json` cơ bản

Chỉ dùng tài liệu này khi tạo dữ liệu đầu vào cho `runtime/run.py`.
Chế độ cơ bản không hỗ trợ phòng học, tài nguyên dùng chung hoặc mã mở rộng Python.

## Mục lục

- Cấu trúc gốc
- Bộ chọn phân công và thời gian
- Tám loại yêu cầu bắt buộc
- Mong muốn ưu tiên
- Các loại không được hỗ trợ

## Cấu trúc gốc

```json
{
  "frame": {
    "days": [
      {
        "day": 2,
        "sessions": [
          { "name": "Sáng", "periods": 5 },
          { "name": "Chiều", "periods": 3 }
        ]
      }
    ]
  },
  "assignments": [
    {
      "id": "toan-6a",
      "teacher": "Nguyễn Văn A",
      "class": "6A",
      "subject": "Toán",
      "periods": 4
    }
  ],
  "requirements": [],
  "preferences": [],
  "compact": false
}
```

- `day`: số từ 2 đến 8, tương ứng Thứ 2 đến Chủ nhật.
- `session`: tên buổi phải khớp chính xác với tên trong `frame`.
- `period`: bắt đầu từ 1 trong từng buổi.
- `id` của phân công nên có và phải duy nhất.
- Tên giáo viên, lớp và môn trong các yêu cầu phải khớp chính xác với `assignments`.

## Bộ chọn phân công

```json
{
  "teachers": ["Nguyễn Văn A"],
  "subjects": ["Toán"],
  "classes": ["6A"]
}
```

Có thể dùng một, hai hoặc cả ba khóa. Bỏ khóa nào thì không lọc theo khóa đó.

## Bộ chọn thời gian

```json
{
  "days": [2, 3],
  "sessions": ["Sáng"],
  "periods": [1, 2]
}
```

Các khóa được hỗ trợ:

- `days`: các ngày cần chọn.
- `sessions`: các buổi cần chọn.
- `periods`: các tiết cụ thể.
- `periods_from`: từ tiết này trở đi.
- `from_start`: số tiết đầu buổi.
- `from_end`: số tiết cuối buổi.

Các điều kiện trong cùng một bộ chọn được kết hợp đồng thời.

## Yêu cầu bắt buộc

### 1. Ghim vào tiết cụ thể: `pin`

```json
{
  "id": "R1",
  "name": "Sinh hoạt lớp vào tiết 5 sáng Thứ 6",
  "type": "pin",
  "selector": { "subjects": ["SHL"] },
  "slots": [
    { "day": 6, "session": "Sáng", "period": 5 }
  ],
  "count": 1
}
```

### 2. Cấm một số tiết: `forbid_slots`

```json
{
  "id": "R2",
  "name": "Cô Lan không dạy chiều Thứ 4",
  "type": "forbid_slots",
  "selector": { "teachers": ["Cô Lan"] },
  "slot_selector": { "days": [4], "sessions": ["Chiều"] }
}
```

### 3. Chỉ cho phép trong một số tiết: `allow_slots`

```json
{
  "id": "R3",
  "name": "Thầy Minh chỉ dạy buổi sáng",
  "type": "allow_slots",
  "selector": { "teachers": ["Thầy Minh"] },
  "slot_selector": { "sessions": ["Sáng"] }
}
```

### 4. Giới hạn số tiết mỗi ngày: `per_day_limit`

```json
{
  "id": "R4",
  "name": "Mỗi giáo viên tối đa 4 tiết một ngày",
  "type": "per_day_limit",
  "per": "teacher",
  "max": 4
}
```

`per` nhận `teacher` hoặc `class`. Chỉ dùng một trong `max`, `min`, `exactly`.
Có thể thêm `selector` để chỉ áp dụng cho một nhóm giáo viên, lớp hoặc môn.

### 5. Trải môn qua nhiều ngày: `spread_days`

```json
{
  "id": "R5",
  "name": "Toán 6A học ít nhất 3 ngày",
  "type": "spread_days",
  "selector": { "subjects": ["Toán"], "classes": ["6A"] },
  "min_days": 3
}
```

Dùng một trong `min_days` hoặc `exactly_days`.

### 6. Hai tiết trong cùng ngày phải liền nhau: `same_day_adjacent`

```json
{
  "id": "R6",
  "name": "Nếu Ngữ văn 6A có hai tiết trong ngày thì phải liền nhau",
  "type": "same_day_adjacent",
  "selector": { "subjects": ["Ngữ văn"], "classes": ["6A"] }
}
```

### 7. Không có quá nhiều tiết liên tục: `no_k_consecutive`

```json
{
  "id": "R7",
  "name": "Giáo viên không dạy 5 tiết liền trong một buổi",
  "type": "no_k_consecutive",
  "per": "teacher",
  "k": 5
}
```

Để diễn đạt “tối đa 3 tiết liền”, đặt `k: 4`.

### 8. Lớp bắt buộc có tiết tại một ô lịch: `class_slot_used`

```json
{
  "id": "R8",
  "name": "Lớp 6A phải học tiết 5 sáng Thứ 6",
  "type": "class_slot_used",
  "classes": ["6A"],
  "slot": { "day": 6, "session": "Sáng", "period": 5 }
}
```

Có thể thêm `selector` để giới hạn môn được phép nằm ở ô lịch đó.

## Mong muốn ưu tiên

Mong muốn không bắt buộc đặt trong `preferences`:

```json
{
  "id": "P1",
  "name": "Ưu tiên Toán vào buổi sáng",
  "selector": { "subjects": ["Toán"] },
  "slot_selector": { "sessions": ["Sáng"] },
  "weight": 2,
  "avoid": false
}
```

- `avoid: false`: ưu tiên nằm trong khoảng đã chọn.
- `avoid: true`: ưu tiên tránh khoảng đã chọn.
- `weight`: số nguyên dương; số lớn hơn nghĩa là ưu tiên mạnh hơn.

## Không hỗ trợ trong chế độ cơ bản

Không tạo các loại yêu cầu sau:

- `resource`
- `shared_days`
- `pair_days_disjoint`
- `min_total_in_slots`
- Bất kỳ yêu cầu nào cần `constraints_extra.py`

Khi người dùng yêu cầu phòng học, phòng máy, phòng Lab hoặc tài nguyên dùng chung, dừng và thông báo chưa hỗ trợ thay vì bỏ qua.
