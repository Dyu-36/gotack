---
name: timetable
description: "Tạo hoặc điều chỉnh thời khóa biểu trường học từ dữ liệu và yêu cầu tự nhiên; tự chọn cách xếp lịch rồi điền kết quả vào mẫu Excel định dạng sẵn. Luôn dùng skill này khi người dùng nhắc tạo TKB, xếp lịch học, xếp lịch dạy, xếp tiết, phân công giảng dạy, chuẩn hóa phân công chuyên môn, kể cả khi không nói chữ 'thời khóa biểu' mà chỉ gửi dữ liệu giáo viên, môn, lớp. Also use for school timetable or class-scheduling requests made in English."
---

# Tạo thời khóa biểu

## Đầu vào

Cần có:

- Khung thời gian: ngày học, buổi học và số tiết.
- Phân công: giáo viên, môn, lớp và số tiết mỗi tuần.
- Các yêu cầu bắt buộc hoặc mong muốn của người dùng.

Dữ liệu có thể nằm trong tin nhắn hoặc file đính kèm. Nếu thiếu dữ liệu bắt buộc, hỏi trực tiếp đúng phần còn thiếu rồi chờ người dùng trả lời.

## Đầu ra

Luôn trả một file Excel `.xlsx` dựa trên mẫu ví dụ:

```text
<skill_dir>/assets/mau-thoi-khoa-bieu.xlsx
```
File này chỉ mang tính chất tham khảo, có thể thêm cột / hàng tùy theo input.

Mẫu có hai sheet:

- `Dữ liệu`: vùng duy nhất được ghi kết quả.
- `Thời khóa biểu`: bảng trình bày, tự lấy dữ liệu bằng công thức; không sửa trực tiếp.

## Dữ liệu ghi vào Excel

Chuẩn hóa mỗi tiết thành một dòng với đúng sáu cột:

```text
Thứ	Buổi	Tiết	Lớp	Môn	Giáo viên
```

Ví dụ một dòng: `2	Sáng	1	6A	Toán	Nguyễn Văn A`.

## Quy trình

1. Đọc và chuẩn hóa tên giáo viên, môn và lớp.
2. Tự chọn cách xếp lịch phù hợp. Có thể suy luận trực tiếp hoặc viết code tạm trong thư mục làm việc; không sửa nội dung skill.
3. Trước khi xuất, kiểm tra đủ số tiết, không trùng lớp và không trùng giáo viên trong cùng thứ, buổi, tiết. Không tự bỏ yêu cầu bắt buộc.
4. Tạo thư mục làm việc riêng và copy file mẫu thành `<run_dir>/thoi-khoa-bieu.xlsx`. Không sửa file mẫu gốc.
5. Chuyển các dòng lịch thành các lệnh `set` trong `<run_dir>/lich-batch.json`. Mỗi phần tử ghi một ô `A:F` của sheet `Dữ liệu`; chia thành các batch tối đa 80 lệnh.
6. Chạy lần lượt từng batch, sau đó đóng file để ghi dữ liệu xuống đĩa:

   ```bash
   officecli batch "<run_dir>/thoi-khoa-bieu.xlsx" --input "<run_dir>/lich-batch.json"
   officecli close "<run_dir>/thoi-khoa-bieu.xlsx"
   ```

7. Đọc lại vùng `Dữ liệu!A1:I...`; nếu cột `Trùng lớp` hoặc `Trùng giáo viên` có giá trị đúng thì sửa lịch và ghi lại batch.
8. Chỉ báo thành công khi file Excel tồn tại, không rỗng, dữ liệu đọc lại đúng và sheet `Thời khóa biểu` còn nguyên.
9. Trả file Excel cho người dùng; không in toàn bộ lịch dài trong chat trừ khi được yêu cầu.

## Quy tắc

- Dùng lệnh `officecli` trong shell.
- Gom mọi phép ghi vào `officecli batch`, không gọi `set` từng ô riêng lẻ: mỗi ô một lệnh là một round-trip shell riêng, hàng trăm ô sẽ rất chậm và dễ làm file hỏng giữa chừng.
- Mỗi lần chạy copy lại từ file mẫu, không dùng lại file kết quả của lần chạy trước: dữ liệu sót lại có thể nằm ngoài vùng bị ghi đè và tạo ra tiết "ma" trong lịch mới.
- Không sửa file mẫu gốc trong skill: đây là asset dùng chung cho mọi lần chạy.
