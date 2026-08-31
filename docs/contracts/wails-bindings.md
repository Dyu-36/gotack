# Wails Bindings Contract

The desktop host exposes exactly one bound object: `window.go.main.App.*`
(the `App` struct in package `main`). The UI reaches it only through
`frontend/src/platform/desktop.ts`; method names here mirror that module.
Host-to-UI events are declared once in `internal/uievents/names.go`.
`frontend/src/platform/events.generated.ts` is generated from that file by
`go run ./internal/uievents/gen` and must never be hand-edited. No workflow
runs the generator, so regenerate it in the same change as an event rename.

## UI -> Host (bound methods)

### Bridge

| Method | Result | Notes |
| --- | --- | --- |
| `BackendReady()` | `bool` | True once the host can serve UI calls. Currently returns a constant `true`, since Wails only binds the object after startup completes. |

### Engine

| Method | Result | Notes |
| --- | --- | --- |
| `EngineStatus()` | `EngineInfo` | No side effects. |
| `StartEngine()` | `EngineInfo` | Attaches or launches Crush; progress via `engine:status`. |
| `StopEngine()` | `EngineInfo` | Kills the child only if this host launched it. |
| `ReconnectEngine()` | `error` | Re-dials without terminating agent work. |

`EngineInfo`: `{status: stopped|starting|running|error, running, endpoint, version, owned, error?}`.

The desktop host starts or adopts the engine during `OnStartup`, before the UI
requests a workspace. Normal UI shutdown disconnects host-owned streams and
terminals but leaves the engine process running, so the next Gotack launch
adopts the warm engine. `StopEngine()` remains the explicit process-stop path
while the current host owns the process.

### Workspace

