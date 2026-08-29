# Wails Bindings Contract

The desktop host exposes exactly one bound object: `window.go.main.App.*`
(the `App` struct in package `main`). The UI reaches it only through
`frontend/src/platform/desktop.ts`; method names here mirror that module.
Host-to-UI events are declared once in `internal/uievents/names.go` and
mirrored by the `events` map in `desktop.ts`.

## UI -> Host (bound methods)

### Bridge

| Method | Result | Notes |
| --- | --- | --- |
| `BackendReady()` | `bool` | True once the host can serve UI calls. |

### Engine

| Method | Result | Notes |
| --- | --- | --- |
| `EngineStatus()` | `EngineInfo` | No side effects. |
| `StartEngine()` | `EngineInfo` | Attaches or launches Crush; progress via `engine:status`. |
| `StopEngine()` | `EngineInfo` | Kills the child only if this host launched it. |
| `ReconnectEngine()` | `error` | Re-dials without terminating agent work. |

`EngineInfo`: `{status: stopped|starting|running|error, running, endpoint, version, owned, error?}`.

### Workspace

| Method | Result | Notes |
| --- | --- | --- |
| `ListRecentWorkspaces()` | `string[]` | Most recent first. |
| `OpenWorkspace(path)` | `WorkspaceInfo` | Attaches in engine, switches event stream, reapplies saved settings, registers the office MCP server. |
| `CurrentWorkspace()` | `WorkspaceInfo?` | Null when nothing is attached. |
| `SelectWorkspace()` | `string` | Native directory picker; empty on cancel. |

### Sessions

| Method | Result | Notes |
| --- | --- | --- |
| `ListSessions()` | `SessionInfo[]` | Sessions of the active workspace. |
| `CreateSession(title)` | `SessionInfo` | Empty title becomes "New session". |
| `RenameSession(id, title)` | `SessionInfo` | Persisted via engine PUT. |
| `DeleteSession(id)` | `error` | UI selects another session afterwards. |
| `SwitchSession(id)` | `error` | Advisory current-session update. |
| `SessionMessages(id)` | `MessageInfo[]` | History replay; live updates come from events. |
| `SendPrompt(id, text)` | `runID`, `error` | Starts one agent turn. |
| `CancelPrompt(id)` | `error` | Interrupts the running turn. |

### Approvals

| Method | Result | Notes |
| --- | --- | --- |
| `AnswerPermission(requestID, decision)` | `bool`, `error` | `decision`: `allow`, `allow_session`, `deny`. |
| `AnswerQuestion(requestID, answers)` | `bool`, `error` | One batch response. |

### Changes and terminal

| Method | Result | Notes |
| --- | --- | --- |
| `ChangedFiles(sessionID)` | `FileStatus[]` | Latest version per path. |
| `FileDiff(sessionID, path)` | `string` | Capped unified diff. |
| `OpenTerminal(cwd)` | `id`, `error` | Lazy PTY; output via events. |
| `WriteTerminal(id, data)` | `error` | |
| `ResizeTerminal(id, cols, rows)` | `error` | |
| `CloseTerminal(id)` | `error` | |

### Settings and catalog

| Method | Result | Notes |
| --- | --- | --- |
| `GetSettings()` | `SettingsInfo` | `api_key` is always empty (write-only). |
| `SaveSettings(settings)` | `error` | Applies provider/model/thinking/credential through the Crush REST API. `small_model` selects the small-task model. |
| `ListProviders()` | `Provider[]` | Live catwalk catalog for the open workspace. Without one, the host uses a private catalog workspace and does not change `CurrentWorkspace()`. Requires the engine. |

### Zalo connection

| Method | Result | Notes |
| --- | --- | --- |
| `GetZaloConfig()` | `ZaloConfigInfo` | `has_token` instead of the secret. |
| `SaveZaloConfig(update)` | `ZaloStatus`, `error` | Empty `token` keeps the stored one; validates via `getMe` and restarts the bridge. |
| `ZaloStatus()` | `ZaloStatus` | Bridge health plus the most recent inbound message. |

The Zalo bridge polls the official Zalo Bot API (`getMe`, long-poll
`getUpdates`, `sendMessage`), serves only allow-listed chats, and replies to a
chat when the agent run it started completes.

## Host -> UI events

| Event | Payload |
| --- | --- |
| `engine:status` | `EngineInfo` |
| `session:delta` | `{session_id, message_id, text, append, seq}` — `seq` starts at 1 per message; a gap forces a frontend resync from `text` |
| `session:done` | `{session_id, text?, error?, cancelled}` |
| `tool:activity` | `{session_id, name, input, finished, tool_call_id}` |
| `permission:request` | `PermissionRequest` |
| `question:request` | `QuestionRequest` |
| `changes:updated` | `{session_id, path}` |
| `terminal:data` | `{id, data}` |
| `terminal:exit` | `{id, code?, error?}` |

## Rules

1. Bound methods stay in package `main`; arguments and results are
   JSON-serializable.
2. Secrets are write-only across this boundary (`api_key`, Zalo token).
3. Update this document, `desktop.ts`, `internal/uievents/names.go`, and the
   Go binds in the same change.
