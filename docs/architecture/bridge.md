# Bridge: lớp host Go

Bridge là toàn bộ code Go của gotack, nơi duy nhất UI và engine gặp nhau.

## Tệp ở gốc repo (package main)

| Tệp | Vai trò |
| --- | --- |
| `main.go` | Điểm vào Wails, tuỳ chọn cửa sổ, embed `frontend/dist` |
| `app.go` | Struct `App` được bind, vòng đời, wiring service |
| `bind_bridge.go` | Probe và handshake, hiện có `BackendReady` |
| `bind_engine.go` | Vòng đời engine: status, start, stop, reconnect |
| `bind_workspace.go` | Chọn và mở workspace |
| `bind_session.go` | Session, lịch sử, gửi và huỷ prompt |
| `bind_permission.go` | Trả lời permission và question |
| `bind_changes.go` | Danh sách file thay đổi và diff |
| `bind_terminal.go` | Terminal tuỳ chọn, tạo khi cần |
| `events.go` | Nơi duy nhất phát sự kiện về UI |

## Vì sao method bind phải ở `package main`

Wails v2 sinh namespace JS theo tên package Go. UI gọi `window.go.main.App.*`
trong `frontend/src/platform/desktop.ts`, và nội dung `frontend/` là bất biến.
Đưa `App` vào `internal/app` sẽ đổi namespace thành `window.go.app.App` và làm UI
đứt kết nối. Vì vậy `bind_*.go` chỉ là vỏ rất mỏng, chuyển tiếp xuống `internal/`.

## Các package trong `internal/`

| Package | Vai trò |
| --- | --- |
| `appconfig` | Cấu hình người dùng, đường dẫn theo hệ điều hành |
| `logging` | Logger chung |
| `engine` | Tìm, khởi chạy, giám sát, dừng tiến trình Crush |
| `crushapi` | Nơi duy nhất nói giao thức REST + SSE của Crush |
| `workspace` | Gắn một project root vào engine |
| `session` | Điều phối hội thoại cho UI |
| `permission` | Chuyển tiếp yêu cầu duyệt và câu hỏi |
| `changes` | File thay đổi và diff |
| `terminal` | PTY tuỳ chọn |
| `uievents` | Tên sự kiện và forwarder SSE sang Wails |

## Ràng buộc quan trọng

Go không cho phép import package `internal` xuyên module. `third_party/crush` là
module riêng, nên gotack **không thể** import `third_party/crush/internal/client`.
Hợp đồng wire được khai báo lại trong `internal/crushapi/contract.go`, giao tiếp
thuần REST + SSE. Xem `../decisions/0003-vendor-crush-under-third-party.md`.

## Quy tắc cho mọi method bind

- Tham số và kết quả phải JSON hoá được, không truyền `context`, channel hay type của engine.
- Không chặn UI: việc dài chạy nền và báo kết quả bằng sự kiện.
- Mỗi API mới phải được ghi vào `../contracts/wails-bindings.md`.
