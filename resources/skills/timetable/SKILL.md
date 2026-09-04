---
name: timetable
description: "Tạo hoặc điều chỉnh thời khóa biểu trường học. Bắt buộc chuẩn hóa phân công chuyên môn ra Excel và chốt thông tin với người dùng trước khi xếp lịch. Dùng khi người dùng yêu cầu tạo TKB, xếp lịch dạy/học, phân công chuyên môn hoặc gửi danh sách giáo viên, lớp, môn. Also use for school timetable or class-scheduling requests in English."
---

# Kỹ năng Xếp Thời Khóa Biểu

Quy trình xếp thời khóa biểu gồm 5 bước tuần tự:

---

## 1: Các loại thông tin cần thu thập
Chủ động thu thập và ghi nhận các thông tin:
1. **Phân công chuyên môn**: Danh sách giáo viên, môn dạy, lớp dạy, số tiết cần dạy trong tuần.
2. **Khung thời gian học**: Các ngày học trong tuần, buổi học (Sáng/Chiều) và số tiết mỗi buổi.
3. **Yêu cầu bắt buộc / Yêu cầu nên có**:
   - Ngày nghỉ cố định, buổi bận của giáo viên.
   - Tiết cố định (Chào cờ, Sinh hoạt lớp...).
   - Ràng buộc môn học (tiết đôi, cách ngày, giới hạn số tiết/buổi...).
*Không cần thu thập theo thứ tự, chỉ cần thu thập đủ thông tin là được*
---

## 2: Chuẩn hóa phân công chuyên môn thành dạng chuẩn bằng file excel
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
Khi xếp lịch, bắt buộc dùng OR-Tools CP-SAT, không tự viết DFS/backtracking: **hard constraints** luôn bắt buộc; **soft constraints** chỉ được đưa vào hàm mục tiêu dưới dạng penalty và không được làm mô hình infeasible, mặc định nếu user không nói gì thì tất cả đều là hard constraints. Nếu solver trả `INFEASIBLE`, dùng assumptions/unsat core để nêu nhóm hard constraints xung đột; nếu trả `FEASIBLE/OPTIMAL`, xác nhận toàn bộ hard constraints đã đạt và liệt kê các soft constraints bị vi phạm cùng penalty.

Chuyển dữ liệu đã chốt thành `problem.json` theo `<skill_dir>/runtime/problem.schema.json`, đặt `output_xlsx`, rồi chạy runner đóng gói bằng `python -u <skill_dir>/runtime/solver.py <problem.json>`. Không tự viết solver thay thế. Chỉ coi `OPTIMAL`, `FEASIBLE`, `INFEASIBLE` là kết quả nghiệp vụ; trạng thái kỹ thuật khác không được suy diễn thành vô nghiệm.

---

## Bước 5: Chuẩn hóa output
Runner chỉ ghi `output_xlsx` sau khi post-validation xác nhận toàn bộ hard constraints. Với `OPTIMAL/FEASIBLE`, dùng chính file Excel runner đã tạo từ `<skill_dir>/assets/mau-thoi-khoa-bieu.xlsx`; không tự ghi lại lịch bằng script khác. Với `INFEASIBLE`, không tạo lịch giả mà nêu nhóm hard constraints đang xung đột bằng mô tả nghiệp vụ. Trả kết quả cho người dùng kèm link Markdown `[Mở thời khóa biểu](file:///...)` khi có file hợp lệ.

---

## Quy tắc giao file chung
- Link file `[Tên file](file:///...)` luôn dùng đường dẫn tuyệt đối và URI-encode khoảng trắng/ký tự đặc biệt.
- Phải lưu/đóng file trước khi giao link và đảm bảo file tồn tại, không rỗng.
