<!-- codebase-memory-mcp:start -->
# Codebase Knowledge Graph (codebase-memory-mcp)

This project uses codebase-memory-mcp to maintain a knowledge graph of the codebase.
ALWAYS prefer MCP graph tools over grep/glob/file-search for code discovery.

## Priority Order
1. `search_graph` — find functions, classes, routes, variables by pattern
2. `trace_path` — trace who calls a function or what it calls
3. `get_code_snippet` — read specific function/class source code
4. `query_graph` — run Cypher queries for complex patterns
5. `get_architecture` — high-level project summary

## When to fall back to grep/glob
- Searching for string literals, error messages, config values
- Searching non-code files (Dockerfiles, shell scripts, configs)
- When MCP tools return insufficient results

## Examples
- Find a handler: `search_graph(name_pattern=".*Handler.*")`
- Who calls it: `trace_path(function_name="Handler", direction="inbound")`
- Read source: `get_code_snippet(qualified_name="main.App")`
<!-- codebase-memory-mcp:end -->

<!-- gotack-layout:start -->
# Repository layout (authoritative)

| Path | Role |
| --- | --- |
| main.go | Wails entry point: window options, embeds frontend/dist |
| app.go | App object bound to the UI, lifecycle and service wiring |
| bind_*.go | Wails-bound API groups: bridge, engine, workspace, session, permission, changes, terminal |
| events.go | Host to UI event emission, single place |
| internal/ | Desktop-side implementation, one package per role |
| frontend/ | Svelte 5 UI. Folder name fixed by Wails v2 |
| third_party/crush/ | Vendored Crush engine, own git history, ignored by this repo |
| docs/ | Architecture, contracts, decisions, guides, plans and product docs |
| build/ | Wails packaging assets per platform |
| scripts/ | Developer entry points |

## Hard rules

1. Bound methods stay in package main. The UI calls `window.go.main.App.*`.
2. Never import `third_party/crush/internal/...`; use `internal/crushapi` over REST + SSE.
3. UI-to-host calls go only through `frontend/src/platform/desktop.ts`.
4. No agent logic in the desktop layer. Crush owns agent execution, sessions, permissions, LSP, MCP and persistence.
5. No polling when SSE events exist. Terminal and editor are lazy-loaded.
6. Keep implementation files under 1000 lines; split by responsibility before a file becomes a mixed-responsibility module.
7. Update the relevant `docs/contracts/` document in the same change when an external or UI/host contract changes.
<!-- gotack-layout:end -->
