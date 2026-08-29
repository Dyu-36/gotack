# Gotack MVP Worklist

Checklist trung tâm cho các việc còn lại của MVP. Khi hoàn thành một mục, đổi `[ ]` thành `[x]` trong cùng commit/PR thực hiện công việc đó.

`docs/roadmap.md` chỉ giữ milestone/phạm vi sản phẩm; file này là checklist triển khai chi tiết.

## P0 - Xác minh và sửa correctness

- [ ] Chạy `go test ./...` trên checkout sạch và sửa tất cả lỗi.
- [ ] Chạy `go vet ./...` và sửa tất cả finding có ý nghĩa.
- [ ] Chạy `pnpm --dir frontend install --frozen-lockfile` và `pnpm --dir frontend check`.
- [ ] Chạy `pnpm --dir frontend build`.
- [ ] Chạy `wails build` trên Windows và xác nhận app khởi động được.
- [ ] Smoke test end-to-end: start/attach Crush -> chọn workspace -> list/create/switch session -> send prompt -> SSE stream -> cancel prompt.
- [ ] Smoke test permission/question flow với request thật từ Crush.
- [ ] Xác nhận UI restart có thể attach lại engine đang chạy và phục hồi workspace/session hợp lý.
- [ ] Sửa mọi contract/type/runtime issue phát hiện trong các bước trên trước khi coi MVP là green.

## P0 - Nối settings vào Crush thật

- [ ] Xác định contract chính xác của Crush cho provider/model/reasoning và credentials.
- [ ] Làm cho Provider/Model/Thinking trong Settings ảnh hưởng đến agent run thật, không chỉ lưu local config.
- [ ] Nối API key/custom endpoint vào cấu hình Crush theo cách an toàn; không log secret.
- [ ] Xác minh thay model/provider trên UI thực sự thay model/provider mà Crush sử dụng ở lượt tiếp theo.
- [ ] Xử lý restart/reconnect khi một thay đổi settings cần khởi động lại engine.

## P1 - Hoàn thiện coding workflow

- [ ] Thêm nút Stop/Cancel trong UI khi session đang streaming và nối vào `CancelPrompt`.
- [ ] Render `tool:activity` trong chat thay vì bỏ qua event.
- [ ] Làm permission UI thành component/modal rõ ràng thay vì flow tối thiểu.
- [ ] Làm question UI thành component/modal hỗ trợ choice, multi-select, text và yes/no đầy đủ.
- [ ] Thêm Changed Files panel dùng `ChangedFiles`.
- [ ] Tự refresh Changed Files khi nhận `changes:updated`.
- [ ] Thêm lightweight diff viewer dùng `FileDiff` với giới hạn kích thước/large-file UX.
- [ ] Thêm session rename persistence vào backend/Crush; không để rename chỉ tồn tại local UI.
- [ ] Thêm session delete persistence vào backend/Crush hoặc ẩn action nếu upstream không hỗ trợ.
- [ ] Quyết định và persist pin session nếu pin là tính năng sản phẩm; nếu không thì bỏ khỏi UI.
- [ ] Hiển thị lỗi engine/transport/session trong UI bằng toast/status thay vì silent fallback.
- [ ] Hoàn thiện reconnect/backoff UX khi SSE/engine mất kết nối.

## P1 - Terminal

- [ ] Lazy-load xterm chỉ khi người dùng mở terminal.
- [ ] Thêm terminal panel nối `OpenTerminal`, `WriteTerminal`, `ResizeTerminal`, `CloseTerminal`.
- [ ] Nối `terminal:data` và `terminal:exit` vào xterm lifecycle.
- [ ] Test resize, Unicode, Ctrl+C, shell exit và đóng workspace/app trên Windows.

## P1 - Crush distribution

- [ ] Chốt cách ship Crush: bundled binary mặc định hay dùng binary trên PATH với bundled fallback.
- [ ] Pin và ghi rõ upstream Crush commit/version trong `third_party/README.md`.
- [ ] Hoàn thiện script/update flow để refresh Crush và kiểm tra lại REST/SSE contract khi upstream thay đổi.
- [ ] Xác nhận locator tìm đúng bundled binary và external binary trên Windows.
- [ ] Xác nhận Gotack chỉ kill Crush process do chính Gotack khởi chạy.

## P1 - CI và Windows packaging

- [x] Thêm GitHub Actions cho Go test/vet và frontend check/build.
- [x] Thêm Windows build job cho Wails.
- [ ] Tạo artifact Windows có thể chạy trên máy sạch có WebView2/system requirements phù hợp.
- [ ] Hoàn thiện icon, version metadata và release naming.
- [ ] Chốt installer/portable strategy cho bản MVP.
- [x] Thêm release checklist và tag/version flow.

## P2 - Hiệu năng và độ bền

- [ ] Đo cold-start time và idle RAM trên máy 6 GB RAM.
- [ ] Đo RAM khi engine + một session + SSE đang chạy; đảm bảo UI không duplicate state lớn từ Crush.
- [ ] Kiểm tra memory/resource leak sau nhiều lần switch workspace/session và reconnect.
- [ ] Thêm log rotation và UI/log viewer tối thiểu.
- [ ] Test workspace/path Unicode và đường dẫn dài trên Windows.
- [ ] Test engine crash, app crash, SSE disconnect và recovery.
- [ ] Test large history/large diff để đảm bảo UI vẫn responsive.

## P2 - Dọn dẹp trước release

- [ ] Cập nhật `docs/roadmap.md` theo trạng thái thực tế và tick các mục đã hoàn thành.
- [ ] Đồng bộ README stack/status với dependency và feature thực tế.
- [ ] Xóa preview/mock code không còn được dùng.
- [ ] Kiểm tra mọi file/abstraction lớn để tránh spaghetti và file vượt ngưỡng 1000 dòng.
- [ ] Chạy một vòng manual regression trên Windows trước tag MVP đầu tiên.
