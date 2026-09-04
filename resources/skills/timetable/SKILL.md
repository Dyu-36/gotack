---
name: timetable
description: "Tạo hoặc điều chỉnh thời khóa biểu trường học từ dữ liệu Excel và yêu cầu người dùng. Dùng khi người dùng yêu cầu tạo TKB, xếp lịch dạy/học, phân công chuyên môn hoặc gửi danh sách giáo viên, lớp, môn. Đọc dữ liệu trực tiếp, dùng OR-Tools CP-SAT để mô hình hóa bài toán hiện tại, kiểm tra đầy đủ hard constraints trước khi giao file. Also use for school timetable or class-scheduling requests in English."
---

# Kỹ năng Xếp Thời Khóa Biểu

## 1. Đọc nguồn và hiểu bài toán

- File người dùng cung cấp và yêu cầu của người dùng là **source of truth**.
- Đọc trực tiếp toàn bộ dữ liệu Excel liên quan, gồm phân công chuyên môn, khung thời gian và các dòng ghi chú/ràng buộc.
- Chuẩn hóa trong quá trình xử lý: tên giáo viên, môn, lớp, số tiết và cách diễn đạt constraint nếu cần.
- Nếu người dùng đã nói rõ dữ liệu đã đúng hoặc đã xác nhận trước đó thì **không hỏi xác nhận lại**. Chỉ hỏi khi có điểm mơ hồ làm thay đổi một hard constraint.
- Có thể tạo file/script tạm trong thư mục làm việc để kiểm tra và chuẩn hóa dữ liệu; không cần tạo `problem.json` hay một schema trung gian.

## 2. Hard constraints và soft constraints

- Mặc định mọi yêu cầu là **hard constraint**, trừ khi người dùng nói rõ là `nên`, `ưu tiên`, `mong muốn` hoặc tương đương.
- Không được tự bỏ, nới lỏng hoặc đổi hard constraint thành soft constraint để tạo được lịch.
- Với constraint gồm nhiều mệnh đề, phải giữ **đầy đủ từng mệnh đề** khi mô hình hóa và khi kiểm tra kết quả.
  - Ví dụ: `T.Hòng dạy từ Thứ 2 đến Thứ 6; riêng Thứ 4 chỉ dạy 2 tiết` nghĩa là vừa phải dạy đủ 5 ngày, vừa phải có **đúng 2 tiết vào Thứ 4**. Không được chỉ kiểm tra điều kiện dạy đủ 5 ngày.
- Trước khi solve, lập một checklist nội bộ các constraint đã đọc được. Mỗi hard constraint trong checklist phải có:
  1. logic tương ứng trong model;
  2. kiểm tra lại sau khi có lời giải.

## 3. Xếp lịch bằng CP-SAT

- Dùng **OR-Tools CP-SAT**. Không tự viết DFS/backtracking cho bài toán xếp lịch.
- Tự viết Python phù hợp trực tiếp với bài toán hiện tại; có thể tạo nhiều script tạm có mục đích rõ ràng như:
  - đọc/kiểm tra input;
  - thử feasibility;
  - solve;
  - validate;
  - ghi Excel.
- Có thể thử nhanh các giả thuyết hoặc phương án bằng một model nhỏ trước khi chạy bản cuối.
- Biểu diễn constraint trực tiếp trong code CP-SAT; không bị giới hạn bởi danh sách `constraint type` cố định.
- Nếu solver trả `INFEASIBLE`, xác định hard constraints xung đột và báo lại cho người dùng. Không tạo lịch giả.
- `FEASIBLE` hoặc `OPTIMAL` chỉ chứng minh các constraint **đã được encode trong model** là thỏa mãn; chưa được phép kết luận "100% đúng" cho đến khi kiểm tra lại toàn bộ checklist nguồn.

## 4. Kiểm tra bắt buộc sau khi solve

Trước khi giao file, đọc lại lời giải và kiểm tra tối thiểu:

- đủ số tiết của từng phân công;
- không trùng lớp cùng một slot;
- không trùng giáo viên cùng một slot;
- từng hard constraint trong checklist nguồn;
- các soft constraint nào không đạt thì phải nêu rõ, không gọi là hard failure.

Validator phải kiểm tra **ý nghĩa gốc** của constraint, không chỉ lặp lại một phiên bản đã bị làm yếu trong model.

Ví dụ: `Không xếp toàn bộ các tiết trong một buổi đều là môn Nặng` không thể kiểm tra đơn giản bằng `số tiết Nặng <= 3` nếu buổi đó chỉ có đúng 3 tiết; trường hợp 3/3 tiết Nặng vẫn vi phạm câu gốc.

Nếu bất kỳ hard constraint nào fail:

- không được báo `100% PASS`;
- không giao file như một lịch hợp lệ;
- sửa model và solve lại, hoặc báo rõ cho người dùng rằng các hard constraints hiện tại không thể đồng thời thỏa mãn.

## 5. Tạo và giao file Excel

- Dùng `<skill_dir>/assets/mau-thoi-khoa-bieu.xlsx` làm template đầu ra khi phù hợp.
- Copy template rồi ghi dữ liệu lịch vào đúng cấu trúc của workbook; không tạo lại format từ đầu nếu không cần.
- Sau khi ghi, mở lại file và kiểm tra dữ liệu thực tế trong workbook trước khi giao.
- Phải lưu/đóng file, đảm bảo file tồn tại và không rỗng.
- Trả link Markdown dạng `[Mở thời khóa biểu](file:///...)` với đường dẫn tuyệt đối và URI-encode ký tự đặc biệt khi cần.
