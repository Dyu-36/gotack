# Contract: Gotack ↔ Crush REST + SSE boundary

This document is the contract for everything the desktop host writes into the
Crush engine's configuration, and every REST endpoint and SSE event it
consumes. The wire shapes are pinned against the Crush commit recorded in
`.crush-pin`; `scripts/update-crush.ps1` refreshes the vendored copy and
`internal/crushapi` speaks to the engine only over this boundary (AGENTS.md
hard rule 2: no imports of `third_party/crush/internal/...`).

Transport: the engine listens on a per-user named pipe (Windows) or unix
socket (`internal/appconfig.PipeEndpoint`). `internal/crushapi` dials it
directly; requests target a fixed dummy host because the transport ignores
it. Scopes follow Crush's own values: `0` = global config, `1` = workspace
config (`crushapi.ConfigScopeGlobal` / `ConfigScopeWorkspace`).

## Config keys written by the host

Single keys use `POST /v1/workspaces/{id}/config/set`. Related keys use
`POST /v1/workspaces/{id}/config/set-batch`, which applies the complete
JSON-path map in one config-store mutation. Either form can be undone with
`POST /v1/workspaces/{id}/config/remove`; deleting an absent key is not an
error server-side.

| Key | Scope | Written by | Value | Undo |
| --- | --- | --- | --- | --- |
| `mcp_servers.gotack-office` | workspace | `office_seed.go` `registerOfficeTools` | `{command, type: "stdio", timeout: 30}` | `RemoveConfigField` on the same key; the host already does this when the officecli binary is absent |
| `mcp_servers.gotack-memory` | workspace | `memory_seed.go` `registerMemoryTools` on every workspace activation | `{command, type: "stdio", timeout: 30}` | `RemoveConfigField` on the same key; the host already does this when the memory binary is absent. Tool shape in `gotack-memory-mcp.md` |
| `mcp_servers.gotack-skills` | workspace | `skills_seed.go` `registerSkillsTools` on every workspace activation | `{command, args: ["--root", "<appconfig.Dir()>/skills"], type: "stdio", timeout: 30}` | `RemoveConfigField` on the same key; the host already does this when the skills binary is absent. Tool shape in `gotack-skills-mcp.md` |
| `mcp_servers.gotack-recall` | workspace | `recall_seed.go` `registerRecallTools` on every workspace activation | `{command, args: ["--data-dir", "<workspace data dir>", "--index-dir", "<appconfig.Dir()>/recall/<workspace-id>"], type: "stdio", timeout: 30}` | `RemoveConfigField` on the same key; the host already does this when the recall binary, active workspace, or data directory is absent. Tool shape in `gotack-recall-mcp.md` |
| `env` | workspace | `office_seed.go` `registerOfficeTools` | Existing environment entries plus Gotack's `PATH`, so agent shells resolve the seeded `officecli` and Python runtime | Restore the previous map. Gotack reads and merges before an atomic batch write; unrelated user keys are preserved |
| `options.skills_paths` | workspace | `office_seed.go` `registerOfficeTools` | Existing entries, then the seeded, per-user, and current project skill directories, deduplicated without reordering | Restore the previous list. Removing the whole key would also remove user entries and is not a safe surgical rollback |
| `options.global_context_paths` | workspace | `context_seed.go` `registerContextPaths` on every workspace activation | `[<appconfig.Dir()>\context-prompt\snapshot-<generation>]`, an immutable projection built by `internal/contextseed` from the writable seeded directory | `RemoveConfigField`; Crush falls back to its schema defaults (`~/.config/crush/CRUSH.md`, `~/.config/AGENTS.md`). The host also removes the key itself when the seeded directory is absent |
| `providers.<id>.{name,type,base_url,discover_models,models,api_key}` | global | `provider_overlays.go` / `provider_codex.go` | One complete Gotack-owned local provider definition, written with `SetConfigFields`; OAuth-only Codex deliberately omits model and key fields | Remove only the Gotack-owned paths, or set `disable: true`. Existing hand-written providers are never claimed unless their stable identity matches |
| `providers.<id>.disable` | global | `settings_crush.go` / `bind_config.go` | `false` when enabled; `true` is the first engine mutation during deletion | `RemoveConfigField` only when the provider definition itself is being removed |
| `providers.<id>.base_url` | global | `settings_crush.go` `applyCrushSettings` | User-supplied custom endpoint URL, after OAuth/provider validation | `RemoveConfigField` |

