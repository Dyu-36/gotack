# gotack

A lightweight desktop client for [Crush](https://github.com/charmbracelet/crush), designed for fast startup, low memory usage, and smooth operation on low-resource machines.

## Goals

- Keep desktop memory usage low enough to remain practical on 6 GB RAM systems.
- Reuse Crush as the coding-agent engine instead of duplicating agent logic.
- Keep the desktop layer thin and independently replaceable.
- Favor native/system components over bundled heavyweight runtimes.
- Start fast and stay responsive while Crush, LSPs, shells, and build tools are running.

## Desktop-specific upgrades

Beyond bringing Crush into a native desktop workflow, `gotack` extends the agent with a small set of capabilities designed specifically for day-to-day desktop use:

- **Timetable Skills** — dedicated timetable-generation skills for creating schedules from user constraints, with support for custom rules, preferences, formats, and institution-specific requirements.
- **Office CLI integration** — built-in workflows for working with Word, Excel, and PowerPoint documents through CLI-based tools, allowing the agent to inspect, generate, and modify office files as part of a normal task.
- **Zalo integration** — connect `gotack` to Zalo so the local agent can receive requests and return results remotely, making the desktop agent accessible even when the user is away from the computer.

## Stack baseline

Baseline as of **2026-08-26**. Rows marked *installed* are what the repo builds
against today; the rest are target versions for features that have not landed.
This table records intent, so it must be reconciled with `go.mod` and
`frontend/package.json` whenever either changes.

| Layer | Version / choice | Status |
| --- | --- | --- |
| Go | **1.27.0** | installed, pinned in `go.mod` |
| Wails | **v2.15.0** | installed |
| Svelte | **5.56.10** | installed |
| TypeScript | **~5.9.3** | installed |
| Desktop web runtime | **System WebView** | installed |
| Crush integration | **REST + SSE API** | installed |
| CodeMirror umbrella package | **6.0.2** | planned, not installed |
| `@codemirror/view` | **6.43.9** | planned, not installed |
| `@xterm/xterm` | **6.0.0**, lazy-loaded | planned, not installed |

Notes:

- Wails v3 is still pre-release, so `gotack` stays on the latest stable Wails v2 release.
- TypeScript is held at 5.9.x deliberately: TS 7 fails `pnpm check` against the current Svelte tooling (commit `1a67994`). Revisit when `svelte-check` supports it.
- CodeMirror 6 is split across independently versioned packages. `codemirror@6.0.2` is the umbrella/basic-setup package, while core packages such as `@codemirror/view` have their own current versions.
- xterm.js beta builds are intentionally excluded from the baseline.
- The editor and terminal rows are targets for when those features land. Neither package is in `frontend/package.json` yet because both sit outside Milestone 1 (`docs/roadmap.md`).
- Open deviation: `frontend/package.json` still uses caret ranges for most dev dependencies, which contradicts the "pin exact versions" policy below. Pin them or relax the policy; leaving the two in conflict makes the policy unenforceable.

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

## Initial scope

The first usable version should focus on the coding-agent workflow rather than becoming a full IDE:

1. Discover or launch the local Crush server.
2. Create and attach to a workspace.
3. List and switch sessions.
4. Send prompts and stream agent activity.
5. Render messages, reasoning, and tool calls.
6. Handle permission and question requests.
7. Show changed files and lightweight diffs.
8. Provide an optional, lazy-loaded terminal.
9. Reconnect cleanly after UI restart or temporary transport loss.

Full IDE functionality, complex editor integrations, and heavyweight extensions are intentionally out of scope for the first milestone.

## Project status

Early development. Architecture and APIs may change while the initial desktop client is being built.

## Upstream

Crush is developed by Charmbracelet:

- https://github.com/charmbracelet/crush

## Repository layout

```text
main.go  app.go  bind_*.go  events.go   desktop host (package main, Wails bindings)
internal/                              host implementation, one package per role
  appconfig  logging  engine  crushapi
  workspace  session  permission  changes  terminal  uievents
frontend/                              Svelte 5 UI (folder name required by Wails v2)
third_party/crush/                     vendored Crush engine (own git history)
docs/                                  architecture, contracts, decisions, guides
build/                                 Wails packaging assets
scripts/                               developer entry points
```

Folder-by-folder roles and the rules that keep the layers apart: `docs/README.md`.
