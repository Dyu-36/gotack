# Wails Bindings Contract

The desktop host exposes exactly one bound object: `window.go.main.App.*`
(the `App` struct in package `main`). The UI reaches it only through
`frontend/src/platform/desktop.ts`; method names here mirror that module.
Host-to-UI events are declared once in `internal/uievents/names.go`.
`frontend/src/platform/events.generated.ts` is generated from that file by
`go run ./internal/uievents/gen/main.go` and must never be hand-edited. CI runs the
generator and fails when the generated file drifts, so regenerate it in the
same change as an event rename before pushing. `bindings_contract_test.go`
reflects the exported `*App` method set and fails when a host-only callback or
an undocumented method enters the Wails surface.

## UI -> Host (bound methods)

### Bridge

| Method | Result | Notes |
| --- | --- | --- |
| `BackendReady()` | `bool` | True once the host can serve UI calls. Currently returns a constant `true`, since Wails only binds the object after startup completes. |
| `GetAutoStart()` | `bool` | True when the per-user launch-at-login entry (`HKCU\...\CurrentVersion\Run`, value `Tack`) exists. |
| `SetAutoStart(enabled)` | `error` | Creates or deletes that entry. The entry always launches `tack.exe --hidden`, so a login start rests in the tray. |

### Engine

| Method | Result | Notes |
| --- | --- | --- |
| `EngineStatus()` | `EngineInfo` | No side effects. |
| `StartEngine()` | `EngineInfo` | Attaches or launches Crush; progress via `engine:status`. |
| `StopEngine()` | `EngineInfo` | Kills the child only if this host launched it. |
| `ReconnectEngine()` | `error` | Re-dials without terminating agent work. |

`EngineInfo`: `{status: stopped|starting|running|error, running, endpoint, version, owned, error?}`.

`error` is independent of `status`. A `running` engine still reports `error`
when the default workspace could not be attached, most often because a stored
provider credential is unusable. The UI must stay interactive in that state --
Settings is the only place the cause can be fixed -- and retries the attach
with `EnsureAssistantWorkspace()`.

The desktop host starts or adopts the engine during `OnStartup`, before the UI
requests a workspace. Normal UI shutdown disconnects host-owned streams and
terminals but leaves the engine process running, so the next Gotack launch
adopts the warm engine. `StopEngine()` remains the explicit process-stop path
while the current host owns the process.

Window lifecycle: the Wails `HideWindowOnClose` option keeps the process alive
when the user closes the window -- the window only hides into the
notification-area icon (`tray_windows.go`), which restores it. There is no
in-app quit path; the process ends when Windows ends it (Task Manager, logoff,
shutdown). `SingleInstanceLock` makes a second launch surface the first
instance's window, except when that launch carries `--hidden` (the autostart
entry), which leaves the running instance hidden.

### Workspace

