---
name: timetable
description: "Tạo hoặc điều chỉnh thời khóa biểu trường học; bắt buộc chuẩn hóa và chốt phân công chuyên môn trước khi xếp lịch. Luôn dùng khi người dùng nhắc tạo TKB, xếp lịch học/dạy/tiết, phân công giảng dạy hoặc chuẩn hóa phân công chuyên môn, kể cả khi chỉ gửi dữ liệu giáo viên, môn, lớp. Also use for school timetable or class-scheduling requests in English."
---

# Tạo thời khóa biểu

## Nguyên tắc bắt buộc

Phân công chuyên môn là dữ liệu nền của thời khóa biểu. Với mọi yêu cầu xếp lịch, phải chuẩn hóa phân công, xuất file và được người dùng xác nhận rõ ràng trước khi xếp. Không coi im lặng là xác nhận.

Có hai luồng:

- **Chỉ chuẩn hóa phân công:** chuẩn hóa, giao file, xin xác nhận rồi dừng.
- **Tạo/điều chỉnh thời khóa biểu:** chuẩn hóa, giao file, chốt với người dùng, sau đó mới thu thập/chốt ràng buộc và xếp lịch.

## Giai đoạn 1 — Chuẩn hóa phân công

Chạy giai đoạn này mặc định ngay khi có dữ liệu phân công; không đợi người dùng nói “chuẩn hóa”. Dữ liệu có thể nằm trong tin nhắn hoặc file đính kèm.

1. Đọc tên giáo viên, môn, lớp và số tiết mỗi tuần.
2. Chuẩn hóa khoảng trắng, cách viết tên giáo viên, tên môn và mã lớp.
3. Tách mỗi tổ hợp giáo viên–môn–lớp thành một dòng.
4. Tách ký hiệu lớp gộp theo ngữ cảnh và số tiết nguồn, ví dụ `Toán 7AB = 8` thành `7A = 4`, `7B = 4`. Không chia đều nếu dữ liệu nguồn chỉ ra cách khác.
5. Chuẩn hóa viết tắt chắc chắn như `Văn` → `Ngữ văn`, `C.Nghệ` → `Công nghệ`; không đoán nội dung mơ hồ như `N.thuật` là Âm nhạc hay Mỹ thuật.
6. Kiểm tra dòng trùng, lớp thiếu, số tiết bất thường và đối soát tổng tiết theo giáo viên/lớp.
7. Tạo `<run_dir>/phan-cong-chuan-hoa.xlsx`, mặc định chỉ có sheet `Phân công` với đúng bốn cột:

```text
Tên giáo viên	Môn	Lớp	Số tiết cần dạy trong tuần
```

Dùng fast path cho bảng phẳng: tạo CSV UTF-8 đã chuẩn hóa, `officecli create`, import một lần, đóng/save rồi đọc lại. Không dò Python, không viết script sinh batch và không hỏi help nếu cú pháp đã có trong skill OfficeCLI XLSX.

Nếu import không đọc lại được, không tiếp tục trên file rỗng. Dùng recipe fallback đã kiểm thử với `officecli batch`; không tự mở rộng sang solver hoặc workflow thời khóa biểu.

## Giai đoạn 2 — Giao file và chốt

Trước khi xếp lịch:

1. Xác minh file tồn tại, không rỗng, đúng sheet/cột và đọc lại được vài dòng mẫu.
2. Báo số giáo viên, số lớp, số dòng phân công, tổng số tiết và các mục mơ hồ.
3. Trả link Markdown `file:///` có URI-encoding, ví dụ `[Mở file phân công](file:///C:/duong-dan/phan-cong-chuan-hoa.xlsx)`. Không chỉ in raw path hoặc path trong backtick.
4. Yêu cầu người dùng xác nhận rõ “Dùng phân công này để xếp lịch” hoặc nêu nội dung cần sửa.
5. Nếu cần sửa, cập nhật file, giao lại và xin xác nhận lại. Mọi thay đổi phân công sau khi chốt đều làm trạng thái xác nhận hết hiệu lực.
6. Không chạy xếp lịch khi chưa có xác nhận. Nếu người dùng chỉ yêu cầu chuẩn hóa, kết thúc sau bước giao file/chốt.

## Giai đoạn 3 — Chốt dữ liệu xếp lịch

Chỉ sau khi phân công đã được xác nhận mới chốt các dữ liệu còn lại:

- Khung thời gian: ngày học, buổi học và số tiết.
- Ngày nghỉ hoặc thời gian bận của giáo viên.
- Yêu cầu bắt buộc và mong muốn về vị trí/phân bố tiết.

Có thể ghi nhận dữ liệu người dùng đã cung cấp sớm, nhưng không được chạy solver hoặc tạo lịch trước khi phân công được chốt.

## Giai đoạn 4 — Xếp và xuất thời khóa biểu

1. Dùng file phân công đã chốt làm nguồn sự thật.
2. Tự chọn cách xếp phù hợp; có thể suy luận hoặc viết code tạm trong thư mục làm việc.
3. Kiểm tra đủ số tiết, không trùng lớp và không trùng giáo viên trong cùng thứ, buổi, tiết; không tự bỏ yêu cầu bắt buộc.
4. Copy `<skill_dir>/assets/mau-thoi-khoa-bieu.xlsx` thành `<run_dir>/thoi-khoa-bieu.xlsx`; không sửa mẫu gốc.
5. Ghi lịch vào sheet `Dữ liệu`, mỗi tiết một dòng với sáu cột:
```text
Thứ	Buổi	Tiết	Lớp	Môn	Giáo viên
```

6. Gom phép ghi vào batch tối đa 80 lệnh, đóng file và đọc lại vùng dữ liệu cùng các cột kiểm tra trùng.
7. Chỉ báo thành công khi file tồn tại, không rỗng, đủ tiết, không trùng và sheet `Thời khóa biểu` còn nguyên.
8. Trả link/file attachment mở được; không in toàn bộ lịch dài trong chat nếu không được yêu cầu.

## Quy tắc giao file

Áp dụng cho cả `phan-cong-chuan-hoa.xlsx` và `thoi-khoa-bieu.xlsx`:

- Luôn dùng đường dẫn tuyệt đối trong link `file:///` và URI-encode khoảng trắng/ký tự đặc biệt.
- Phải đóng/save OfficeCLI trước khi giao file.
- Phải đọc lại dữ liệu sau khi đóng.
- Không báo hoàn thành nếu link/attachment không trỏ tới file thật.
- Không dùng lại file kết quả lần trước; tạo run directory riêng cho mỗi lần chạy.
