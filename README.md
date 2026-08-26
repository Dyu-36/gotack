# gotack

A lightweight desktop client for [Crush](https://github.com/charmbracelet/crush), designed for fast startup, low memory usage, and smooth operation on low-resource machines.

## Goals

- Keep desktop memory usage low enough to remain practical on 6 GB RAM systems.
- Reuse Crush as the coding-agent engine instead of duplicating agent logic.
- Keep the desktop layer thin and independently replaceable.
- Favor native/system components over bundled heavyweight runtimes.
- Start fast and stay responsive while Crush, LSPs, shells, and build tools are running.

## Proposed stack

- **Go** for the desktop/backend layer.
- **Wails 2** as the desktop shell.
- **Svelte + TypeScript** for the UI.
- **System WebView** instead of bundling Chromium.
- **Crush REST + SSE API** as the engine boundary.
- **CodeMirror 6** for lightweight file viewing/editing when needed.
- **xterm.js**, lazy-loaded, for terminal support.

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
