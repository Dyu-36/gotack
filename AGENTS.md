<!-- codebase-memory-mcp:start -->
# Codebase Knowledge Graph (codebase-memory-mcp)

This project uses codebase-memory-mcp to maintain a knowledge graph of the codebase.
ALWAYS prefer MCP graph tools over grep/glob/file-search for code discovery.
NOTE: The `project` argument is REQUIRED for all tool calls (for this repository, use `project="gotack"`).

## Priority Order
1. `search_graph` — find functions, classes, routes, variables by pattern (requires `project`)
2. `trace_path` — trace who calls a function or what it calls (requires `project` and `function_name`)
3. `get_code_snippet` — read specific function/class source code (requires `project` and `qualified_name`)
4. `query_graph` — run Cypher queries for complex patterns (requires `project` and `query`)
5. `get_architecture` — high-level project summary (requires `project`)

## When to fall back to grep/glob
- Searching for string literals, error messages, config values
- Searching non-code files (Dockerfiles, shell scripts, configs)
- When MCP tools return insufficient results

## Examples
- List projects: `list_projects()`
- Find a handler: `search_graph(project="gotack", name_pattern=".*Handler.*")`
- Who calls it: `trace_path(project="gotack", function_name="Handler", direction="inbound")`
- Read source: `get_code_snippet(project="gotack", qualified_name="main.App")`
<!-- codebase-memory-mcp:end -->

<!-- gotack-layout:start -->
# Repository layout (authoritative)

| Path | Role |
| --- | --- |
| main.go | Wails entry point: window options (hide-on-close, single instance, `--hidden` start), embeds frontend/dist |
| app.go | App object bound to the UI, lifecycle and service wiring |
| tray_windows.go, tray_other.go | Notification-area icon (`fyne.io/systray`); closing the window only hides it and the tray restores it. Windows implementation plus the non-Windows compile stub |
| bind_*.go | Wails-bound API groups: host, engine, workspace, session, permission, changes, terminal, config, files, OAuth, and Zalo. `bind_host.go` holds the one-method groups (`BackendReady`, `SelectWorkspace`) and the autostart toggle (`GetAutoStart`, `SetAutoStart`) |
| events.go | Host to UI event emission, single place |
| office_seed.go, context_seed.go, guard_seed.go, memory_seed.go, skills_seed.go, recall_seed.go, schedule_host.go, reflection_host.go, settings_crush.go | package main helpers that are not bound methods; they take `*App` only for config and resource seeding |
| internal/ | Desktop-side implementation, one package per role: appconfig, attachments, autostart, bundleseed, changes, contextseed, crushapi, engine, enginelink, guard, logging, mcp, memory, office, officecli, openaioauth, permission, recall, reflection, schedule, session, skillmanage, terminal, uievents, userstrings, workspace, zalo |
| cmd/guard/ | PreToolUse approval hook (destructive-command blocklist, graduated tiers); ships as guard.exe |
| cmd/memory/ | Persistent self-editing memory MCP server over stdio; curates MEMORY.md / USER.md in the seeded context dir; ships as memory.exe |
| cmd/skills/ | Progressive procedural-skill MCP server over stdio; ships as skills.exe |
| cmd/recall/ | Cross-session recall MCP server over stdio; reads crush.db read-only, index in recall.db; ships as recall.exe |
| frontend/ | Svelte 5 UI. Folder name fixed by Wails v2 |
| third_party/crush/ | Vendored Crush engine with its own git history; ignored by this repo |
| third_party/README.md, third_party/patches/ | Tracked pin/patch documentation and Gotack-owned patches applied to the vendored engine |
| resources/skills/ | Skill tree bundled into release artifacts |
| resources/context/ | Tracked persona context files seeded into the user data dir and shipped in release artifacts |
| resources/context/legacy/ | Hash-pinned stock TACK.md bases referenced by `stock-manifest.json`; used by `internal/contextseed` as auto-migration and three-way merge sources |
| resources/bin/ | Ignored runtime payloads; `scripts/prepare-resources.ps1` recreates them before packaging |
| docs/ | Contracts, decisions, patterns, plans and templates |
| build/ | Wails packaging assets per platform |
| scripts/ | Developer/build entry points: Windows PowerShell tooling plus the cross-platform Node repository-invariant check |
| .agents/skills/, .harness-core/ | Vendored repository-harness protocol and skills; .harness-core/manifest.json pins the upstream file hashes |
| .github/workflows/ | ci.yml and release.yml |
| .gitattributes | Normative end-of-line policy: the whole tree is stored and checked out as LF (see docs/patterns/encoding-invariants.md) |

