# gotack Worklists

Checklist trung tam cho cac viec con lai. Khi hoan thanh mot muc, doi `[ ]` thanh `[x]` trong cung commit/PR thuc hien cong viec do.

## P0 - Xac minh va sua correctness

- [ ] Chay `go test ./...` tren checkout sach va sua tat ca loi.
- [ ] Chay `go vet ./...` va sua tat ca finding co y nghia.
- [ ] Chay `pnpm install --frozen-lockfile` va `pnpm --filter gotack-frontend check`.
- [ ] Chay `pnpm --filter gotack-frontend build`.
- [ ] Chay `wails build` tren Windows va xac nhan app khoi dong duoc.
- [ ] Smoke test end-to-end: start/attach Crush -> chon workspace -> list/create/switch session -> send prompt -> SSE stream -> cancel prompt.
- [ ] Smoke test permission/question flow voi request that tu Crush.
- [ ] Xac nhan UI restart co the attach lai engine dang chay va phuc hoi workspace/session hop ly.
- [ ] Sua moi contract/type/runtime issue phat hien trong cac buoc tren truoc khi coi MVP la green.

## P0 - Noi settings vao Crush that

- [ ] Xac dinh contract chinh xac cua Crush cho provider/model/reasoning va credentials.
- [ ] Lam cho Provider/Model/Thinking trong Settings anh huong den agent run that, khong chi luu local config.
- [ ] Noi API key/custom endpoint vao cau hinh Crush theo cach an toan; khong log secret.
- [ ] Xac minh thay model/provider tren UI thuc su thay model/provider ma Crush su dung o luot tiep theo.
- [ ] Xu ly restart/reconnect khi mot thay doi settings can khoi dong lai engine.

## P1 - Hoan thien coding workflow

- [ ] Them nut Stop/Cancel trong UI khi session dang streaming va noi vao `CancelPrompt`.
- [ ] Render `tool:activity` trong chat thay vi bo qua event.
- [ ] Lam permission UI thanh component/modal ro rang thay vi flow toi thieu.
- [ ] Lam question UI thanh component/modal ho tro choice, multi-select, text va yes/no day du.
- [ ] Them Changed Files panel dung `ChangedFiles`.
- [ ] Tu refresh Changed Files khi nhan `changes:updated`.
- [ ] Them lightweight diff viewer dung `FileDiff` voi gioi han kich thuoc/large-file UX.
- [ ] Them session rename persistence vao backend/Crush; khong de rename chi ton tai local UI.
- [ ] Them session delete persistence vao backend/Crush hoac an action neu upstream khong ho tro.
- [ ] Quyet dinh va persist pin session neu pin la tinh nang san pham; neu khong thi bo khoi UI.
- [ ] Hien thi loi engine/transport/session trong UI bang toast/status thay vi silent fallback.
- [ ] Hoan thien reconnect/backoff UX khi SSE/engine mat ket noi.

## P1 - Terminal

- [ ] Cai va pin exact `@xterm/xterm` version.
- [ ] Lazy-load xterm chi khi nguoi dung mo terminal.
- [ ] Them terminal panel noi `OpenTerminal`, `WriteTerminal`, `ResizeTerminal`, `CloseTerminal`.
- [ ] Noi `terminal:data` va `terminal:exit` vao xterm lifecycle.
- [ ] Test resize, Unicode, Ctrl+C, shell exit va dong workspace/app tren Windows.

## P1 - Crush distribution

- [ ] Chot cach ship Crush: bundled binary mac dinh hay dung binary tren PATH voi bundled fallback.
- [ ] Pin va ghi ro upstream Crush commit/version trong `third_party/README.md`.
- [ ] Hoan thien script/update flow de refresh Crush va kiem tra lai REST/SSE contract khi upstream thay doi.
- [ ] Xac nhan locator tim dung bundled binary va external binary tren Windows.
- [ ] Xac nhan gotack chi kill Crush process do chinh gotack khoi chay.

## P1 - CI va Windows packaging

- [ ] Them GitHub Actions cho Go test/vet va frontend check/build.
- [ ] Them Windows build job cho Wails.
- [ ] Tao artifact Windows co the chay tren may sach co WebView2/system requirements phu hop.
- [ ] Hoan thien icon, version metadata va release naming.
- [ ] Chot installer/portable strategy cho ban MVP.
- [ ] Them release checklist va tag/version flow.

## P2 - Hieu nang va do ben

- [ ] Do cold-start time va idle RAM tren may 6 GB RAM.
- [ ] Do RAM khi engine + mot session + SSE dang chay; dam bao UI khong duplicate state lon tu Crush.
- [ ] Kiem tra memory/resource leak sau nhieu lan switch workspace/session va reconnect.
- [ ] Them log rotation va UI/log viewer toi thieu.
- [ ] Test workspace/path Unicode va duong dan dai tren Windows.
- [ ] Test engine crash, app crash, SSE disconnect va recovery.
- [ ] Test large history/large diff de dam bao UI van responsive.

## P2 - Don dep truoc release

- [ ] Cap nhat `docs/roadmap.md` theo trang thai thuc te va tick cac muc da hoan thanh.
- [ ] Dong bo README stack/status voi dependency va feature thuc te.
- [ ] Xoa preview/mock code khong con duoc dung.
- [ ] Kiem tra moi file/abstraction lon de tranh spaghetti va file vuot nguong 1000 dong.
- [ ] Chay mot vong manual regression tren Windows truoc tag MVP dau tien.
