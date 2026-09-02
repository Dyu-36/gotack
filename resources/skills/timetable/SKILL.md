---
name: timetable
description: "Tạo thời khóa biểu trường học từ khung thời gian, phân công chuyên môn và các yêu cầu sắp xếp của thầy/cô; xuất kết quả ra Excel bằng bộ xếp lịch có sẵn. Dùng khi người dùng yêu cầu tạo TKB, xếp lịch học, xếp lịch dạy hoặc điều chỉnh thời khóa biểu. Không xử lý yêu cầu về phòng học hoặc tài nguyên."
---

# Tạo thời khóa biểu

## Đầu vào

Cần có đủ ba nhóm dữ liệu:

- Khung thời gian: ngày học, các buổi và số tiết mỗi buổi.
- Phân công chuyên môn: giáo viên, môn học, lớp và số tiết mỗi tuần.
- Các yêu cầu bắt buộc hoặc mong muốn ưu tiên của thầy/cô.

Dữ liệu có thể nằm trong cuộc trò chuyện hoặc file đính kèm. Nếu thiếu dữ liệu bắt buộc, chỉ hỏi phần còn thiếu trước khi xếp lịch.

## Phạm vi

Chỉ dùng các loại yêu cầu trong `reference/problem-schema-core.md`.

Không xử lý yêu cầu liên quan đến phòng học, phòng máy, phòng Lab hoặc tài nguyên dùng chung. Khi gặp loại yêu cầu này, dừng trước khi xếp lịch và thông báo rõ rằng chế độ cơ bản chưa hỗ trợ. Không âm thầm bỏ qua yêu cầu.

## Quy trình

1. Đọc dữ liệu và chuẩn hóa tên giáo viên, lớp và môn học.
2. Tóm tắt ngắn số giáo viên, số lớp và tổng số tiết.
3. Tạo một thư mục làm việc riêng cho lần xếp lịch hiện tại.
4. Đọc `reference/problem-schema-core.md` rồi tạo `<run_dir>/problem.json`:
   - Đưa yêu cầu bắt buộc vào `requirements`.
   - Đưa mong muốn ưu tiên vào `preferences`.
   - Chỉ đặt `compact: true` khi người dùng yêu cầu học liền từ tiết đầu và không có tiết trống.
5. Chạy đúng một lệnh:

   ```bash
   python -X utf8 "<skill_dir>/runtime/run.py" "<run_dir>/problem.json" "<run_dir>/thoi-khoa-bieu.xlsx"
   ```

6. Chỉ báo thành công khi lệnh kết thúc thành công và file Excel tồn tại.
7. Trả về bản tóm tắt cùng đường dẫn file Excel. Không in toàn bộ thời khóa biểu dài trong chat, trừ khi người dùng yêu cầu.

Nếu không xếp được lịch, báo các yêu cầu đang xung đột hoặc dữ liệu đang thiếu. Không tự ý bỏ yêu cầu để tạo lịch cho xong.

## Quy tắc thực thi

- Không gọi agent hoặc subagent.
- Không tạo hoặc dùng `constraints_extra.py`.
- Không dùng yêu cầu loại `resource`.
- Không tự viết hoặc sửa chương trình xếp lịch.
- Không tạo Excel bằng công cụ khác.
- Không dùng lại file kết quả của lần chạy trước.
