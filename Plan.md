# Kế hoạch hợp nhất Gotack

## Mục tiêu

- Hợp nhất `gotack` và `tack-engine` thành một repo duy nhất: `gotack`.
- Có một Go module chính: `github.com/Dyu-36/gotack`.
- Sử dụng Gotack qua cả terminal tương tác và desktop app Wails/Svelte.
- Hai giao diện dùng chung lõi agent, tools, providers, permissions và persistence.
- Repo `gotack` tự build, kiểm thử và phát hành toàn bộ sản phẩm, không phụ thuộc repo `tack-engine`.
- Sau khi hoàn tất và kiểm chứng việc chuyển đổi, có thể xóa repo `tack-engine`.

## Kiến trúc đích

- Desktop giữ entrypoint ở root cùng `frontend/` để giảm thay đổi cấu trúc Wails.
- Terminal đặt tại `cmd/gotack/`, với các lệnh dự kiến:
  - `gotack chat`: chat tương tác trong terminal.
  - `gotack run`: thực thi prompt từ dòng lệnh.
  - `gotack serve`: chạy engine phục vụ các giao diện.
- Source engine chuyển vào `internal/agentcore/`; mã giao diện terminal nằm trong `internal/terminal/`.
- Giữ ranh giới API/IPC giữa giao diện và engine. Hai giao diện dùng chung API client và cơ chế quản lý tiến trình, tránh tạo hai luồng thực thi agent riêng.
- Sản phẩm có executable terminal `gotack` và desktop `gotack-desktop`; hợp nhất repo không yêu cầu gom thành một executable.

## Hướng thực hiện

### 1. Nhập source và lịch sử

Nhập engine vào `gotack`, giữ lịch sử Git và license/notices của từng phần. Có thể tạm giữ hai module trong bước chuyển tiếp để kiểm chứng hành vi hiện tại; đích cuối cùng vẫn là một module chính.

### 2. Hợp nhất lõi và bổ sung terminal

Chuyển import engine sang namespace `gotack`, xử lý package trùng tên và tổ chức lại các entrypoint. Giữ hành vi desktop, cấu hình và dữ liệu người dùng hiện có. Xây dựng giao diện terminal với nhập prompt, streaming, duyệt quyền tool và hủy tác vụ.

Engine hiện chỉ có lệnh `server`; giao diện chat terminal là tính năng cần triển khai thêm.

### 3. Hợp nhất build và release

Gom CI, scripts và release vào `gotack`. Bỏ `.tack-pin` và các bước lấy source từ repo engine riêng sau khi có cơ chế nhận diện bản build và kiểm tra tương thích thay thế. Build terminal, desktop và engine từ cùng commit, giữ kiểm thử IPC thực tế.

### 4. Kiểm chứng và kết thúc chuyển đổi

Kiểm thử cả hai giao diện, cập nhật tài liệu sử dụng và xác nhận một checkout mới của `gotack` có thể build toàn bộ sản phẩm. Sau khi đạt các tiêu chí dưới đây, chuyển tài liệu cần thiết và kết thúc sử dụng repo `tack-engine` trước khi xóa.

## Tiêu chí hoàn thành

- Chỉ cần clone `gotack` để build và chạy cả terminal lẫn desktop.
- Có một Go module chính và một pipeline phát hành toàn sản phẩm.
- Cả hai giao diện chạy được agent, streaming, tools, quyền thực thi và hủy tác vụ.
- Desktop vẫn mở được session, cấu hình và dữ liệu trước khi hợp nhất.
- Không còn yêu cầu checkout, tải source hoặc build từ repo `tack-engine`.
- License/notices được giữ đầy đủ; kiểm thử và product build thành công.
- Việc xóa repo `tack-engine` không ảnh hưởng đến build hoặc sử dụng Gotack.
