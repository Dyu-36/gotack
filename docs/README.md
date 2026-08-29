# Tài liệu gotack

Chỉ mục tài liệu. Mỗi thư mục một vai trò rõ ràng:

| Thư mục | Vai trò |
| --- | --- |
| `architecture/` | Cấu trúc hệ thống: tổng quan, bridge, engine, UI |
| `contracts/` | Hợp đồng giao tiếp: binding Wails và REST + SSE của Crush |
| `decisions/` | ADR: quyết định kiến trúc kèm lý do |
| `guides/` | Hướng dẫn phát triển và đóng gói phát hành |
| `patterns/` | Các quy ước kỹ thuật dùng lại trong repo |
| `plans/` | Kế hoạch triển khai (`active/` / `completed/`) |
| `product/` | Tài liệu đặc tả sản phẩm |
| `templates/` | Mẫu tài liệu: runbook, ADR, kế hoạch thực thi |
| `WORKFLOW.md` | Quy trình làm việc và ranh giới quyết định |
| `roadmap.md` | Phạm vi từng mốc và phần ngoài phạm vi |

## Kế hoạch hiện tại

- `plans/active/mvp.md` - checklist triển khai chi tiết cho MVP.
- `roadmap.md` - milestone/phạm vi cấp sản phẩm; không lặp lại task triển khai chi tiết.

## Đọc theo thứ tự

1. `architecture/overview.md` - ba lớp và cách chúng chạy cùng nhau.
2. `architecture/bridge.md` - lớp host Go, nơi mọi lệnh của UI đi qua.
3. `architecture/engine.md` - Crush lo việc gì, gotack không lo việc gì.
4. `architecture/ui.md` - quy ước bên trong `frontend/`.
5. `contracts/` - đọc trước khi thêm bất kỳ API mới nào.

## Quy tắc bất biến

- `frontend/` giữ nguyên tên: Wails v2 quy định thư mục frontend theo đường dẫn cố định.
- Method bind cho UI phải nằm trong `package main` ở gốc repo, namespace `window.go.main.App`.
- Không import `third_party/crush/internal/...`: Go chặn package internal giữa hai module.
- Mọi thay đổi API phải cập nhật `contracts/` trong cùng commit.