This table and the `Repository layout` block in `README.md` describe the same
tree. Update both in the same change, or the two will drift.

## Hard rules

1. Bound methods stay in package main. The UI calls `window.go.main.App.*`.
2. Never import `third_party/crush/internal/...`; use `internal/crushapi` over REST + SSE.
3. UI-to-host calls go only through `frontend/src/platform/desktop.ts`.
4. Desktop-layer services must not duplicate the engine: the agent turn loop, tool dispatch, message/session persistence, and permission adjudication belong to Crush. Desktop services may schedule runs, curate seeded context files, index engine history read-only, and reflect on completed runs — always over the REST + SSE boundary, never by importing `third_party/crush/internal/...` (`docs/decisions/0001`).
5. No polling when SSE events exist. Terminal and editor are lazy-loaded.
6. Keep implementation files under 1000 lines; split by responsibility before a file becomes a mixed-responsibility module.
7. Update the relevant `docs/contracts/` document in the same change when an external or UI/host contract changes.
8. Never leave a bound field or method that the host accepts and then ignores. Either implement it, or remove it from the Go struct, `desktop.ts` and the contract in the same change. A field the UI believes works is worse than a missing field.
<!-- gotack-layout:end -->

<!-- ui-verification:start -->
# UI & End-to-End Verification (agent-browser)

When testing frontend UI (`frontend/src/`) or end-to-end user workflows, use `agent-browser` (token-efficient Accessibility-Tree CLI/MCP):
1. **Launch dev server or built app**:
   - Dev mode: `pnpm --dir frontend dev` (serves at `http://localhost:5173`)
   - Executable mode: `$env:WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS = "--remote-debugging-port=9222"; Start-Process .\build\bin\gotack.exe`
2. **Inspect UI with compact Accessibility Tree (`@e1`, `@e2`...)**:
   - `npx agent-browser open http://localhost:5173 && npx agent-browser snapshot -i`
3. **Simulate user interactions**:
   - `npx agent-browser fill @eN "text to type" && npx agent-browser click @eM`
4. **Capture visual proof & verify state**:
   - `npx agent-browser screenshot test-result.png`
<!-- ui-verification:end -->

<!-- HARNESS:BEGIN -->
## Harness

Start with the requested outcome and use the repository as the system of record.
Read `docs/WORKFLOW.md` and only relevant product, design, plan, code, and
validation material.

- Answers, explanations, reviews, diagnoses, plans, and status reports are
  read-only. Inspect only what is needed; change nothing.
- For a bounded change, inspect affected behavior and proof, implement, and
  validate. No control-plane operation is required.
- Use one `docs/plans/active/` file when work spans sessions, coordinates
  contributors, has dependencies, or needs recovery. Move it to
  `docs/plans/completed/` only after validation.
- Before editing, identify repository authority for each new externally
  observable policy. If materially different choices remain open, stop before
  edits; configurable defaults are not authority.
- For architecture, reliability, security, or quality invariant work, read
  `docs/patterns/encoding-invariants.md` and enforce only accepted rules.
- Report reusable agent friction. Change guidance, tools, runbooks, or validation
  for that purpose only when explicitly asked to use `$improve-harness`.
- Also pause when product intent remains ambiguous, recovery is difficult,
  validation is weakened, or authority is insufficient.
- Claim completion only with executable or observable evidence. Report outcome,
  changes, validation, and unresolved risks.

Harness has no task database or orchestration lifecycle. Use repository plans
and behavior-level proof; do not create parallel control-plane state.
<!-- HARNESS:END -->
