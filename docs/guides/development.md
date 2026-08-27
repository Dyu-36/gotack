# Hướng dẫn phát triển

## Yêu cầu

- Go theo `go.mod`
- Node và pnpm theo `packageManager` trong `package.json`
- Wails CLI v2
- WebView2 trên Windows

## Chạy

```powershell
pnpm install
wails dev
```

`wails dev` dùng `frontend:install` và `frontend:dev` trong `wails.json`, chạy Vite
watcher rồi mở cửa sổ với binding Go được sinh lại.

## Vòng lặp khi thêm một API

1. Thêm method vào `bind_*.go` đúng vai trò, giữ vỏ mỏng.
2. Cài phần việc thật vào package `internal/` tương ứng.
3. Nếu có sự kiện mới, khai báo tên trong `internal/uievents/names.go`.
4. Cập nhật `docs/contracts/wails-bindings.md`.
5. `wails dev` sinh lại `frontend/wailsjs/`, thư mục này không commit.

## Kiểm tra trước khi commit

```powershell
go build ./...
go vet ./...
pnpm --filter gotack-frontend build
```

## Ranh giới không được vượt

- Không thêm code Go vào `frontend/`, không thêm logic UI vào host.
- Không sửa `third_party/crush` cho nhu cầu riêng của desktop.
- Không tạo package tiện ích chung kiểu `utils`, đặt tên theo vai trò.
- Không polling khi đã có sự kiện SSE.
