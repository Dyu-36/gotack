---
name: timetable
description: "Tạo, kiểm tra hoặc điều chỉnh thời khóa biểu trường học từ Excel và yêu cầu người dùng. Dùng khi yêu cầu tạo TKB, xếp lịch dạy/học, phân công chuyên môn, chuẩn hóa bảng phân công hoặc cung cấp giáo viên, lớp, môn để lập lịch. Dùng Python đi kèm Gotack, OR-Tools CP-SAT và hai mẫu Excel của skill; kiểm tra độc lập mọi ràng buộc bắt buộc trước khi giao file. Also use for school timetable, teaching allocation and class-scheduling requests in English."
---

# Xếp thời khóa biểu

## 1. Chọn môi trường chạy

Trên bản Windows đóng gói, chạy Python bằng đường dẫn tuyệt đối trong biến môi trường `GOTACK_PYTHON`, không thay Python của hệ thống hoặc của project:

```powershell
$python = $env:GOTACK_PYTHON
if (-not $python -or -not (Test-Path -LiteralPath $python -PathType Leaf)) {
    throw 'Không tìm thấy Python đi kèm Gotack. Kiểm tra bộ cài hoặc cấu hình GOTACK_PYTHON khi chạy bản phát triển.'
}
& $python -I -c 'import openpyxl, defusedxml; from ortools.sat.python import cp_model; print("timetable runtime ready")'
if ($LASTEXITCODE -ne 0) { throw 'Runtime thời khóa biểu không hợp lệ; không tiếp tục với dữ liệu chưa xử lý.' }
```

Chạy các script xử lý bằng `& $python -I <đường-dẫn-script> ...`. Dùng đối số đường dẫn tuyệt đối; không dựa vào Python tìm thấy qua `PATH`. Không cài package vào môi trường người dùng, không tải mã nguồn solver từ website. Runtime đã có OR-Tools, openpyxl và defusedxml. Hai file mẫu nằm trong `assets/` cạnh `SKILL.md`; xác định `<skill_dir>` từ vị trí skill vừa đọc.

## 2. Đọc dữ liệu nguồn

- Lấy file nguồn và yêu cầu của người dùng làm căn cứ. Đọc tất cả sheet liên quan, ô gộp, phân công chuyên môn, khung thời gian và ghi chú/ràng buộc.
- Không suy ra dữ liệu chỉ từ phần trích dẫn đính kèm. Mở workbook gốc bằng Python; giữ nguyên file nguồn.
- Đối chiếu giá trị công thức với cached values khi cần. Khi cached values thiếu hoặc dữ liệu chưa tính, báo rõ thay vì biến ô trống thành số 0.
- Chuẩn hóa tên giáo viên, môn, lớp và số tiết nhưng không tự thêm giáo viên, lớp, định mức hoặc thời gian còn thiếu.
- Không hỏi lại dữ liệu đã được xác nhận. Chỉ hỏi về thiếu sót làm thay đổi bài toán; nêu rõ mục còn mơ hồ, không âm thầm chọn giả định.
- Tạo một thư mục riêng cho yêu cầu trong `output/`, chứa dữ liệu chuẩn hóa, script giải bài toán và kết quả kiểm tra. Không ghi đè sản phẩm cũ hoặc file nguồn.

## 3. Chuẩn hóa phân công

- Dùng `assets/phan-cong-chuan-hoa.xlsx` làm mẫu khi cần chuẩn hóa. Đầu ra có đúng một sheet `Phân công`, bốn cột `Tên giáo viên | Môn | Lớp | Số tiết`; mỗi dòng là một phân công giáo viên–môn–lớp. Ghi giá trị trực tiếp, không thêm công thức.
- Không mặc định chia đều số tiết khi tách ký hiệu như `7AB`. Xác định số tiết trong nguồn là của từng lớp, tổng hai lớp hay lớp học ghép. Chỉ tách khi ý nghĩa đã rõ; giữ ràng buộc học ghép nếu có.
- Không gộp giáo viên chỉ vì tên viết tắt giống nhau. Mở rộng tên môn khi chắc chắn và ghi lại các chuyển đổi để đối chiếu.
- Khi chỉ được yêu cầu chuẩn hóa, giao file chuẩn hóa và dừng. Khi đã được yêu cầu cả chuẩn hóa và xếp lịch, tiếp tục nếu dữ liệu đủ rõ; không tạo bước xác nhận hình thức.
- Nếu người dùng yêu cầu chốt phân công trước khi xếp, tôn trọng bước chốt đó. Một thay đổi thực chất trong phân công làm mất hiệu lực việc chốt cũ.

## 4. Lập mô hình đầy đủ

