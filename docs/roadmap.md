# Roadmap

## Mốc 1 - MVP dùng được hằng ngày

- [ ] Vòng đời engine: tìm, khởi chạy, giám sát, dừng đúng chủ sở hữu
- [ ] Client REST + SSE trong `internal/crushapi`
- [ ] Chọn và mở workspace
- [ ] Danh sách session, tạo và chuyển session
- [ ] Gửi prompt, stream token, huỷ lượt
- [ ] Duyệt permission và trả lời question
- [ ] Danh sách file thay đổi và diff gọn
- [ ] Cài đặt: theme, engine, đường dẫn
- [ ] Đóng gói bản Windows đầu tiên

## Mốc 2 - Chất lượng

- [ ] Gắn lại engine đang chạy sau khi UI khởi động lại
- [ ] Backoff và thông báo rõ khi mất kết nối engine
- [ ] Terminal lazy load
- [ ] Log xoay vòng và cửa sổ xem log

## Ngoài phạm vi hiện tại

- Editor đầy đủ kèm syntax service
- Viết lại tính năng agent trong host
- Đồng bộ nhiều máy