Key-name verification: `options.global_context_paths` is the json tag of
`Options.GlobalContextPaths` in the vendored `internal/config/config.go`.
`options.context_paths` is deliberately **not** written: overwriting it would
suppress the engine's discovery of per-project `AGENTS.md` / `CRUSH.md` /
`.github/copilot-instructions.md`.

`env` and `options.skills_paths` are read, merged, and sent together through
`config/set-batch`; a read failure skips the write rather than guessing over
user config. The batch is atomic inside Crush. It is not a distributed
transaction with Gotack's local settings file or the credential store, so
provider flows remain explicitly dependency-ordered and retryable.

### Keys written through dedicated config endpoints

These are written through typed endpoints; their rollback behavior is shown
explicitly below:

| Effect | Endpoint | Written by | Undo |
| --- | --- | --- | --- |
| Preferred large + small model pair | `POST /v1/workspaces/{id}/config/models` (`SetPreferredModelPair`) | settings, OAuth migration, and provider selection | One request sets both `large` and `small`, promotes recent history, and updates per-instance pins in one config mutation. Sending both values as `null` (`RemovePreferredModelPair`) clears both slots and pins while retaining recent history |
| Provider API key | `POST /v1/workspaces/{id}/config/provider-key` (`SetProviderAPIKey`) | `settings_crush.go` | `RemoveConfigField` on `providers.<id>.api_key`; provider deletion disables first, then performs this idempotent cleanup |
| ChatGPT OAuth token | `POST /v1/workspaces/{id}/config/provider-key` (`SetProviderOAuthToken`, kind `oauth`) | `bind_oauth.go` | Crush preserves account routing and refresh metadata, stores the live Codex catalog, routes Responses requests to `https://chatgpt.com/backend-api/codex`, and refreshes through the dedicated endpoint. `DeleteProvider("codex")` removes the credential after disabling the provider and clearing any selected pair |

### Runtime state, not config keys

- `POST /v1/workspaces/{id}/permissions/skip` (`SetPermissionsSkip`):
  receives the saved `auto_approve` value. Prompts stay enabled by default;
  only explicit `auto_approve: true` sends `skip: true`. It is runtime state,
  not a Crush config key.
- `POST /v1/workspaces/{id}/agent/init` (`InitAgent` / `EnsureAgent`):
  initializes the workspace agent once provider/model config is applied;
  `EnsureAgent` skips when the agent is already ready.

## REST endpoints consumed

All in `internal/crushapi`; one method per route, no business logic.

| Endpoint | Method(s) | Consumers |
| --- | --- | --- |
| `/v1/health` | GET | `engine` supervisor probe before adopting a running engine |
| `/v1/version` | GET | engine info shown in the UI |
| `/v1/workspaces` | GET, POST | workspace listing/opening (`internal/workspace`); POST carries `path`, optional `data_dir`, `yolo`, `client_id` |
| `/v1/workspaces/{id}/config` | GET | provider readiness and credential presence (`config_read.go`) |
| `/v1/workspaces/{id}/config/set` | POST | single-field config mutations |
| `/v1/workspaces/{id}/config/set-batch` | POST | one non-empty map of related config fields; all fields persist atomically |
| `/v1/workspaces/{id}/config/remove` | POST | undo path for every `config/set` key |
| `/v1/workspaces/{id}/config/models` | POST | typed atomic mutation of `large` and/or `small`; object sets, `null` deletes |
| `/v1/workspaces/{id}/config/provider-key` | POST | credential storage |
| `/v1/workspaces/{id}/config/refresh-oauth` | POST | provider-specific OAuth refresh; used by ChatGPT status checks and the agent's auth retry path |
| `/v1/workspaces/{id}/providers` | GET | configured-provider listing |
| `/v1/workspaces/{id}/sessions` | GET, POST | session list/create (`internal/session`) |
| `/v1/workspaces/{id}/sessions/{sid}` | GET, PUT, DELETE | session rename/delete (`session_mutation.go`) |
| `/v1/workspaces/{id}/sessions/{sid}/messages` | GET | message history for the transcript |
| `/v1/workspaces/{id}/sessions/{sid}/history` | GET | changed-file history |
| `/v1/workspaces/{id}/agent` | GET, POST | agent readiness check; prompt submission (202 queued) |
| `/v1/workspaces/{id}/agent/init` | POST | agent initialization |
| `/v1/workspaces/{id}/agent/sessions/{sid}/cancel` | POST | abort an in-flight turn |
| `/v1/workspaces/{id}/current-session` | POST | marks the active session for this client id |
| `/v1/workspaces/{id}/permissions/grant` | POST | answers a permission request from the UI |
| `/v1/workspaces/{id}/permissions/skip` | POST | see runtime state above |
| `/v1/workspaces/{id}/questions/answer` | POST | answers an agent question batch |
| `/v1/workspaces/{id}/events` | GET (SSE) | the event stream below |

