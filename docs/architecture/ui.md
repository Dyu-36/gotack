# frontend/ — Tầng UI

## Công nghệ

- Svelte 5 (runes: `$state`, `$effect`, `.svelte.ts` modules) + TypeScript, build bằng Vite.
- Được nhúng vào binary Go qua `//go:embed all:frontend/dist` (xem `main.go` ở root) — Wails AssetServer serve.
- Terminal (`@term/xterm`) và editor (CodeMirror 6) lazy-load khi cần — không init lúc startup.

## Cấu trúc

```text
frontend/src/
├── main.ts                    # entry, mount App
├── App.svelte                 # layout shell, wire các state + component
├── app/
│   └── theme.svelte.ts        # theme state (runes)
├── platform/
│   └── desktop.ts             # ⚠ biên duy nhất chạm window.go (Wails IPC)
├── features/
│   └── conversations/         # state sessions/chat
└── components/
    ├── Sidebar.svelte         # danh sách session, workspace picker
    ├── ChatArea.svelte        # luồng tin nhắn + streaming
    ├── Composer.svelte        # ô nhập prompt
    └── SettingsModal.svelte   # cài đặt theme
```

## Quy tắc giao tiếp ra ngoài

1. **Chỉ `platform/desktop.ts` được phép gọi `window.go.*`.** Component/state gọi qua hàm export từ module này. Lý do: UI phải giữ khả năng chạy standalone trong browser (Wails API vắng mặt → fallback graceful, ví dụ `backendReady()` trả `false` khi `window.go` undefined).
2. **State nằm trong Svelte stores / runes**, không mirror state dài hạn của engine. Session history, permission, tool state là của Crush; UI chỉ cache để render.
3. **Streaming events** (khi implement) đăng ký qua Wails runtime event (`EventsOn`) — cũng bọc trong `platform/`, expose dạng callback/store cho UI.

## Hiện trạng

- `desktop.backendReady()` → probe `App.BackendReady()` trên host; `App.svelte` dùng nó để bật/tắt gửi tin nhắn.
- Chat đang chạy ở chế độ preview local (`sendPreviewMessage`) — chưa nối vào engine thật.

## Khi nối engine thật

Thêm method vào `DesktopApp` type trong `platform/desktop.ts` theo đúng contract mà host bind (xem `bridge.md`), rồi expose qua store trong `features/`. Không đổi cấu trúc component khi thay backend — đây là mục tiêu thiết kế của biên này.