| Method | Result | Notes |
| --- | --- | --- |
| `ListRecentWorkspaces()` | `string[]` | Most recent first. |
| `OpenWorkspace(path)` | `WorkspaceInfo` | Attaches in engine, switches event stream, reapplies saved settings, and registers the bundled runtime services and skills. |
| `EnsureAssistantWorkspace()` | `WorkspaceInfo` | Attaches the always-available default workspace (`C:\` on Windows) so startup chat has a real session context. |
| `CurrentWorkspace()` | `WorkspaceInfo?` | Null when nothing is attached. |
| `SelectWorkspace()` | `string` | Native directory picker; empty on cancel. |
| `OpenGeneratedFile(path)` | `error` | Opens an existing allowlisted document/image with the OS default application. Rejects relative, device, network, directory, executable, and missing paths. |
| `RevealGeneratedFile(path)` | `error` | Opens the containing folder and selects the same validated file. |

Security-relevant default: every workspace keeps Crush permission prompts
enabled. Only an explicit saved `"auto_approve": true` setting skips them. The
default assistant workspace is the drive root, so changes to workspace
attachment must preserve this opt-in posture and keep it documented here and
in `README.md`.

### Sessions

| Method | Result | Notes |
| --- | --- | --- |
| `ListSessions()` | `SessionInfo[]` | Sessions of the active workspace. |
| `CreateSession(title)` | `SessionInfo` | Empty title becomes "New session". |
| `RenameSession(id, title)` | `SessionInfo` | Persisted via engine PUT. |
| `DeleteSession(id)` | `error` | UI selects another session afterwards. |
| `SwitchSession(id)` | `error` | Advisory current-session update. |
| `SessionMessages(id)` | `MessageInfo[]` | History replay includes `model`, `provider`, attachment metadata, image content, and `tool_calls`, so selecting or restoring a conversation reapplies its latest model, restores file/image chips, and rebuilds tool rows; live updates come from events. |
| `SendPrompt(id, text, attachments)` | `runID`, `error` | Starts one agent turn after workspace/session/model readiness. `attachments` is `PromptAttachment[]`; each file is limited to 5 MiB. Text extracted from a file travels inside the prompt text; only bytes the model consumes natively (vision images) are forwarded as Crush attachments. |
| `CancelPrompt(id)` | `error` | Interrupts the running turn. |
| `PickPromptFiles()` | `PromptFilePick[]`, `error` | Native multi-file picker. Returns `{file_name, mime_type, size, path}` per file; empty on cancel. The webview never reads the bytes. |
| `AttachmentLimits()` | `AttachmentLimitsInfo` | `{max_bytes, max_derived_lines, max_derived_bytes}` from `internal/appconfig`, so the composer enforces the same cap as the host. |

`PromptAttachment`: `{file_name, mime_type?, content?, path?}`. `file_name` is
reduced to its basename using either Windows or POSIX separators before it is
stored or replayed. `content` is standard base64 (upload or pasted image);
`path` is an absolute host path for a
file chosen with `PickPromptFiles()`, dropped on the window, or written in the
prompt as an `@[C:\dir\file.xlsx]` tag. Exactly one of the two is required and
`path` wins when both are sent, so a multi-megabyte spreadsheet never crosses the
webview as base64. `SendPrompt` removes `@[...]` and `@"..."` tags from the
visible text and turns each one into an attachment.
When `text` is empty but attachments are provided, the backend automatically composes
a Vietnamese review prompt ("Hãy xem và xử lý tệp/các tệp đính kèm sau:").
`MessageInfo.attachments[]`: `{file_name, mime_type, size, content?, path?}`. `path` remains host-only metadata until the user clicks Open or Reveal. The UI
file picker accepts multiple files; pasting clipboard image data into the
composer adds the image to the same pending attachment list.
`MessageInfo.tool_calls[]`: `{id, name, input?, finished}`. Replay reuses the
same `tool:<id>` row identity as the `tool:activity` event so reloaded history
and a live stream converge instead of duplicating rows. Assistant rows with
empty `text` are tool-only agent steps and must not be rendered as bubbles.

Attachment handling is fail-soft: a file that cannot be decoded, converted, or
extracted becomes a warning line inside the prompt and never aborts the turn.
Each accepted file is saved under `%APPDATA%/gotack/attachments/<hash>/<name>`
and wrapped in the prompt as
`<gotack-attachment name mime size path>...</gotack-attachment>`; `SessionMessages`
parses those markers back out, so replayed prompts show clean text plus chips.
Legacy binary formats (`.xls`, `.doc`, `.ppt`, OpenDocument) are converted to
OOXML through LibreOffice `soffice` or, if absent, Microsoft Office COM before
extraction; when neither is available the file still reaches the model as a
path with instructions to use the bundled `officecli` command.

PDF text is extracted with `pdftotext`, then LibreOffice, then Word COM; when
none of them is installed the file still reaches the model as a path plus a
warning line. The attachment cache is trimmed once per launch (`OnStartup` calls
`attachments.PruneCache`): entries older than 14 days go first, then the oldest
remaining entries until the cache fits its size budget. There is no background
sweeper, and the UI no longer polls run state -- conversation status comes from
`session:done` plus one reconcile before the next send.

### Files dropped on the window

`main.go` enables Wails file drop and the host emits `prompt:files` with
`PromptFilePick[]` when files land on the window. The webview renders chips from
that metadata only; the bytes stay on disk until `SendPrompt` reads them.

### Approvals

| Method | Result | Notes |
| --- | --- | --- |
| `AnswerPermission(requestID, decision)` | `bool`, `error` | `decision`: `allow`, `allow_session`, `deny`. |
| `AnswerQuestion(requestID, answers)` | `bool`, `error` | One batch response. Interactive batches expire after 60 seconds; timeout closes the form and makes the agent ask in normal text. |

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
| `SaveSettings(settings)` | `error` | Applies provider/model/thinking/credential through the Crush REST API. Rejects an API key or a custom endpoint for an OAuth-backed provider: `codex signs in with ChatGPT, not an API key; use the openai provider for an API key` and `provider "codex" signs in with OAuth and does not accept a custom endpoint`. |
| `ListProviders()` | `Provider[]` | Live Catwalk catalog for the open workspace plus Gotack-local provider overlays that are omitted automatically once Catwalk publishes the same provider ID. The local overlays are Mistral (`openai-compat`, `https://api.mistral.ai/v1`) with curated model capabilities, and Codex (`openai`, Codex backend) with no models and no API-key field; configured custom-provider models from Crush are merged ahead of the fallback metadata. Without a workspace, the host uses a private catalog workspace and does not change `CurrentWorkspace()`. Requires the engine. |
| `GetProviderUsage(providerID)` | `ProviderUsageInfo` | Returns provider-defined quota windows without exposing credentials. Codex reads the signed-in ChatGPT account's five-hour, weekly, and any additional windows; providers without an account-usage endpoint return `available: false` instead of an estimated balance. |
| `RevealProviderAPIKey(providerID)` | `string`, `error` | Returns the stored key for one provider so Settings can reveal it on explicit user action. Deliberate exception to the write-only secret rule below. |
| `DeleteProvider(providerID)` | `error` | Removes the provider's stored credential and configuration from the Crush config. |
| `LoginChatGPTOAuth()` | `ChatGPTOAuthStatus`, `error` | Launches OpenAI's browser OAuth PKCE flow for ChatGPT accounts (Free/Go/Plus/Pro/Business/Edu/Enterprise), seeds the `codex` provider, stores the credential and account-routing metadata there, loads the models available to that account from the Codex backend, and selects the first available model when the previous selection is unavailable. |
| `GetChatGPTOAuthStatus()` | `ChatGPTOAuthStatus`, `error` | Returns connection state, email, plan, and expiry for the `codex` provider. A malformed credential or one without a ChatGPT account ID is disconnected; an expired credential with a refresh token is refreshed through Crush before status is returned. A pre-split login stored on `openai` is moved to `codex` first (see below). |
| `LogoutChatGPTOAuth()` | `error` | Removes the `codex` provider's credential and configuration from Crush. The `openai` provider and its API key are untouched. |

`ProviderUsageInfo`:

`{provider_id, provider_name, available, plan?, limit_reached, windows, updated_at, unavailable_reason?}`.

Each `windows[]` entry is
`{id, name?, used_percent, remaining_percent, window_seconds?, resets_at?}`.
`resets_at` and `updated_at` are Unix milliseconds. The values are percentages
because subscription providers do not expose a reliable absolute token balance.
The host keeps the OAuth token and account-routing headers out of the webview.
The badge refreshes when the selected provider changes, when `session:done`
arrives, when the user opens or manually refreshes the popover, and never by a
background polling loop.

**Provider split.** ChatGPT subscription sign-in and the public OpenAI API are
two separate providers, because one provider ID holds exactly one credential in
Crush and storing an OAuth token rewrites that provider's endpoint and catalog:

- `openai` -- public OpenAI API, API key only. Accepts a custom endpoint.
- `codex` ("ChatGPT (Codex)") -- ChatGPT subscription, browser OAuth only. Its
  endpoint is fixed to the Codex backend and its catalog is account-scoped.

Installs that signed in before the split keep their credential on `openai`. The
host moves it to `codex` automatically, once, on engine connect and on the next
`GetChatGPTOAuthStatus()`, and strips the OAuth leftovers from `openai`. No
second sign-in is required.

Whichever of the two paths performs the move also repoints the selection in the
same pass, because a saved selection naming `openai` is replayed into the engine
by every later settings apply and would restore the broken routing:

- `models.large` / `models.small` and the saved provider move to `codex`.
- The saved model id is validated against the account-scoped Codex catalog and
  falls back to that provider's default large model when it is not offered.
- A saved custom endpoint is cleared, since `codex` rejects one.
- A provider the user selected deliberately is left alone; only an empty,
  `openai`, or already-`codex` selection is taken over.
- An install where the credential already reached `codex` while the selection
  stayed on `openai` is converged on the next connect. The migration itself no
  longer sees anything to move there, so the repoint is gated on `openai`
  holding no credential at all -- an API key on `openai` keeps its selection.

The same guard runs on every settings apply, not only during the migration. The
webview replays the selection it read at boot, so a payload that still names
`openai` while that provider holds no credential is repointed at `codex` before
it reaches the engine, and `SaveSettings` persists what was applied rather than
what was sent. Three writes are never redirected: one carrying an API key, a
provider-only write, and one naming an `openai` provider that already holds a
key.

The migration is best effort: a failure is logged and retried on the next status
check instead of blocking startup.

`ChatGPTOAuthStatus`: `{connected, email?, plan?, expires_at?}`.

`SettingsInfo`:

`{theme, provider, credential_provider?, provider_only?, model, thinking, api_key, custom_url}`.

`small_model` and `autostart_engine` were previously on the wire with no effect
and have been removed from the Go struct, `desktop.ts` and this contract under
hard rule 8. There is nothing to migrate: Settings shows one model selector and
`applyCrushSettings` sets `models.large` and `models.small` to the same model ID,
and `OnStartup` always adopts or starts the engine, so neither field ever had a
value worth persisting. `encoding/json` ignores unknown members, so an existing
`config.json` still loads.

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

The packaged runtime contains `officecli.exe`, the OfficeCLI skill tree, the
`timetable` skill and its formatted Excel template. At startup the host copies
the runtime and skills into `<configDir>/bin/` and `<configDir>/skills/`, adds
the bin directory to `PATH`, and merges `options.skills_paths`. It also removes
the retired `mcp_servers.gotack-office` entry from existing workspace configs.
No Office MCP server or Office tool set is registered.

The timetable skill accepts natural-language requirements and reads the source
workbook directly. The agent may create task-specific scratch Python scripts and
models the current problem directly with the bundled Python + OR-Tools CP-SAT
environment; there is no timetable-specific `problem.json`, schema, or bundled
solver runner. User/source requirements remain the source of truth: every hard
constraint must be represented in the model and independently checked again
after solving. `FEASIBLE`/`OPTIMAL` only proves the constraints actually encoded
in that model. After the source-level validation passes, the agent writes the six
normalized columns into `Dữ liệu` in `assets/mau-thoi-khoa-bieu.xlsx` and
re-opens the workbook before delivery.

## Host -> UI events

| Event | Payload |
| --- | --- |
| `engine:status` | `EngineInfo` |
| `prompt:files` | `PromptFilePick[]` |
| `session:delta` | `{session_id, message_id, text, append, seq}` — `seq` starts at 1 per message; a gap forces a frontend resync from `text` |
| `session:done` | `{session_id, text?, error?, cancelled}` |
| `tool:activity` | `{session_id, name, input, finished, tool_call_id}` |
| `task:progress` | `{session_id, run_id?, state, elapsed_seconds, limit_seconds, solutions?, penalty?, result_status?, hard_constraints_satisfied?, soft_violation_count?}` — business task lifecycle only; no shell/PID details |
| `permission:request` | `PermissionRequest` |
| `question:request` | `QuestionRequest` |
| `question:resolved` | `{batch_id}` — closes the matching form after answer, cancel, or timeout |
| `changes:updated` | `{session_id, path}` |
| `terminal:data` | `{id, data}` |
| `terminal:exit` | `{id, code?, error?}` |

## Rules

1. Bound methods stay in package `main`; arguments and results are
   JSON-serializable. Every other host callback stays unexported or is passed
   as a function value. Update the exact allowlist in `bindings_contract_test.go`
   only when this contract gains or loses a bound method.
2. Secrets are write-only across this boundary, with one deliberate exception:
   `RevealProviderAPIKey` returns a stored provider key on explicit user
   action. The Zalo token is never returned; `api_key` is always empty on read.
   Any new secret-returning method must be listed here.
3. Update this document, `desktop.ts`, `internal/uievents/names.go`, and the
   Go binds in the same change, then regenerate `events.generated.ts`.
4. A field documented here as having no effect must either be implemented or
   removed from `SettingsInfo` and `desktop.ts`; do not leave a third state
   where the UI believes it works.
