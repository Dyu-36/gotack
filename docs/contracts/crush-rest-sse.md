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

All keys below are written through
`POST /v1/workspaces/{id}/config/set` (`Client.SetConfigField`) unless noted
otherwise. Every one of them is removable through
`POST /v1/workspaces/{id}/config/remove` (`Client.RemoveConfigField`);
deleting an absent key is not an error server-side. This is the rollback
guarantee: nothing the host attaches requires a migration to undo.

| Key | Scope | Written by | Value | Undo |
| --- | --- | --- | --- | --- |
| `mcp_servers.gotack-office` | workspace | `office_seed.go` `registerOfficeTools` | `{command, type: "stdio", timeout: 30}` | `RemoveConfigField` on the same key; the host already does this when the officecli binary is absent |
| `env` | workspace | `office_seed.go` `registerOfficeTools` | `{PATH: …}` so agent shells resolve the seeded `officecli` and Python runtime | `RemoveConfigField` on `env`. Caveat: the host owns the whole map and overwrites it per workspace; user-defined env vars in the same workspace config are replaced, not merged |
| `options.skills_paths` | workspace | `office_seed.go` `registerOfficeTools` | `[<appconfig.Dir()>\skills]` | `RemoveConfigField`; Crush falls back to its built-in skill discovery |
| `options.global_context_paths` | workspace | `context_seed.go` `registerContextPaths` on every workspace activation | `[<appconfig.Dir()>\context]`, the directory seeded by `internal/contextseed` | `RemoveConfigField`; Crush falls back to its schema defaults (`~/.config/crush/CRUSH.md`, `~/.config/AGENTS.md`). The host also removes the key itself when the seeded directory is absent |
| `providers.<id>.disable` | global | `settings_crush.go` `applyCrushSettings` | `false`, enabling the selected provider | `RemoveConfigField` |
| `providers.<id>.base_url` | global | `settings_crush.go` `applyCrushSettings` | custom endpoint URL | `RemoveConfigField` |

Key-name verification: `options.global_context_paths` is the json tag of
`Options.GlobalContextPaths` in the vendored `internal/config/config.go`.
`options.context_paths` is deliberately **not** written: overwriting it would
suppress the engine's discovery of per-project `AGENTS.md` / `CRUSH.md` /
`.github/copilot-instructions.md`.

Known sharp edges:

- `options.skills_paths` is replaced, not merged. Gotack currently registers
  one skills directory, so nothing is lost today; a second directory must
  read-then-merge or build the full intended list in one place.
- `env` is replaced wholesale per workspace (see table).

### Keys written through dedicated config endpoints

These do not go through `config/set`, so `RemoveConfigField` does not undo
them:

| Effect | Endpoint | Written by | Undo |
| --- | --- | --- | --- |
| Preferred large model (`models.large`) | `POST /v1/workspaces/{id}/config/model` (`SetPreferredModel`) | `settings_crush.go` | Select another model through the same endpoint; there is no unset |
| Preferred small model (`models.small`) | same endpoint | `settings_crush.go` (pinned to the same selection as large) | same |
| Provider API key | `POST /v1/workspaces/{id}/config/provider-key` (`SetProviderAPIKey`) | `settings_crush.go` | Credentials are owned by Crush; the host never deletes them |

### Runtime state, not config keys

- `POST /v1/workspaces/{id}/permissions/skip` (`SetPermissionsSkip`):
  permission prompts are forced off on every workspace Gotack attaches, both
  activation paths. It is runtime state, not a persisted config key.
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
| `/v1/workspaces/{id}/config/set` | POST | every key in the table above |
| `/v1/workspaces/{id}/config/remove` | POST | undo path for every `config/set` key |
| `/v1/workspaces/{id}/config/model` | POST | model selection |
| `/v1/workspaces/{id}/config/provider-key` | POST | credential storage |
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

## Seeding and its Windows-only limitation

The Office runtime (`internal/officecli`) and the persona context
(`internal/contextseed`) are seeded from directories shipped next to the
executable into `appconfig.Dir()` (`%AppData%\gotack` on Windows). Honest
limitations:

- Release packaging is Windows-only (`release.yml` builds a
  `windows/amd64` ZIP), so in practice seeding only ever runs from a Windows
  payload. The context seeder itself is platform-neutral and deliberately
  does not repeat risk R10: it locates its bundled directory by the tracked
  `TACK.md` marker rather than by a hardcoded executable name, so the branch
  is reachable on other platforms if a payload ever ships there. The office
  seeder's source discovery still hardcodes `officecli.exe` and remains
  unreachable off Windows.
- `options.global_context_paths` is rewritten on every workspace activation,
  so a user who wants additional global context directories must currently
  add them to the seeded directory; the host does not merge.
- Seeded-file updates are size-keyed, like the office seeder: a bundled file
  whose content changes without a size change is not re-propagated.

## Rollback summary

To detach everything the host attached to a workspace:

```text
RemoveConfigField mcp_servers.gotack-office        (workspace scope)
RemoveConfigField env                              (workspace scope)
RemoveConfigField options.skills_paths             (workspace scope)
RemoveConfigField options.global_context_paths     (workspace scope)
RemoveConfigField providers.<id>.disable           (global scope)
RemoveConfigField providers.<id>.base_url          (global scope)
```

plus deleting the seeded directories under `appconfig.Dir()` (`bin`,
`skills`, `context`). Model and credential writes are the only ones without a
host-side removal path, because Crush owns them.
