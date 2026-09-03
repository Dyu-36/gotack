---
name: timetable
description: "Tạo hoặc điều chỉnh thời khóa biểu trường học. Bắt buộc chuẩn hóa phân công chuyên môn ra Excel và chốt thông tin với người dùng trước khi xếp lịch. Dùng khi người dùng yêu cầu tạo TKB, xếp lịch dạy/học, phân công chuyên môn hoặc gửi danh sách giáo viên, lớp, môn. Also use for school timetable or class-scheduling requests in English."
---

# Kỹ năng Xếp Thời Khóa Biểu

Quy trình xếp thời khóa biểu gồm 5 bước tuần tự:

---

## Bước 1: Nêu các loại thông tin cần thu thập
Chủ động thu thập và ghi nhận các thông tin:
1. **Phân công chuyên môn**: Danh sách giáo viên, môn dạy, lớp dạy, số tiết cần dạy trong tuần.
2. **Khung thời gian học**: Các ngày học trong tuần, buổi học (Sáng/Chiều) và số tiết mỗi buổi.
3. **Yêu cầu bắt buộc / Yêu cầu nên có**:
   - Ngày nghỉ cố định, buổi bận của giáo viên.
   - Tiết cố định (Chào cờ, Sinh hoạt lớp...).
   - Ràng buộc môn học (tiết đôi, cách ngày, giới hạn số tiết/buổi...).

---

## Bước 2: Chuẩn hóa phân công chuyên môn thành dạng chuẩn bằng file excel
Chuẩn hóa thông tin vào file excel `<run_dir>/phan-cong-chuan-hoa.xlsx` (sheet `Phân công`) theo cấu trúc:

```text
Tên giáo viên | Môn | Lớp | Số tiết cần dạy trong tuần
```

- Tách rõ các lớp gộp (ví dụ: `7AB` → `7A`, `7B`).
- Chuẩn hóa tên giáo viên, môn học và mã lớp.

---

## Bước 3: Chốt thông tin với user trước khi xếp lịch
⚠️ **Điểm dừng bắt buộc**: Dừng lại xin xác nhận từ người dùng trước khi xếp lịch. Nội dung trả về lúc chốt gồm:
1. **Link phân công chuyên môn**: Trả link Markdown dạng URI `[Mở file phân công](file:///...)` mở được trực tiếp.
2. **Các yêu cầu sẽ dùng để xếp**: Tóm tắt khung thời gian học cùng các yêu cầu bắt buộc / yêu cầu nên có đã thu thập.

*Chỉ tiến hành xếp lịch khi người dùng đã xác nhận đồng ý.*

---

## Bước 4: Xếp lịch
Sử dụng dữ liệu đã chốt để xếp lịch, bắt buộc tuân thủ các ràng buộc cứng:
- **Không trùng giáo viên**: Cùng một tiết, một giáo viên không dạy nhiều lớp.
- **Không trùng lớp**: Cùng một tiết, một lớp không học nhiều môn.
- **Đúng số tiết**: Xếp đủ số tiết theo phân công chuyên môn đã chốt.
- Tối ưu các yêu cầu bắt buộc và yêu cầu nên có đã thống nhất.

---

## Bước 5: Chuẩn hóa output
1. Copy file mẫu `<skill_dir>/assets/mau-thoi-khoa-bieu.xlsx` sang `<run_dir>/thoi-khoa-bieu.xlsx` (không sửa file mẫu).
2. Điền dữ liệu lịch đã xếp vào sheet `Dữ liệu` theo đúng 6 cột:
   ```text
   Thứ | Buổi | Tiết | Lớp | Môn | Giáo viên
   ```
   *(Sheet `Thời khóa biểu` tự động cập nhật bảng hiển thị theo công thức).*
3. Kiểm tra tính toàn vẹn của file (đủ tiết, không trùng, không ô lỗi) và đóng/lưu file hoàn tất.
4. Trả kết quả cho người dùng kèm link Markdown `[Mở thời khóa biểu](file:///...)`.

---

## Quy tắc giao file chung
- Link file `[Tên file](file:///...)` luôn dùng đường dẫn tuyệt đối và URI-encode khoảng trắng/ký tự đặc biệt.
- Phải lưu/đóng file trước khi giao link và đảm bảo file tồn tại, không rỗng.
