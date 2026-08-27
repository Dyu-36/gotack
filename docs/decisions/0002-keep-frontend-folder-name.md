# 0002 - Giữ nguyên tên thư mục `frontend/`

## Bối cảnh

Khi tổ chức lại repo, tên `frontend/` được xem xét đổi thành `ui/` hoặc `app/`
cho khớp quy ước đặt tên theo vai trò.

## Quyết định

Giữ nguyên `frontend/`.

## Lý do

- Wails v2 xác định thư mục frontend theo đường dẫn cố định `<project>/frontend`,
  dùng cho cả `wails dev` và `wails build`. Đổi tên làm hai lệnh này đứt.
- `main.go` embed `all:frontend/dist`.
- `wails.json` và `pnpm-workspace.yaml` trỏ tới cùng thư mục đó.
- Nội dung `frontend/` là bất biến trong đợt tổ chức lại này.

## Hệ quả

Tên thư mục UI không phải chỗ thể hiện quy ước đặt tên. Vai trò của nó được ghi
trong `AGENTS.md` và `docs/architecture/ui.md`.
