---
name: timetable
description: Quy trình tạo thời khóa biểu trường học: thu thập thông tin, chuẩn hóa phân công, tự động sắp xếp lịch tối ưu và xuất ra tệp Excel chuẩn.
---

# Quy Trình Tạo Thời Khóa Biểu

## ⚠️ Quy tắc giao tiếp với người dùng (BẮT BUỘC)
- Toàn bộ câu trả lời, giải thích và thông báo phải **thuần tiếng Việt, gần gũi, xưng hô thầy/cô (hoặc bạn)**.
- **TUYỆT ĐỐI KHÔNG dùng từ ngữ kỹ thuật, tên thuật toán hay tiếng Anh:**
  - ❌ Tránh: *"bộ giải ortools"*, *"chạy solver"*, *"validate dữ liệu"*, *"ràng buộc cứng/mềm"*, *"penalty"*, *"file JSON"*, *"infeasible"*...
  -  Thay bằng: *"hệ thống đang tiến hành xếp lịch"*, *"rà soát lại phân công"*, *"yêu cầu bắt buộc / yêu cầu ưu tiên"*, *"chưa thể xếp được lịch do bị trùng/vướng lịch..."*.

## ⚠️ Quy tắc kỷ luật (BẮT BUỘC — tránh lãng phí thời gian của thầy/cô)
- **KHÔNG in lại bảng dài** (quá 30 dòng) vào khung chat. Chỉ tóm tắt: số giáo viên, số lớp, tổng số tiết.
- **KHÔNG chạy lệnh Python khám phá** (`python -c` để dò dữ liệu). Đọc trực tiếp file input là đủ.
- **KHÔNG viết script kiểm tra riêng** (check_*.py, validate.py...). Việc kiểm tra nằm **bên trong** công cụ xếp lịch có sẵn.
- **KHÔNG nhẩm tính tay.** Mọi phép tính (tổng tiết, kiểm tra trùng...) đều bằng công cụ.
- **KHÔNG tự viết chương trình xếp lịch.** Bộ xếp lịch đã có sẵn trong thư mục skill (`runtime/solver.py`), đã được kiểm định. Việc của bạn chỉ là **mô tả bài toán** cho nó (Bước 2).
- **KHÔNG tự tạo file Excel.** File TKB chỉ được sinh ra bằng `runtime/exporter.py` (Bước 3). Cấm `officecli`, cấm `openpyxl`/script tự viết, cấm dụng từng ô bằng tay — kể cả khi skill Office khác được kích hoạt kèm.
- **KHÔNG lặp lại một thao tác đã thất bại** mà chưa đổi gì. Nếu một bước lỗi 2 lần liền, dừng lại, đọc kỹ thông báo lỗi, đổi cách làm.
- **KỶ LUẬT NGÂN SÁCH THỜI GIAN & THỬ NGHIỆM**:
  - **Ngân sách thời gian bằng số**: Lần 1 chạy nhanh (Phase A 20s / Phase B 10–15s). Tối đa 2 lần thử, tổng thời gian toàn bộ tác vụ <= 5 phút.
  - **CẤM tăng `phase_a_seconds` lên hàng trăm giây** (solver có trần cứng 60s). Nếu vô nghiệm/timeout, vấn đề là do xung đột ràng buộc hoặc phân công quá tải, hãy nới ràng buộc thay vì tăng thời gian.
  - **TUYỆT ĐỐI KHÔNG copy `solver.py` sang `work/solver.py` để patch.** Mọi yêu cầu đặc thù chỉ viết qua plugin `work/constraints_extra.py`.
  - **KHÔNG tự ý bật `"compact": true`** trừ khi đề bài yêu cầu rõ ràng "học liền mạch không trống giữa buổi". Bật oan sẽ làm bài toán chật hoặc vô nghiệm.

---

### 📌 BƯỚC 1: Tiếp nhận & đọc dữ liệu
- Tiếp nhận khung thời gian (ngày, buổi, tiết) và các mong muốn từ thầy/cô.
- Đọc file input đính kèm (đã được chuyển sẵn thành văn bản có bảng): bảng phân công cần có thông tin: Giáo viên | Môn học | Lớp | Số tiết.
- Sau khi đọc, chỉ báo cáo ngắn gọn: số giáo viên, số lớp, tổng số tiết/tuần, các yêu cầu đặc biệt kèm theo (nếu có). KHÔNG in lại toàn bộ bảng.

---

### 📌 BƯỚC 2: Xếp lịch tự động

**Ưu tiên giao cho agent chuyên trách nếu có tool `agent`:**
- Nếu tool `agent` dùng được, gọi đúng **một lần** với một đối số `prompt`:
  - `{"prompt":"Xếp thời khóa biểu từ file <đường dẫn file input>, khung thời gian <mô tả>, các yêu cầu <danh sách>. Kết quả ghi vào output/schedule.json."}`
  - KHÔNG tách đọc dữ liệu, tạo bài toán, chạy bộ xếp lịch hay kiểm tra kết quả thành nhiều lời gọi: các bước này dùng chung file và phải chạy tuần tự trong một agent.

