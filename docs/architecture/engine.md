# Engine: Crush

Crush là bộ não. gotack không thay thế bất kỳ phần nào của nó.

Nguồn: https://github.com/charmbracelet/crush - vendored tại `third_party/crush/`
với `.git` và `go.mod` riêng, được `.gitignore` của repo này bỏ qua.

## Crush sở hữu

| Vùng | Đường dẫn tham chiếu |
| --- | --- |
| Vòng lặp agent, tool call | `third_party/crush/internal/agent` |
| Session và message | `third_party/crush/internal/session`, `internal/message` |
| Permission và question | `third_party/crush/internal/permission`, `internal/question` |
| Lịch sử file và snapshot | `third_party/crush/internal/history` |
| LSP, MCP, skills, hooks | `third_party/crush/internal/lsp`, `internal/skills`, `internal/hooks` |
| Lưu trữ SQLite | `third_party/crush/internal/db` |
| HTTP server, proto, SSE | `third_party/crush/internal/server` |

## gotack sở hữu

- Cửa sổ, WebView, theme, phím tắt.
- Vòng đời tiến trình engine và khả năng gắn lại sau khi UI khởi động lại.
- Chuyển sự kiện engine thành sự kiện UI.

## Không làm

- Không fork logic agent sang host Go.
- Không sửa `third_party/crush` để phục vụ nhu cầu riêng của desktop.
- Không đọc trực tiếp SQLite của Crush: mọi truy cập đi qua REST.
- Không import `third_party/crush/internal/...`, Go chặn internal xuyên module.

## Khi cập nhật upstream

1. Ghi commit được pin vào `third_party/README.md`.
2. So `internal/server/proto.go` của Crush với `internal/crushapi/contract.go`.
3. Cập nhật `../contracts/crush-rest-sse.md` nếu route hoặc payload đổi.