| Method | Result | Notes |
| --- | --- | --- |
| `ListRecentWorkspaces()` | `string[]` | Most recent first. |
| `OpenWorkspace(path)` | `WorkspaceInfo` | Attaches in engine, switches event stream, reapplies saved settings, registers the office MCP server. |
| `EnsureAssistantWorkspace()` | `WorkspaceInfo` | Attaches the always-available default workspace (`C:\` on Windows) so startup chat has a real session context. |
| `CurrentWorkspace()` | `WorkspaceInfo?` | Null when nothing is attached. |
| `SelectWorkspace()` | `string` | Native directory picker; empty on cancel. |

Security-relevant default: every workspace the host attaches is opened with
Crush permission prompts skipped, and the default assistant workspace is the
drive root. This is a deliberate single-user desktop trade-off. Any change to
workspace attachment must keep this documented here and in `README.md`.

### Sessions

| Method | Result | Notes |
| --- | --- | --- |
| `ListSessions()` | `SessionInfo[]` | Sessions of the active workspace. |
| `CreateSession(title)` | `SessionInfo` | Empty title becomes "New session". |
| `RenameSession(id, title)` | `SessionInfo` | Persisted via engine PUT. |
| `DeleteSession(id)` | `error` | UI selects another session afterwards. |
| `SwitchSession(id)` | `error` | Advisory current-session update. |
| `SessionMessages(id)` | `MessageInfo[]` | History replay includes `model`, `provider`, attachment metadata, and image content so selecting or restoring a conversation reapplies its latest model and restores file/image chips; live updates come from events. |
| `SendPrompt(id, text, attachments)` | `runID`, `error` | Starts one agent turn after workspace/session/model readiness. `attachments` is `PromptAttachment[]`; each file is limited to 5 MiB and forwarded to Crush's native attachment contract. |
| `CancelPrompt(id)` | `error` | Interrupts the running turn. |

`PromptAttachment`: `{file_name, mime_type?, content}` where `content` is standard base64.
When `text` is empty but attachments are provided, the backend automatically composes
a Vietnamese review prompt ("Hãy xem và xử lý tệp/các tệp đính kèm sau:").
`MessageInfo.attachments[]`: `{file_name, mime_type, size, content?}`. The UI
file picker accepts multiple files; pasting clipboard image data into the
composer adds the image to the same pending attachment list.

### Approvals

| Method | Result | Notes |
| --- | --- | --- |
| `AnswerPermission(requestID, decision)` | `bool`, `error` | `decision`: `allow`, `allow_session`, `deny`. |
| `AnswerQuestion(requestID, answers)` | `bool`, `error` | One batch response. |

### Changes and terminal

| Method | Result | Notes |
| --- | --- | --- |
| `ChangedFiles(sessionID)` | `FileStatus[]` | Latest version per path. Returns `changes.FileStatus` directly with no bind-layer DTO; `desktop.ts` mirrors the same wire shape under the name `ChangedFileInfo`. |
| `FileDiff(sessionID, path)` | `string` | Capped unified diff (256 KiB). |
| `OpenTerminal(cwd)` | `id`, `error` | Lazy PTY; output via events. |
| `WriteTerminal(id, data)` | `error` | |
| `ResizeTerminal(id, cols, rows)` | `error` | Rejects sizes outside 1..1000. |
| `CloseTerminal(id)` | `error` | |

### Settings and catalog

| Method | Result | Notes |
| --- | --- | --- |
| `GetSettings()` | `SettingsInfo` | `api_key` is always empty (write-only). |
| `SaveSettings(settings)` | `error` | Applies provider/model/thinking/credential through the Crush REST API. |
| `ListProviders()` | `Provider[]` | Live catwalk catalog for the open workspace. Without one, the host uses a private catalog workspace and does not change `CurrentWorkspace()`. Requires the engine. |
| `RevealProviderAPIKey(providerID)` | `string`, `error` | Returns the stored key for one provider so Settings can reveal it on explicit user action. Deliberate exception to the write-only secret rule below. |
| `DeleteProvider(providerID)` | `error` | Removes the provider's stored credential and configuration from the Crush config. |

`SettingsInfo`:
`{theme, autostart_engine, provider, credential_provider?, provider_only?, model, small_model, thinking, api_key, custom_url}`.

Two fields are on the wire but have no effect today. They are kept for
compatibility with the current UI type; do not build behavior on them without
implementing them first.

| Field | Actual behavior |
| --- | --- |
| `small_model` | Accepted and discarded. `SaveSettings` stores `model` into the small slot, and the Crush config write sets `models.large` and `models.small` to the same model ID. Settings intentionally shows one model selector. |
| `autostart_engine` | Forced to `true` on both read and write. The host always adopts or starts the engine during `OnStartup`, so the toggle cannot be turned off. |

### Zalo connection

| Method | Result | Notes |
| --- | --- | --- |
| `GetZaloConfig()` | `ZaloConfigInfo` | `has_token` instead of the secret; exposes `paired_chats` and the rotating `pairing_code`. |
| `SaveZaloConfig(update)` | `ZaloStatus`, `error` | Empty `token` keeps the stored one; validates via `getMe` and restarts the bridge. |
| `TestZaloConnection()` | `ZaloStatus`, `error` | Re-checks `getMe` against the stored token. |
| `RemoveZaloToken()` | `ZaloStatus`, `error` | Wipes channel state and stops the bridge. |
| `RegenerateZaloPairingCode()` | `ZaloStatus`, `error` | Issues a fresh six-digit pairing code. |
| `UnpairZaloChat(chatID)` | `ZaloStatus`, `error` | Revokes one chat and forgets its session. |
| `ZaloStatus()` | `ZaloStatus` | Live bridge health, bot identity, paired chats, last error. |
| `SendZaloFile(req)` | `string`, `error` | Pushes a local file to a paired chat (or every paired chat when `chat_id` is empty). |

The Zalo bridge polls the official Zalo Bot API (`getMe`, long-poll
`getUpdates`, `sendMessage`, `sendPhoto`, `sendChatAction`, `deleteWebhook`),
serves only chats that paired via `/pair <6-digit code>`, and replies to a
chat when the agent run it started completes. Inbound media accepts the direct
and nested image URL fields used by the Bot API (`photo`, `photo_url`,
`image_url`, `picture_url`, and attachment objects), downloads the file into
the host's temporary Zalo inbox, and includes its local path in the agent turn.
Sessions persist across desktop restarts under `<configDir>/zalo.json` and are re-bound
to the matching chat when the bridge reconnects.

Access control is pairing-only. The legacy `zalo.allowed_chats` config key is
imported once at startup for backwards compatibility and is never written
again; it is not an allow-list the UI can manage.

### Bundled Office and timetable integration

The packaged runtime under `build/bin/resources/` contains `officecli.exe`, a
Python 3.12 runtime with `ortools` and `openpyxl`, the
`officecli-xlsx|docx|pptx|...` skill tree, and the `timetable` skill copied
from the Stack 2.2.0 implementation (schema, solver and exporter included).
At startup the host copies the runtime and skills into `<configDir>/bin/` and
`<configDir>/skills/`, prepends the bin directory to `PATH`, and registers the
Office MCP server plus `options.skills_paths` through the Crush config
endpoints. Crush can therefore invoke the Office MCP tools or execute the
timetable solver/exporter with the bundled `python` command without separate
user setup.

Platform limit: the bundle is located by looking for `officecli.exe`, so
seeding only resolves on Windows. On other platforms the Office MCP server is
still registered but the bundled runtime and skills are not seeded.

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
2. Secrets are write-only across this boundary, with one deliberate exception:
   `RevealProviderAPIKey` returns a stored provider key on explicit user
   action. The Zalo token is never returned; `api_key` is always empty on read.
   Any new secret-returning method must be listed here.
3. Update this document, `desktop.ts`, `internal/uievents/names.go`, and the
   Go binds in the same change, then regenerate `events.generated.ts`.
4. A field documented here as having no effect must either be implemented or
   removed from `SettingsInfo` and `desktop.ts`; do not leave a third state
   where the UI believes it works.
