# Tổng quan kiến trúc

gotack là lớp desktop mỏng bọc quanh Crush. Ba lớp, ba trách nhiệm tách biệt.

| Lớp | Vị trí | Trách nhiệm |
| --- | --- | --- |
| UI | `frontend/` | Svelte 5 + Vite, chỉ render và gửi lệnh |
| Host | `main.go`, `app.go`, `bind_*.go`, `events.go`, `internal/` | Vòng đời cửa sổ, vòng đời engine, cầu nối UI - engine |
| Engine | `third_party/crush/` | Agent, session, permission, LSP, MCP, lưu trữ |

## Mô hình tiến trình

```text
gotack.exe (WebView + host Go)
   |  window.go.main.App.*   UI gọi hàm host (Wails binding)
   |  EventsEmit             host đẩy sự kiện về UI
   v
crush server (tiến trình riêng)
   REST + SSE qua unix socket hoặc Windows named pipe
```

## Luồng một lượt hội thoại

1. UI gọi `SendPrompt` qua `frontend/src/platform/desktop.ts`.
2. `bind_session.go` chuyển tiếp xuống `internal/session`.
3. `internal/crushapi` gửi request tới Crush và mở stream SSE.
4. `internal/uievents` đẩy token, tool call, yêu cầu permission về UI qua `events.go`.
5. UI vẽ lại theo sự kiện, không polling.

## Nguyên tắc

- Host không chứa logic agent. Mọi quyết định thuộc về Crush.
- Engine sống độc lập: khởi động lại UI không được giết agent đang chạy.
- Ngân sách bộ nhớ cho máy 6 GB: terminal và editor lazy load.
- Một vai trò một package trong `internal/`, không có package tiện ích chung.
