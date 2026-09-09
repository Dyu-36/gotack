Các thay đổi này giữ nguyên skill `timetable`. Chúng sửa cách Gotack cấu hình, điều khiển và quan sát engine. Chưa có phép chạy đối chứng để khẳng định mức giảm thời gian.

| Thay đổi | Lý do và giới hạn |
| --- | --- |
| Hợp nhất `extra_body` theo từng lớp; cấu hình rõ ràng thắng giá trị mặc định | Tránh engine ghi đè cấu hình provider. Giữ cả `false` và `null`. |
| Phân biệt effort được yêu cầu, effort engine chọn và các trường reasoning trong provider options cuối | Nhìn thấy fallback thay vì suy từ lựa chọn trên giao diện. Đây là options trước khi adapter gửi request; không chứng minh provider thực thi đúng effort hoặc cho phép tắt thinking. |
| Shell trả `is_error`, `status`, `exit_code`; job nền cũng giữ trạng thái lỗi | Không còn biểu diễn tiến trình thất bại như một kết quả văn bản thành công. Lỗi không có exit code xác định để trống trường đó. |
| Python mặc định UTF-8 trên Windows khi chưa có cấu hình môi trường tương ứng | Giảm lỗi in tiếng Việt; không ghi đè lựa chọn encoding rõ ràng của người dùng. |
| Prompt nền dành cho trợ lý cá nhân; quy trình repository chỉ áp dụng cho công việc mã nguồn | Giảm chỉ dẫn coding chồng lên các tác vụ khác. Giữ nguyên phần bắt buộc đọc và thực hiện skill, cùng yêu cầu kiểm tra đầu ra. |
| Chuẩn hóa JSON input và bỏ `description` của bash khi tính chữ ký lặp | Nhận diện lệnh lặp dù đổi mô tả hoặc thứ tự trường JSON. Không phân tích ngữ nghĩa script và không tự cắt bước kiểm định. |
| Engine dùng thư mục config/data/cache riêng của Gotack | Không kế thừa cấu hình Crush toàn cục. Cấu hình tại workspace được chọn vẫn có hiệu lực. |

Log chẩn đoán nằm tại `%LOCALAPPDATA%\gotack\input-pipeline.jsonl`. Mỗi dòng có `run_id`, `session_id`, `started_at`, `app_build` và `engine_build` để nối đúng phiên với đúng binary. `app_build.id` là SHA-256 của chương trình Gotack đang chạy; `engine_build.source_digest` nhận diện tập tin nguồn được build bởi script bên dưới.

`model_calls` ghi riêng các lần stream cho vòng tool, tạo tiêu đề và tóm tắt; gồm thời điểm bắt đầu/kết thúc, reasoning, tool input, text, lỗi và usage được provider báo. Không dùng bộ đếm token của session làm tổng toàn phiên. `provider_attempts` nối thời điểm ghi request HTTP và nhận byte phản hồi đầu tiên với từng model call.

`tool_calls` ghi thời gian thực hiện tool, phần chạy hook, phần đi qua cơ chế cấp quyền và trạng thái kết quả. Thời gian hook/quyền nằm trong tổng thời gian tool, không cộng lần nữa. Tool chạy nền chỉ đo đến lúc trả kết quả của lần gọi đó; cần đối chiếu các lần lấy kết quả job sau đó để hiểu toàn bộ tiến trình nền.

Các mốc `_us` là microsecond tính từ `started_at`; `delivery_us`, `hook_us`, `permission_us` là khoảng thời gian. `delivery_us` đo thời gian chuyển từng stream part cho bộ xử lý engine. Thời gian model stream bao gồm provider, mạng, xử lý stream và phần giao kết quả này; không phải phép đo thời gian GPU. Các khoảng có thể chồng nhau, nhất là tạo tiêu đề và tool chạy đồng thời. Trường không quan sát được để trống; stream tạo tiêu đề còn chạy khi phiên kết thúc có thể chưa có `ended_us` trong snapshot.

Mỗi nhóm bản ghi được giới hạn 2.048 mục; `execution_records_dropped` báo số bản ghi bị bỏ khi chạm giới hạn. Log không thêm nội dung prompt, câu lệnh shell, đầu ra tool hay API key. Các options reasoning được chọn qua danh sách trường cho phép, không ghi toàn bộ request.

Engine có repo riêng tại `third_party/engine-source`, bị Git của Gotack bỏ qua. Bản vá được lưu tại `patches/engine-execution.patch`; pipeline phát hành áp dụng lên commit trong `.tack-pin`. Build tại máy bằng PowerShell 7:

```powershell
./scripts/build-engine.ps1
wails build -platform windows/amd64 -webview2 download -m
```

Script từ chối checkout khác pin hoặc bản vá không áp dụng được; không tự reset mã nguồn. Manifest nằm cạnh engine tại `build/bin/resources/tack-engine.exe.build.json`, gồm commit nền, hash nguồn, hash bản vá và SHA-256 binary. Khi chuyển bản vá thành commit trong repo engine, cần cập nhật pin và cách áp dụng bản vá cùng nhau.

Trong đợt sửa này chỉ biên dịch và đối chiếu tính toàn vẹn tập tin; không chạy unit test, integration test, benchmark hay yêu cầu model. Các nhóm cần người kiểm thử đối chiếu gồm provider options/fallback, shell thất bại và job nền, Unicode, kích hoạt skill, nhận diện lặp, log theo session và độ đúng của đầu ra timetable. Không mặc định tốc độ cải thiện chỉ vì build thành công.

Môi trường máy thử được sao lưu rồi bỏ khỏi vị trí hoạt động: dữ liệu Gotack trong Roaming/Local AppData, WebView của `tack.exe` và `Downloads\.tack`. Giữ lại thư mục skills đã cài; không khôi phục config, thông tin provider, lịch sử hay memory cũ. Manifest sao lưu cụ thể được ghi tại `artifacts/timetable-audit/machine-reset.json`. Sau khi mở app, nhập lại provider/API key và tạo phiên mới. Dữ liệu đầu vào trong Downloads được giữ nguyên.
