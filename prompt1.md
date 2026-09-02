# Agent 1 Prompt: Complete Hermes Self-Learning Loop Parity

You are an expert autonomous software engineer working on **Gotack** (a desktop assistant application built with Go, Wails v2, and the Crush engine).

Your task is to inspect, polish, and complete the **Self-Learning Loop** in Gotack to achieve full behavioral and architectural parity with the official **NousResearch Hermes Agent** (`ab9866bc64df48281a2d929dfb1dfd1001973d24`), without inventing any extraneous mechanisms.

---

## 1. Background & Architecture Context

Gotack provides a desktop interface around the headless Crush agent engine.
- **Engine boundary**: Desktop host services communicate with Crush **strictly over its REST API and Server-Sent Events (SSE)**. Never import `third_party/crush/internal/...`.
- **Package layout**:
  - Host entry points & Wails-bound APIs: Package `main` (`main.go`, `app.go`, `bind_*.go`, `*_seed.go`, `*_host.go`).
  - Core desktop business logic: `internal/` (one package per responsibility, e.g., `internal/memory`, `internal/skillmanage`, `internal/reflection`, `internal/guard`, `internal/recall`).
  - Standalone MCP helper servers: `cmd/` (`cmd/memory`, `cmd/skills`, `cmd/guard`, `cmd/recall`), built as standalone executables and wired via standard MCP over stdio.

---

## 2. Core Self-Learning Primitives to Audit & Complete

The Hermes self-learning architecture consists of 5 bounded components:

### A. Persistent Memory (`internal/memory` & `cmd/memory`)
- **Files**: Curates `MEMORY.md` (project memory, cap: 2,200 runes) and `USER.md` (user profile, cap: 1,375 runes) located in the seeded context directory.
- **Operations**:
  - Bounded atomic mutations via MCP tool calls.
  - Hard cap enforcement: Refuse writes when the cap is exceeded; do **not** automatically evict existing memory.
  - Plain-text representation (no JSON/metadata wrapping stored inside the markdown files).
  - Cross-process file locking (Windows `LockFileEx` / Unix `flock`) and atomic replace via temporary files.
  - **Token optimization**: Successful write responses must be compact acknowledgments and must **not** echo the full stored text back to the LLM.

### B. Procedural Skills (`internal/skillmanage` & `cmd/skills`)
- **Discovery**: Crush canonical `view` and workspace skill catalog remain authoritative for skill discovery.
- **Handshake & Mutations**:
  - `skill_view`: Used as a same-process fresh-read verification handshake before mutations.
  - `skill_manage`: 5 standard operations (`create`, `patch`, `delete`, `revert`, `manage`).
  - Safety constraints: Max-20 rollback batch history; managed-root path confinement; background-only ownership manifest (`.ownership.json`) to protect user-created skills from unauthorized background modification.

### C. Background Review & Reflection (`internal/reflection` & `reflection_host.go`)
- **Cadence triggers**:
  - Memory review: Triggered every **10 accepted user turns**.
  - Skill review: Triggered every **10 model iterations** (reset when `skill_manage` is invoked).
- **Execution**:
  - Runs in a detached background session via Crush REST API using the configured model.
  - Context digest: Compact transcript slice (latest 24 items, bounded rune size: max 300 runes for user/assistant messages, 200 runes for tool outputs).
  - Safety constraints: 16-iteration hard ceiling; cancellation if a new foreground user turn begins; suppression during scheduled jobs; automated cleanup of the detached session upon completion.

### D. Guard Hook (`internal/guard` & `cmd/guard`)
- **PreToolUse hook**: Evaluates tool calls before execution.
- Enforces tool approval tiers, a destructive-command floor, and a strict review roster tool allowlist for background reflection sessions.

### E. Cross-Session Recall (`internal/recall` & `cmd/recall`)
- **Indexing**: Strictly read-only indexing of engine session history from `crush.db` into a derived SQLite FTS5 `recall.db`.
- **Search**: `session_search` / query tools with a 24-KiB hydrated-content token budget ceiling.

---

## 3. Strict Rules & Anti-Goals

1. **NO Invented Mechanisms**: Do NOT add autonomous background daemons, 3-strike pattern distillation, Journey/timeline UIs, literal prompt nudge injectors, or staging approval queues. Follow the upstream Hermes architecture strictly.
2. **File Size Invariant**: Every Go source file must stay strictly under **1,000 lines**.
3. **No Engine Internal Imports**: Always interact with Crush via `internal/crushapi` or stdio MCP.
4. **Token Budgeting**: Keep all tool schemas, responses, and digests as token-compact as possible.
5. **No Broken Bindings**: Every field in Wails bindings and contracts must be fully wired and functional.

---

## 4. Step-by-Step Instructions

1. **Audit Existing Implementation**:
   - Check `internal/memory`, `internal/skillmanage`, `internal/reflection`, `internal/guard`, `internal/recall`, and `cmd/*`.
   - Review the active execution plan at `docs/plans/active/hermes-learning-loop.md`.
2. **Fix Gaps & Complete Wiring**:
   - Verify that all 4 helper binaries (`memory`, `skills`, `guard`, `recall`) are properly registered in `settings_crush.go` and workspace configuration.
   - Ensure the skill root path merging handles both bundled and learned skills cleanly.
   - Confirm reflection background triggers correctly track turn counts and fire detached reviews without blocking foreground UI.
3. **Run Full Test Suite & Validation**:
   ```bash
   go test ./internal/memory ./internal/skillmanage ./internal/recall ./internal/reflection ./internal/guard
   go test ./cmd/memory ./cmd/skills ./cmd/recall ./cmd/guard
   go test ./...
   go vet ./...
   node scripts/check-repository-invariants.mjs
   ```
4. **Verify Contracts & Documentation**:
   - Check that `docs/contracts/` (`gotack-memory-mcp.md`, `gotack-skills-mcp.md`, `gotack-recall-mcp.md`, `gotack-reflection.md`) precisely match the code.
   - Once all criteria are met, update `docs/plans/active/hermes-learning-loop.md` to ready/completed.
