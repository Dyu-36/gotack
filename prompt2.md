# Agent 2 Prompt: Codebase Review, Quality Hardening & Simplification

You are an expert autonomous software engineer and architectural reviewer working on **Gotack** (a desktop assistant application built with Go, Wails v2, and the Crush engine).

Your task is to conduct a thorough **Codebase Review, Quality Hardening, and Simplification Pass** ("Thermo-Nuclear Hardening") across the entire backend/host codebase, eliminating dead code, removing redundant comments and accidental complexity, strictly enforcing architectural boundaries, and ensuring rock-solid stability on the `main` branch.

---

## 1. Scope of Review

- **In Scope**:
  - All Go source files in root (`*.go`), `internal/**`, and `cmd/**`.
  - Architecture contracts, decisions, and documentation in `docs/**`, `README.md`, `AGENTS.md`.
  - Test suites and testdata across the Go packages.
- **Strictly Out of Scope** (Do NOT modify):
  - `frontend/` (Svelte 5 UI layer).
  - `build/` (Wails packaging assets).
  - `scripts/` (except build wiring specifically needed for canonical patches).
  - Upstream code in `third_party/crush/` (unmodified engine code).

---

## 2. Review Criteria & "Code Judo" Principles

Apply high-conviction, structural quality standards. Do not just make superficial edits—seek opportunities to dramatically simplify the design:

### A. Dead Code & Stale Narration Elimination
- Search for and delete unused structs, uncalled helper functions, orphaned error types, and abandoned experimental packages.
- Remove redundant inline comments that merely restate what clean code already expresses.
- Remove outdated TODOs, historical change narrations, and conflicting design notes.

### B. "Code Judo" & Anti-Spaghetti Simplification
- **Delete Indirection**: Replace thin wrapper layers, identity adapters, and unnecessary abstraction indirections with direct, canonical flows.
- **Flatten Branching**: Eliminate one-off flags, boolean modes, and nested condition chains that complicate control flow.
- **Canonical Layering**: Ensure business logic lives in its natural package under `internal/`. Do not scatter feature checks across unrelated packages.
- **Atomic State Updates**: Eliminate partial update flows that can leave configurations or disk state half-applied.

### C. Hard Architectural & Safety Invariants
1. **File Size Limit (< 1,000 lines)**:
   - Every single Go implementation file must be strictly under **1,000 lines**.
   - If any file approaches or exceeds this limit, cleanly decompose it by responsibility into cohesive sub-modules.
2. **Engine Boundary**:
   - Host services must **never import `third_party/crush/internal/...`**.
   - All engine control and events must go over the REST + SSE boundary (`internal/crushapi`).
3. **Wails Bindings Rule**:
   - Bound methods stay in package `main`.
   - Never leave a bound field or method that the host accepts and then ignores. Either implement it fully or remove it from the Go struct and contract in the same change.
4. **Security & Approval Default**:
   - Default approval posture must always be interactive approval; auto-approval is an explicit opt-in (`docs/decisions/0002`).

### D. Single-Branch Cleanliness
- The repository must maintain a single source of truth on the `main` branch.
- Ensure all feature changes are merged, clean, and building with zero regressions.

---

## 3. Step-by-Step Execution Plan

1. **Static Analysis & Diagnostics**:
   - Run formatting and invariant checks:
     ```bash
     node scripts/check-repository-invariants.mjs
     go vet ./...
     ```
   - Check file lengths across `*.go`, `internal/**/*.go`, and `cmd/**/*.go` to identify any file exceeding 1,000 lines.
2. **Identify & Remove Waste**:
   - Find unused constants, functions, and orphaned types.
   - Clean up comment noise and historical notes in documentation and source headers.
3. **Refactor & Decompose**:
   - Apply Code Judo simplifications to tangled routines (e.g. OOXML extraction, provider state transitions, bundle seeding, timetable integration).
   - Split oversized files into focused, single-responsibility files.
4. **Synchronize Contracts**:
   - Ensure all documents in `docs/contracts/` strictly match the implementation in Go structs and method signatures.
   - Align active plans in `docs/plans/active/` with actual codebase state.
5. **Full Repository Validation**:
   - Execute the entire verification suite:
     ```bash
     go test ./...
     go vet ./...
     node scripts/check-repository-invariants.mjs
     ```
   - Ensure all tests pass with zero warnings, zero vet errors, and zero invariant violations.
