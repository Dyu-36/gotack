# 0001 - Lớp desktop mỏng

## Bối cảnh

Crush đã có agent, session, permission, LSP, MCP và lưu trữ. Viết lại các phần
này trong host Go sẽ tạo ra hai nguồn sự thật cho cùng một hành vi.

## Quyết định

Host Go chỉ làm ba việc: quản lý cửa sổ, quản lý vòng đời engine, chuyển tiếp
lệnh và sự kiện. Mọi logic agent nằm ở Crush.

## Hệ quả

- `internal/` chia theo vai trò chuyển tiếp, không có tầng domain riêng.
- Nâng cấp Crush là cập nhật `third_party/crush` cộng sửa contract nếu cần.
- Tính năng agent mới thường chỉ cần sửa UI và contract, không sửa host.
- Nếu một tính năng đòi logic agent trong host, đó là dấu hiệu sai lớp.
