# Đóng gói và phát hành

## Build

```powershell
pnpm --filter gotack-frontend build
wails build
```

Kết quả nằm ở `build/bin/`, thư mục này không commit.

## Tài nguyên đóng gói

`build/` chứa tài nguyên theo nền tảng: icon, manifest, metadata. Wails CLI sinh
bản mặc định khi thiếu. Icon ứng dụng có thể lấy từ `frontend/public/tack.ico`.

## Engine đi kèm

gotack cần một binary `crush` khi chạy. Hai lựa chọn:

| Cách | Ưu | Nhược |
| --- | --- | --- |
| Đóng gói kèm binary | cài một lần là chạy | gói to hơn, phải theo nhịp upstream |
| Dùng `crush` có sẵn trên máy | gói nhẹ | phụ thuộc môi trường người dùng |

`internal/engine/locator.go` phải xử lý được cả hai trường hợp.

## CI dự kiến

- Kiểm tra: `go build ./...`, `go vet ./...`, build frontend.
- Phát hành: build theo từng nền tảng, đính kèm artifact theo tag.
- Repo chưa có workflow nào để tránh cấu hình sai; thêm khi bắt đầu phát hành thật.
