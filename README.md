# gotack

A lightweight general-purpose desktop AI assistant powered by [Crush](https://github.com/charmbracelet/crush), designed for fast startup, low memory usage, and smooth operation on low-resource machines.

## Goals

- Keep desktop memory usage low enough to remain practical on 6 GB RAM systems.
- Reuse Crush as the local agent engine instead of duplicating agent logic.
- Keep the desktop layer thin and independently replaceable.
- Favor native/system components over bundled heavyweight runtimes.
- Start fast and stay responsive while Crush, LSPs, shells, and build tools are running.

## Desktop-specific upgrades

Beyond bringing Crush into a native desktop workflow, `gotack` ships a small set of capabilities designed specifically for day-to-day desktop use:

- **Tack-style local assistant** — the primary Crush prompt is aligned with Stack's Tack agent for general filesystem, Office, automation, system, research and software tasks. Its read-only sub-agent follows Stack's Sage research role, while Crush still injects Gotack's live skills and local context.
- **Zalo connection** — connect `gotack` to an official [Zalo Bot](https://bot.zaloplatforms.com) token in Settings. Access is granted per chat by pairing: Settings shows a rotating six-digit code, and a chat joins by sending `/pair <code>`. Codes can be reissued (`RegenerateZaloPairingCode`) and individual chats revoked (`UnpairZaloChat`). The bridge long-polls `getUpdates`, forwards messages and image/document attachments from paired chats to the agent (one reusable session per chat), and sends the finished answer and referenced output files back, so the desktop agent stays reachable while the user is away. The token is stored locally and never returned to the UI. Paired chats and their sessions persist in `<configDir>/zalo.json`. The older `zalo.allowed_chats` config key is imported once at startup for backwards compatibility and is never written again.
- **Office integration** — the Stack-compatible `officecli` executable, Office skill set, timetable solver/exporter and a bundled `office` MCP server (built from `cmd/office`) are installed into Crush's runtime whenever a workspace opens. The agent gains typed tools to inspect, read, create and edit Word (.docx), Excel (.xlsx) and PowerPoint (.pptx) files without a separate Office CLI setup. Resource seeding looks for `officecli.exe`, so today the bundle is only discovered on Windows; on other platforms Gotack starts without it and the Office MCP server is still registered.
- **Live model catalog** — the provider and model pickers are populated from the engine's `GET /v1/workspaces/{id}/providers` catalog (including per-model context windows and costs) instead of a bundled static list. Settings deliberately exposes a single model selector, and the host writes that one model ID into both `models.large` and `models.small` in the Crush config; there is no separate small-task model today.

## Stack baseline

Baseline as of **2026-08-31**. Rows marked *installed* are what the repo builds
against today; the rest are target versions for features that have not landed.
This table records intent, so it must be reconciled with `go.mod` and
`frontend/package.json` whenever either changes.

| Layer | Version / choice | Status |
| --- | --- | --- |
| Go | **1.27.0** | installed, pinned in `go.mod` |
| Wails | **v2.15.0** | installed, also pinned in both workflows |
| Node.js | **24** | installed, set in `.github/workflows` |
| pnpm | **11.20.0** | installed, pinned via `packageManager` |
| Svelte | **5.56.10** | installed |
| TypeScript | **~5.9.3** | installed |
| Vite | **8.2.2** | installed |
| Vitest | **4.1.11** | installed |
| `@sveltejs/vite-plugin-svelte` | **7.3.0** | installed |
| Tailwind CSS | **4.3.3** (`tailwindcss` + `@tailwindcss/vite`) | installed |
| Desktop web runtime | **System WebView** | installed |
| Crush integration | **REST + SSE API** | installed |
| Crush pin | **`.crush-pin`** | installed; single tracked owner at the repository root, read by both workflows and `scripts/update-crush.ps1` |
| Markdown rendering | **`marked` ^18.0.11** + **`dompurify` ^3.4.14** | installed |
| Toasts | **`svelte-sonner` 1.2.1** | installed |
| UI font | **`@fontsource-variable/inter` 5.3.0** | installed |
| `@xterm/xterm` | **6.0.0**, lazy-loaded | installed |
| `@xterm/addon-fit` | **0.11.0** | installed |
| CodeMirror umbrella package | **6.0.2** | planned, not installed |
| `@codemirror/view` | **6.43.9** | planned, not installed |

Go direct dependencies (`go.mod`), all installed:

| Module | Version | Used for |
| --- | --- | --- |
| `github.com/wailsapp/wails/v2` | v2.15.0 | desktop shell and bindings |
| `github.com/xuri/excelize/v2` | v2.11.0 | `.xlsx` read/write in `internal/office` |
| `github.com/UserExistsError/conpty` | v0.1.4 | Windows PTY for `internal/terminal` |
| `github.com/creack/pty` | v1.1.24 | POSIX PTY for `internal/terminal` |
| `github.com/Microsoft/go-winio` | v0.6.2 | named-pipe transport to Crush |
| `github.com/google/uuid` | v1.6.0 | request and session identifiers |

Notes:

- Wails v3 is still pre-release, so `gotack` stays on the latest stable Wails v2 release.
- TypeScript is held at 5.9.x deliberately: TS 7 fails `pnpm check` against the current Svelte tooling. Revisit when `svelte-check` supports it.
- CodeMirror 6 is split across independently versioned packages. `codemirror@6.0.2` is the umbrella/basic-setup package, while core packages such as `@codemirror/view` have their own current versions.
- xterm.js beta builds are intentionally excluded from the baseline.
- The terminal panel lazy-loads `@xterm/xterm` only when opened; no editor package ships until the editor feature lands.
- Open deviation: five direct dependencies are still declared as ranges rather than exact pins — `marked ^18.0.11` and `dompurify ^3.4.14` (runtime), `@types/dompurify ^3.2.0` and `svelte-check ^4.7.6` (dev), and `typescript ~5.9.3`. The remaining eleven entries in `frontend/package.json` are pinned exactly. Pin these five or relax the policy; leaving the two in conflict makes the policy unenforceable.

Version policy:

- pin exact toolchain and direct dependency versions for reproducible builds;
- stay on the Wails v2 stable line until Wails v3 reaches a production-stable release and migration is justified;
- allow patch updates after CI/build verification;
- do not upgrade major frontend/runtime dependencies automatically;
- keep the Crush protocol boundary versioned independently from the desktop UI.

## Architecture

```text
┌───────────────────────────────────────┐
│               gotack                  │
│                                       │
│  Svelte / TypeScript                  │
│  ├── workspace                        │
│  ├── sessions                         │
│  ├── chat                             │
│  ├── tool activity                    │
│  ├── permissions                      │
│  ├── diff / files                     │
│  └── terminal                         │
│             │                         │
│         Wails bridge                  │
│             │                         │
│        thin Go client                 │
└─────────────┬─────────────────────────┘
              │ REST + SSE
              │ Unix socket / named pipe
              ▼
┌───────────────────────────────────────┐
│                Crush                  │
│                                       │
│  server → backend                     │
│           ├── agent                   │
│           ├── sessions                │
│           ├── permissions             │
│           ├── LSP                     │
│           ├── MCP                     │
│           ├── shell                   │
│           └── SQLite                  │
└───────────────────────────────────────┘
```

## Design principles

### Thin desktop layer

`gotack` should not reimplement Crush internals. The desktop app owns presentation, local process lifecycle, and client-side state. Crush remains responsible for agent execution, sessions, tools, permissions, MCP, LSP integration, and persistence.

### Process isolation

The preferred runtime model is two processes:

```text
gotack
  └── crush
```

This keeps the UI lifecycle separate from active agent work. A UI restart should be able to reconnect to the running Crush server instead of terminating agent activity.

### Low memory first

Features that can consume significant memory should be loaded only when required. In particular:

- do not bundle Chromium or Electron;
- do not initialize a full editor until a file is opened;
- lazy-load terminal support;
- avoid Monaco in the initial version unless its capabilities are required;
- keep long-running state in Crush rather than duplicating it in the UI;
- avoid background polling where SSE events are available.

### Single-user trust model

Gotack attaches every workspace with Crush permission prompts skipped, and the
default assistant workspace is the drive root (`C:\` on Windows) so that
startup chat always has a real session context. This is a deliberate
single-user desktop trade-off, not an oversight; treat it as a
security-relevant default when changing workspace handling.

## Initial scope

The first usable version focuses on a fast local-assistant workflow rather than becoming a full IDE:

1. Discover or launch the local Crush server.
2. Create and attach to a workspace.
3. List and switch sessions.
4. Send prompts for local files, Office work, system tasks or code and stream agent activity.
5. Render messages, reasoning, and tool calls.
6. Handle permission and question requests.
7. Show changed files and lightweight diffs.
8. Provide an optional, lazy-loaded terminal.
9. Reconnect cleanly after UI restart or temporary transport loss.

Full IDE functionality, complex editor integrations, and heavyweight extensions are intentionally out of scope for the first milestone.

## Project status

Release candidate. The desktop client, Zalo connection, and Office integration
are implemented and covered by the Go unit and smoke suites.

What automated validation proves today (`.github/workflows/ci.yml`):
`go test ./...`, `go vet ./...`, `gofmt`, generated UI-event drift,
repository invariants, `pnpm --dir frontend check`, `pnpm --dir frontend test`,
`pnpm --dir frontend build`, and a Windows `wails build`. `release.yml` re-runs
the source checks, including repository invariants and frontend tests, then
builds the pinned Crush commit and `cmd/office`, bundles `officecli.exe` plus
`resources/skills`, and publishes the portable ZIP.

What is **not** covered by automated validation, and is therefore manually
verified only:

- the Zalo bridge against the live Bot API;
- the bundled timetable Python runtime — no workflow runs
  `scripts/prepare-resources.ps1`, and `release.yml` does not copy
  `resources/bin/` into the artifact;
- any UI end-to-end run.

Report issues against the tagged releases.

## Upstream

Crush is developed by Charmbracelet:

- https://github.com/charmbracelet/crush

## Repository layout

```text
main.go  app.go  bind_*.go  events.go   desktop host (package main, Wails bindings)
office_seed.go  settings_crush.go      package main helpers, not bound methods
internal/                              host implementation, one package per role
  appconfig  attachments  changes  crushapi  engine  enginelink  logging  mcp
  office  officecli  permission  session  terminal  uievents  workspace  zalo
cmd/office/                            bundled Office MCP server (stdio), ships as office.exe
frontend/                              Svelte 5 UI (folder name required by Wails v2)
third_party/crush/                     vendored Crush engine (own git history, ignored here;
                                       only third_party/README.md is tracked)
resources/skills/                      skill tree bundled into release artifacts
docs/                                  contracts, decisions, patterns, plans, product, templates
build/                                 Wails packaging assets per platform
scripts/                               developer entry points (PowerShell, Windows only)
.agents/skills/  .harness-core/        vendored repository-harness protocol and skills;
                                       .harness-core/manifest.json pins upstream file hashes
.github/workflows/                     ci.yml and release.yml
.gitattributes                         normative LF end-of-line policy for the whole tree
```

Folder-by-folder roles and the rules that keep the layers apart: `docs/README.md`.