The two atomic mutation routes reject empty payloads, invalid scopes, and
invalid model entries with HTTP 400. `/config/models` accepts only `large`
and `small`; a set value must contain both provider and model. Missing
workspaces return 404, persistence failures return 500, and success is an
empty HTTP 200 response.

## SSE events consumed

One subscription per attached workspace:
`GET /v1/workspaces/{id}/events?client_id=<uuid>`, allow-listed to exactly
these envelope kinds (`bind_engine.go` attach path, decoded in
`internal/uievents`):

| Kind | Lifecycle | Use |
| --- | --- | --- |
| `message` | `updated` | token deltas, tool-call activity for the transcript |
| `run_complete` | flat payload | turn finished: final text, cancelled flag; drives session-done routing (UI, Zalo, and scheduled-run outcome bookkeeping via `internal/schedule`) |
| `permission_request` | flat payload | pairs with the permission relay in `internal/permission` |
| `question_batch_request` | flat payload | agent question batches surfaced in the UI |
| `file` | flat payload | file-change notifications feeding the changes panel |

Envelope shape: `data: {"type": "<kind>", "payload": {"type":
"created|updated|deleted", "payload": {...}}}`. Some kinds (e.g.
`run_complete`, `permission_request`) carry a flat payload without the inner
wrapper; the decoder handles both. Malformed envelopes are dropped, never
fatal. Rule: SSE is the only path state takes to the UI and to host reactions
— no polling of REST routes where an event exists (AGENTS.md hard rule 5).

## Seeding and release-platform limitation

The Office runtime (`internal/officecli`) and the persona context
(`internal/contextseed`) are seeded from directories shipped next to the
executable into `appconfig.Dir()` (`%AppData%\gotack` on Windows). Honest
limitations:

- Release packaging currently builds a Windows `amd64` ZIP. Both seeders are
  platform-neutral: context uses the tracked `TACK.md` marker and Office uses
  the platform executable name. A future non-Windows release must ship the
  matching runtime payload.
- `options.global_context_paths` is rewritten on every workspace activation
  with one immutable snapshot path. The host does not merge additional global
  context directories; files intended for the persona projection must be
  placed under the seeded context directory.
- Seeded-file updates are size-keyed: a bundled file whose content changes
  without a size change is not re-propagated. User-editable context files are
  preserved when untracked or size-modified; managed runtime files are
  replaceable. A malformed seed report fails before copying, and report
  replacement itself is atomic.

## Rollback summary

To detach Gotack-owned workspace integration, remove its four MCP entries,
`options.global_context_paths`, and `env`. For `options.skills_paths`, restore
the prior list or remove only Gotack's seeded/user/project additions; deleting
the whole key can destroy unrelated user configuration.

```text
RemoveConfigField mcp_servers.gotack-office        (workspace scope)
RemoveConfigField mcp_servers.gotack-memory        (workspace scope)
RemoveConfigField mcp_servers.gotack-skills        (workspace scope)
RemoveConfigField mcp_servers.gotack-recall        (workspace scope)
RemoveConfigField env                              (workspace scope)
RemoveConfigField options.global_context_paths     (workspace scope)
```

Provider deletion is safety-first and convergent: persist a cleared local
selection when needed, set `providers.<id>.disable: true`, atomically clear a
selected model pair, then remove `api_key` and `oauth`. Each cleanup is
idempotent, so a failed attempt can be retried without re-enabling a partially
stripped provider.

Seeded directories under `appconfig.Dir()` (`bin`, `skills`, `context`) may
then be deleted. The derived `recall/<workspace-id>` directory is rebuildable
and may be deleted without touching engine history.
