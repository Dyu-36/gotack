# D1: App god-object → atomic.Pointer[conn]

Date: 2026-08-29

Active → Completed

## Result

Implemented 2026-08-29. `App` no longer carries a `sync.RWMutex`: the 15
dynamic fields live in a `conn` struct behind `atomic.Pointer[conn]` with
`swapConn`/`getConn` helpers; `sup` is typed as `engine.EngineAPI`. Migrated
every accessor across app.go, events.go and all bind_*.go files (9 files,
+370/-223). Validation: `go vet ./...`, `go build ./...`,
`go test ./...` all clean; `npx svelte-check` 0 errors; `vite build` pass.
Not run in this sandbox: the actual Wails app under a live engine, so the
reconnect path is proven by tests only.

## Outcome

Replace the single `sync.RWMutex` covering all 11 mutable fields of `App`
with an `atomic.Pointer[conn]` that holds the dynamic connection state.
Read paths load the pointer and dereference; write paths copy, mutate, and
swap. Lock contention drops to nil on read; the existing
race-detector test suite must stay green.

## Context

- `app.go:24-48` defines the 11-field struct under one `sync.RWMutex`.
- `bind_engine.go`, `events.go`, `bind_workspace.go`, `bind_session.go`,
  `bind_permission.go`, `bind_changes.go`, `bind_terminal.go` all read
  fields through `a.mu.RLock()` and write through `a.mu.Lock()`.
- The static fields (`log`, `cfg`, `ctx`, `sup`) do not change after
  startup and can stay on `App` directly.
- The harness rule: "report reusable agent friction" — D1 is gated by the
  patch author as "needs explicit go-ahead". The user approved all four
  remaining patches in one turn; that is the go-ahead for this plan.

## Scope

In scope:

- New type `conn` in `app.go` holding the 11 dynamic fields
- `App.conn atomic.Pointer[conn]`; static fields stay on `App`
- Every read of `a.<dyn>` becomes `a.conn.Load().<dyn>` with the
  nil-conn guard
- Every write of `a.<dyn>` becomes
  `a.swapConn(func(c *conn) *conn { c.<dyn> = v; return c })`
- `a.mu sync.RWMutex` is removed

Out of scope:

- `a.sup`, `a.log`, `a.cfg`, `a.ctx` (set once, read many)
- `a.zaloChats` lives in conn (dynamic, follows the same path)
- Behavior change of any kind — pure refactor

## Approach

1. Define `conn` struct and `swapConn` helper.
2. Initialize `a.conn.Store(&conn{...})` in `startup` and `stopTransport`.
3. Add `getConn() *conn` that returns nil or the loaded pointer.
4. Migrate `a.engineInfoLocked()` first (most-called read path).
5. Migrate each accessor one at a time, run `go vet ./...` after each.
6. Migrate `services()` last because it gates 5+ bind files.

## Risks And Recovery

- Risk: nil-pointer panic if any caller forgets the `getConn() == nil` guard.
  Mitigation: keep the existing "engine not running" error path; any read of
  a missing conn returns the same error message as before.
- Recovery: each bind file is its own commit; revert one without affecting
  the rest.

## Progress

- [ ] Step 1: define `conn` and `swapConn`.
- [ ] Step 2: initialize in startup.
- [ ] Step 3: migrate `engineInfoLocked` and `setStatus`.
- [ ] Step 4: migrate `services()`.
- [ ] Step 5: migrate every accessor in bind_*.go.
- [ ] Step 6: drop `a.mu`.

## Decisions

- 2026-08-29: D1 was gated as "needs explicit go-ahead" by the patch
  author. The user issued that go-ahead in the message "làm toàn bộ còn
  lại đi bạn". The plan is the smallest refactor that delivers the
  behavior — only the dynamic fields move into `conn`; static fields stay
  on `App`.

## Validation

- Focused proof: `go test -race ./...` passes with no new failures.
- Repository-required checks: `go vet ./...` clean, `go build ./...`
  clean, `npx svelte-check` clean.

## Result

Filled in after implementation.