**Quy trình tự làm (hoặc nội dung agent con thực hiện) — MÔ TẢ BÀI TOÁN, KHÔNG VIẾT THUẬT TOÁN:**

1. **Điền file `work/problem.json`** theo schema trong `<đường dẫn skill>/reference/problem-schema.md`:
   - `frame`: khung thời gian (ngày, buổi, số tiết mỗi buổi).
   - `assignments`: bảng phân công 4 cột đã đọc ở Bước 1 (cộng các tiết suy ra như SHL theo chủ nhiệm nếu dữ liệu yêu cầu).
   - `requirements`: **từng** yêu cầu bắt buộc của thầy/cô dịch sang đúng một loại trong schema (ghim, cấm/không được, giới hạn/ngày, trải ngày, tiết đôi liền kề, phòng học hạn chế...). Đối chiếu bảng ví dụ trong tài liệu schema — gần như mọi yêu cầu thực tế đều có mẫu tương ứng.
   - `preferences`: các mong muốn "đẹp" (ưu tiên buổi, tránh giờ xấu...) với trọng số.
   - Nếu thầy/cô yêu cầu **lớp học liền mạch từ tiết đầu, không trống giữa buổi** → thêm `"compact": true`; nếu không nói gì thì để mặc định (`false` — không bật).
2. **Yêu cầu nào không diễn đạt được bằng schema** → viết hàm ngắn trong `work/constraints_extra.py` đúng mẫu "API mở rộng" cuối tài liệu schema (chỉ viết đúng ràng buộc đó, 10–30 dòng). Tuyệt đối KHÔNG bỏ silently yêu cầu nào.
3. **Chạy bộ xếp lịch có sẵn** (lưu ý cờ UTF-8 bắt buộc trên Windows):
   ```bash
   python -X utf8 "<đường dẫn skill>/runtime/solver.py" work/problem.json output/schedule.json
   ```
   *(Có thể dùng thêm cờ `--time-budget 30` hoặc `--phase-a-only` khi cần kiểm tra nhanh)*.
4. **Đọc tổng kê trên màn hình:**
   - **Thành công** (bảng PASS hết): sang Bước 3.
   - **"CHƯA XẾP ĐƯỢC — CÁC YÊU CẦU MÂU THUẪN"**: solver chỉ rõ nhóm yêu cầu conflict (hoặc chạy thêm cờ `--diagnose`). Giải thích cho thầy/cô bằng ngôn ngữ đời thường kèm số liệu (ví dụ: *"Thầy Hưng chỉ còn 6 tiết trống trong tuần nhưng được phân 8 tiết..."*) và **đề xuất nới yêu cầu nào**. KHÔNG thử lại mù quáng quá 2 lần.
   - Mã lỗi 3 (problem.json lỗi): sửa đúng theo thông báo (key lạ/thiếu trường) rồi chạy lại.

---

### 📌 BƯỚC 3: Xuất Excel & Bàn giao

> ⛔ **CHỈ CÓ MỘT ĐƯỜNG DUY NHẤT ĐỂ TẠO FILE TKB: `runtime/exporter.py`.**
> - **CẤM tạo file Excel bằng bất kỳ cách khác**: không `officecli`, không `openpyxl`/`pandas` tự viết, không script tạo .xlsx riêng, không tạo bằng tay từng ô. File TKB có định dạng chuẩn trường học (A4 ngang, gộp ô, khung tiết/buổi) đã được dụng sẵn trong `exporter.py` — tự dung file khác luôn ra sai định dạng và bị coi là thất bại.
> - **Nếu skill `officecli-xlsx` (hoặc bất kỳ skill Office nào) được kích hoạt kèm theo — Bỏ QUA nó cho tác vụ thời khóa biểu.** Các skill đó trigger vì thấy đuôi `.xlsx`, nhưng ở đây chúng **không áp dụng**. Skill `timetable` có quyền ưu tiên cao nhất.
> - Chỉ dùng `officecli` khi thầy/cô yêu cầu rõ một việc **ngoài** TKB (ví dụ: gộp thêm một sheet báo cáo vào file đã xuất), và chỉ sau khi `exporter.py` đã tạo xong file gốc.
> - Nếu `exporter.py` lỗi: đọc thông báo lỗi và sửa `output/schedule.json`, **không được vòng sang cách khác để “có file cho xong”**. Báo lại cho thầy/cô nếu không xử lý được.

1. Chạy công cụ tạo file Excel:
   ```bash
   python -X utf8 "<đường dẫn skill>/runtime/exporter.py" output/schedule.json output/thoi-khoa-bieu.xlsx
   ```
2. Hiển thị bảng thời khóa biểu dạng Markdown trong khung chat (nếu nhiều lớp, mỗi lớp một bảng).
3. Đính kèm liên kết mở file Excel: `[Mở thời khóa biểu](file:///<path>/thoi-khoa-bieu.xlsx)`.
