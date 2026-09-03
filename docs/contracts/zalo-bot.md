# Zalo Bot Contract

Gotack connects the local agent to Zalo chats through the official Zalo Bot
API. This document is the contract for that external boundary (AGENTS.md hard
rule 7); the implementation lives in `internal/zalo` with the UI surface in
`bind_zalo.go`. Update this document in the same change as either.

The channel is a transport only: inbound chat text becomes one Crush agent
run submitted over the same REST path the UI uses, and the answer returns
over the same SSE `run_complete` event through the forwarder's host callback. The
desktop never executes agent logic itself (ADR 0001).

## External API surface

The host talks HTTPS to the Zalo Bot API:

- Base URL `https://bot-api.zaloplatforms.com`, overridable with the
  `ZALO_BOT_API_BASE` environment variable (test seam).
- Every call is `POST {base}/bot{token}/{method}` with a JSON body and an
  envelope response `{ok, result, error_code, description}`. Non-2xx or
  `ok: false` becomes a typed `zalo.APIError`; the token is redacted from
  every error string.
- Methods consumed: `getMe` (token validation and bot name), `getUpdates`
  (long polling), `sendMessage` (markdown attempted first for URL-free text,
  retried plain), `sendChatAction` (best effort), `sendPhoto`, and
  `deleteWebhook` (called on token set/test and worker start so long polling
  stays authoritative for the token).
- HTTP client: 120 s timeout, one retry on deadline/EOF transport errors,
  404-style poll timeouts (HTTP 408 error code) treated as "no updates".

Inbound updates are parsed defensively: array or single-update payloads,
`chat_id`/`chat.id` fallbacks, and a loose attachment-URL search across
common key names. Messages from unpaired chats are ignored, except `/pair`.

## Config keys consumed

`config.json` (`internal/appconfig`), `zalo` object:

| Key | Status | Meaning |
| --- | --- | --- |
| `zalo.enabled` | live | the on/off switch the UI edits via `SaveZaloConfig` |
| `zalo.token` | **deprecated** (removal target Gotack v1.0) | legacy bot token; consumed only by the one-shot `ImportLegacy` migration below |
| `zalo.allowed_chats` | **deprecated** (removal target Gotack v1.0) | legacy allow-list; consumed only by `ImportLegacy` |

The live channel state lives in `<appconfig.Dir()>/zalo.json`, written
atomically (temp file plus rename, mode 0600) by the host only:

```json
{
  "token": "***",
  "bot_name": "My Bot",
  "pairing_code": "123456",
  "paired_chat_ids": ["user_id_1"],
  "update_offset": 4711,
  "chat_sessions": { "user_id_1": "session_id_1" }
}
```

| Field | Meaning |
| --- | --- |
| `token` | the bot secret; never crosses the Wails boundary (the UI sees only a boolean plus the last four characters) |
| `pairing_code` | six-digit code, rotated after every successful pairing and on demand |
| `paired_chat_ids` | chats allowed to talk to the bot |
| `update_offset` | persisted long-poll cursor; survives restarts |
| `chat_sessions` | durable chat-id to Crush-session-id map |

A malformed file never blocks desktop startup: the manager opens with empty
state and surfaces the parse error through `Status.LastError`.

## Pairing

`/pair <code>` from a chat validates against the current six-digit code,
appends the chat to `paired_chat_ids`, rotates the code, and replies with
the help text. The UI can rotate the code (`RegenerateZaloPairingCode`) and
revoke one chat (`UnpairZaloChat`, which also drops that chat's session
mapping). Everything else from an unpaired chat is dropped.

## Session mapping

One chat runs at most one turn at a time. A normal message:

1. Reuses `chat_sessions[chat_id]` when present; otherwise the host creates
   a session titled `Zalo: <chat_id>` through `internal/session`.
2. Marks the session in the unattended roster **before** any prompt runs
   (`guard.MarkUnattendedSession`, same file as scheduled runs), so the
   guard denies ask-tier operations with a legible reason instead of hanging
   on a prompt nobody can answer (ADR 0002, `gotack-approvals.md`). A failed
   mark fails the turn.
3. Denies the interactive `question` tool with rule `unattended-question`.
   The tool result instructs the model to ask for missing information in the
   ordinary assistant reply and end the turn. Zalo does not render or answer
   desktop question forms; the user's next chat message starts the next turn.
4. Submits the text over the engine REST path and remembers the session id.

Answers flow back through the unexported host completion callback into
`Manager.Done`, which matches
the session id to its chat and delivers the reply. `/new` forgets the chat's
mapping (next message creates a fresh session), and switching workspaces in
the UI calls `ResetSessions` because sessions belong to one workspace.

## Media handling

- **Inbound**: when an update carries an attachment URL, the file is
  downloaded into `%TEMP%/gotack-zalo-inbox` with the same 45 MiB bound as
  outbound files, then the prompt is extended with
  `Tệp Zalo đã tải về máy tại: <path>` (or a default file prompt when the
  message had no text).
- **Outbound**: a completed answer is scanned for markdown link targets and
  Windows paths; up to 5 existing, sendable files per turn are uploaded and
  pushed. Uploads try Litterbox, Tmpfiles, 0x0.st, and File.io in order
  (Litterbox skipped for blocked executable-like extensions). Images go
  through `sendPhoto`, other files as a link message. Reply text is
  sanitized first: media markdown and path lines are stripped, and the
  remainder is chunked at 1800 characters.
- **Sendable extensions**: png, jpg, jpeg, webp, gif, bmp, pdf, xlsx, xls,
  csv, docx, doc, pptx, ppt, txt, zip, mp4; non-empty and ≤ 45 MiB.
- **Commands**: `/send <name|path>` and `/files` resolve files against the
  current workspace (root, `output/`, `input/`); `/screenshot` captures the
  desktop through PowerShell on Windows only and sends it as a photo.

## Legacy import path

`Manager.ImportLegacy(token, allowed)` runs once at startup with the
deprecated `zalo.token` and `zalo.allowed_chats` values. It migrates only
when no channel token exists yet, then stores the token, mints a pairing
code, and treats the allow-list as pre-paired chats. Existing channel state
always wins, and the behavior is frozen until removal: at the Gotack v1.0
removal target the two config fields, the call site in `app.go` startup, and
`ImportLegacy` itself are dropped together. `RemoveZaloToken` additionally
clears both legacy keys from `config.json` when the user disconnects.

## UI surface

The Wails-bound methods (`GetZaloConfig`, `SaveZaloConfig`,
`TestZaloConnection`, `RemoveZaloToken`, `RegenerateZaloPairingCode`,
`UnpairZaloChat`, `ZaloStatus`, `SendZaloFile`) are documented in
`wails-bindings.md`; this contract covers the external Bot API side they
drive.
