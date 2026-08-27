# Hợp đồng binding Wails

Mọi lệnh UI gọi host đều nằm trong bảng này. UI chỉ gọi qua
`frontend/src/platform/desktop.ts`, không chạm `window.go` trong component.

Namespace: `window.go.main.App.<Method>`.

## Method

| Method | Tệp | Trạng thái |
| --- | --- | --- |
| `BackendReady()` | `bind_bridge.go` | đã có |
| `EngineStatus()` | `bind_engine.go` | đã có |
| `StartEngine()` | `bind_engine.go` | đã có |
| `StopEngine()` | `bind_engine.go` | đã có |
| `ReconnectEngine()` | `bind_engine.go` | đã có |
| `SelectWorkspace()` | `bind_dialog.go` | đã có, native folder picker |
| `ListRecentWorkspaces()` | `bind_workspace.go` | đã có |
| `OpenWorkspace(path)` | `bind_workspace.go` | đã có |
| `CurrentWorkspace()` | `bind_workspace.go` | đã có |
| `ListSessions()` | `bind_session.go` | đã có |
| `CreateSession(title)` | `bind_session.go` | đã có |
| `SwitchSession(id)` | `bind_session.go` | đã có |
| `SessionMessages(id)` | `bind_session.go` | đã có |
| `SendPrompt(id, text)` | `bind_session.go` | đã có |
| `CancelPrompt(id)` | `bind_session.go` | đã có |
| `AnswerPermission(id, decision)` | `bind_permission.go` | đã có |
| `AnswerQuestion(id, answers)` | `bind_permission.go` | đã có |
| `ChangedFiles(id)` | `bind_changes.go` | đã có |
| `FileDiff(id, path)` | `bind_changes.go` | đã có |
| `OpenTerminal(cwd)` | `bind_terminal.go` | đã có, lazy |
| `WriteTerminal(id, data)` | `bind_terminal.go` | đã có, lazy |
| `ResizeTerminal(id, cols, rows)` | `bind_terminal.go` | đã có, lazy |
| `CloseTerminal(id)` | `bind_terminal.go` | đã có, lazy |
| `GetSettings()` | `bind_config.go` | đã có |
| `SaveSettings(settings)` | `bind_config.go` | đã có |

## Sự kiện host tới UI

Hằng số tên sự kiện khai báo trong `internal/uievents/names.go`.

| Sự kiện | Nội dung |
| --- | --- |
| `engine:status` | trạng thái engine đổi |
| `session:delta` | token stream, đã gộp để giảm nhịp render |
| `session:done` | lượt hội thoại kết thúc |
| `tool:activity` | tool đang chạy |
| `permission:request` | yêu cầu duyệt, UI trả lời bằng method bind |
| `question:request` | câu hỏi của agent |
| `changes:updated` | danh sách file thay đổi |
| `terminal:data` | output PTY, chỉ khi terminal mở |
| `terminal:exit` | PTY kết thúc kèm exit code |

Đổi tên method hoặc sự kiện là thay đổi hợp đồng: sửa bảng này trong cùng commit.
