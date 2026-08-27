# 0003 - Vendored Crush ở `third_party/` và tự viết client

## Bối cảnh

Crush được clone vào repo với `.git` và `go.mod` riêng, bị `.gitignore` bỏ qua.
Trước đây nó nằm ngay ở gốc repo, lẫn với code của gotack.

## Quyết định

1. Chuyển sang `third_party/crush/`, giữ nguyên tên và toàn bộ nội dung.
2. Không dùng thư mục `vendor/` ở gốc: Go sẽ bật vendor mode và làm build sai.
3. gotack tự viết client REST + SSE trong `internal/crushapi`.

## Lý do cho điểm 3

Go chặn import package `internal` xuyên module. `third_party/crush/internal/client`
thuộc module của Crush nên module gotack không thể import. Tài liệu cũ ghi ngược
lại và đã được sửa trong `docs/architecture/bridge.md`.

## Hệ quả

- Ranh giới giữa gotack và Crush là giao thức, không phải kiểu dữ liệu Go.
- Khi upstream đổi payload, chỉ `internal/crushapi/contract.go` phải đổi theo.
- Cần pin commit upstream trong `third_party/README.md`.
- Nếu sau này cần dùng chung code Go, phải xin upstream export ra package công khai.