- Phân loại mọi yêu cầu: bắt buộc là hard constraint; các từ `nên`, `ưu tiên`, `mong muốn` là soft constraint trừ khi người dùng giải thích khác.
- Không bỏ, nới hoặc biến hard constraint thành soft constraint để lấy được lịch.
- Gán mã cho từng ràng buộc nguồn. Với mỗi mã, lưu nguyên văn, cách diễn giải, logic trong model và kiểm tra độc lập tương ứng. Giữ đủ từng mệnh đề của câu.
- Phân biệt `chỉ được dạy trong Thứ 2–Thứ 6` với `phải có tiết trong cả 5 ngày`; không tự thêm yêu cầu có tiết mỗi ngày. `Đúng 2 tiết` khác `tối đa 2 tiết`.
- Kiểm tra miền dữ liệu trước khi giải: số tiết hợp lệ, khung thời gian đủ, giáo viên/lớp tồn tại, các yêu cầu cố định không tự mâu thuẫn. Báo lỗi dữ liệu riêng với kết luận vô nghiệm.
- Dùng **OR-Tools CP-SAT**; viết model Python phù hợp trực tiếp với bài toán hiện tại. Không chỉ nhờ mô hình ngôn ngữ tự điền lịch.
- Chỉ tối ưu soft constraints sau khi đã biểu diễn đủ hard constraints. Ghi rõ thứ tự ưu tiên và trọng số có ảnh hưởng đến kết quả. Không dùng trọng số để hy sinh hard constraints.

## 5. Giải và diễn giải trạng thái

- Đặt giới hạn tài nguyên có chủ đích, lưu trạng thái solver, objective, best bound và tham số đã dùng. Có thể tiếp tục tìm khi chưa có kết quả; không coi hết thời gian là vô nghiệm.
- `OPTIMAL`: tối ưu cho model đã mã hóa, chưa phải chứng minh model phản ánh đầy đủ yêu cầu nguồn.
- `FEASIBLE`: có lời giải thỏa model, chưa chứng minh tối ưu. Giao lịch hợp lệ sau kiểm tra, nêu đúng mức đảm bảo.
- `UNKNOWN`: chưa xác định được; không nói vô nghiệm và không tạo lịch giả.
- `MODEL_INVALID`: sửa model hoặc dữ liệu, không kết luận yêu cầu người dùng bất khả thi.
- `INFEASIBLE`: solver chứng minh model không có lời giải. Kiểm tra lại cách mã hóa so với nguồn trước khi báo xung đột. Dùng assumptions/unsat core hoặc các phép giải thu gọn để tìm nhóm ràng buộc liên quan; không gọi nhóm đó là nhỏ nhất nếu chưa chứng minh.

## 6. Kiểm tra độc lập

Trước khi giao lịch, đọc lại lời giải và kiểm tra bằng code độc lập với các biểu thức CP-SAT:

- Đủ số tiết của từng phân công, không có tiết hoặc phân công tự phát sinh.
- Không trùng giáo viên hoặc lớp trong cùng thời điểm; các buổi/lớp học ghép được xử lý đúng theo dữ liệu nguồn.
- Phòng học, sức chứa và thiết bị khi đề bài có yêu cầu.
- Từng hard constraint theo mã; từng soft constraint và mức vi phạm còn lại.
- Các ràng buộc về chuỗi tiết, số buổi, thời gian nghỉ, lịch cố định, môn nặng/nhẹ và phân bố theo ngày khi có trong nguồn.

Kiểm tra ý nghĩa nguyên gốc, không chỉ lặp lại một phiên bản đã bị làm yếu. Ví dụ: `Không để toàn bộ tiết trong buổi đều là môn nặng` vẫn bị vi phạm ở buổi 3 tiết có 3 tiết nặng, dù thỏa `số tiết nặng <= 3`.

Nếu một hard constraint kiểm tra không đạt: coi lời giải hiện tại là không hợp lệ, tìm lỗi model/validator hoặc tiếp tục giải rồi kiểm tra lại. **Validator thất bại không chứng minh bài toán vô nghiệm.** Không giao lịch sai dưới nhãn hoàn tất.

## 7. Xuất và giao file

- Dùng `assets/mau-thoi-khoa-bieu.xlsx` khi cấu trúc mẫu phù hợp. Sao chép mẫu rồi ghi dữ liệu trực tiếp vào sheet `Thời khóa biểu`; giữ format và ô gộp hữu ích.
- Kiểm tra kích thước mẫu trước khi ghi. Khi cần mở rộng cho số lớp/khung giờ khác, mở rộng có kiểm soát, không bỏ lớp hoặc cắt lịch cho vừa mẫu.
- Ghi tên môn/giáo viên như văn bản, không để dữ liệu nguồn bắt đầu bằng dấu `=` trở thành công thức không mong muốn.
- Lưu/đóng file, mở lại workbook đầu ra và đối chiếu từng tiết với lời giải đã được kiểm tra. Kiểm tra số sheet, tên sheet, ô ngoài vùng in và nội dung thực tế; file tồn tại và không rỗng.
- Lưu báo cáo kiểm tra ngắn gọn cùng đầu ra: ràng buộc bắt buộc, kết quả, soft constraints còn chưa tối ưu và trạng thái solver thật.
- Trả link Markdown `file:///` với đường dẫn tuyệt đối, URI-encode ký tự đặc biệt. Nêu rõ lịch đã kiểm tra hợp lệ hay chỉ dữ liệu chuẩn hóa; không nói đã đạt tối ưu khi solver chỉ trả `FEASIBLE`.
