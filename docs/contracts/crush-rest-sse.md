# Hợp đồng REST + SSE với Crush

Chỉ `internal/crushapi` được nói giao thức này.

## Kênh truyền

| Nền tảng | Kênh | Tệp |
| --- | --- | --- |
| Windows | named pipe, dial bằng go-winio | `transport_windows.go` |
| linux, macOS | unix socket | `transport_unix.go` |

Không mở cổng TCP công khai. Đường dẫn socket hoặc tên pipe do
`internal/appconfig/paths.go` quyết định.

## Nguồn sự thật của payload

- `third_party/crush/internal/server` - route và proto.
- `third_party/crush/internal/swagger` - spec sinh từ server.

Sao chép hình dạng dữ liệu sang `internal/crushapi/contract.go`, không import code.

## Nhóm route cần cho MVP

| Nhóm | Dùng để |
| --- | --- |
| health, version | handshake và kiểm tra tương thích |
| workspace | attach project root |
| session | liệt kê, tạo, đọc lịch sử |
| message, prompt | gửi lượt mới, huỷ lượt |
| permission, question | trả lời yêu cầu của agent |
| history, diff | file thay đổi và nội dung diff |
| events (SSE) | stream token, tool, permission |

## Quy tắc

- Một stream SSE cho mỗi session đang hoạt động.
- Không polling thay cho sự kiện.
- Lỗi transport đi qua `internal/engine/health.go` để backoff và reconnect.
- Khi phiên bản giao thức lệch, báo UI qua `engine:status` thay vì crash.
