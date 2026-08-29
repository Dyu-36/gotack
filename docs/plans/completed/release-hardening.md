# Release Hardening: Real Integrations, No Demo Paths

## Outcome

Shipped gotack as a release candidate with: (1) every UI flow wired to the real
backend with no demo/preview state, (2) a built-in Zalo connection so the
desktop agent receives requests and returns results remotely, (3) a built-in
Office integration so the agent can inspect, create, and edit Word, Excel, and
PowerPoint files, and (4) clean, English-only, dead-code-free sources.

## Context

- The UI still contained a preview conversation store
  (`conversation-state.svelte.ts`) with seeded conversations and a fake
  assistant reply (`sendPreviewMessage`), plus a hardcoded provider/model
  catalog that pretended to be Crush data.
- Crush exposes `GET /v1/workspaces/{id}/providers` (catwalk provider catalog
  with models), so the catalog could be made real.
- Zalo ships an official Bot API (`https://bot-api.zaloplatforms.com`,
  `POST /bot<token>/<method>`; `getMe`, long-poll `getUpdates` returning one
  update, `sendMessage {chat_id, text}`; envelope `{ok, result, error_code,
  description}`). This gives a real two-way channel from a desktop app with no
  public webhook requirement.
- Crush supports stdio MCP servers via config key
  `mcp_servers.<name> = {command, args, env, type, timeout}` (hot-reloaded),
  so a bundled `office` MCP binary can be registered per workspace through the
  existing config mutation API.

## Approach

1. **Live catalog**: added `crushapi.ListProviders` + bound `ListProviders`;
   the frontend loads providers/models from the engine after workspace attach
   (`features/conversations/catalog.svelte.ts`). Deleted
   `conversation-state.svelte.ts` (demo seeds, fake replies, hardcoded
   catalog). Composer and SettingsModal consume the live catalog. Model picks
   now carry real context windows and costs.
2. **Zalo bridge**: `internal/zalo` (Bot API client + polling bridge with
   allow-list, message dedupe, per-chat single-flight, chat-to-session
   mapping, reply on run completion), a `DoneSink` on the UI event forwarder,
   binds `GetZaloConfig` / `SaveZaloConfig` / `ZaloStatus` (token is
   write-only), and a Zalo section in Settings.
3. **Office MCP**: `internal/office` (xlsx via excelize; docx/pptx via minimal
   OOXML; shared markdown-subset source) + `cmd/office` stdio MCP server
   exposing `office_info`, `office_read`, `office_create`, `office_edit`;
   gotack registers `mcp_servers.gotack-office` (workspace scope) on workspace
   open when the bundled binary exists and unregisters when it does not.
   CI/release workflows build and bundle `office.exe` into the portable zip.
4. **Quality pass**: English-only comments and identifiers; removed dead code
   (demo store, unused state setters, unused CSS components/tokens, the
   non-functional attach button); deleted the stale `GetSettings` demo
   defaults; fixed `wails.json` frontend commands that broke `wails build`;
   created `docs/contracts/wails-bindings.md` (referenced by code but missing);
   made the Small-Task model selector real (`small_model` end to end).

## Decisions

- User-visible product copy (UI labels, Zalo replies, file-dialog titles) may
  stay Vietnamese; code comments, identifiers, log/error strings are English.
- Zalo credentials live in gotack's local config (write-only over the bind);
  the bridge ignores chats that are not allow-listed.
- Office integration ships as an MCP server (typed tools, absolute binary path,
  no PATH dependency) instead of a shell CLI surface.
- `appconfig.Defaults()` no longer pins demo provider/model values; empty
  settings mean Crush's catalog defaults apply.

## Validation

- `go test ./...` and `go vet ./...`: pass (zalo client/bridge, office
  round-trips, MCP protocol, appconfig all covered).
- `pnpm --dir frontend check`: 0 errors, 0 warnings; `pnpm --dir frontend
  build`: pass.
- `wails build -platform windows/amd64 -clean`: pass (after fixing
  `wails.json`); new binds confirmed in generated `frontend/wailsjs`.
- Office round-trip: generated docx/pptx/xlsx through the real `office.exe`
  MCP binary and verified with python-docx, python-pptx and openpyxl
  (paragraphs, bold runs, tables, slides, typed cells, appended rows).
- Zalo behavior covered by httptest-based unit tests (allow-list, dedupe,
  busy reply, truncation, failure replies).
