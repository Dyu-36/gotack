# Execution Plan: Thermo-Nuclear Maintainability Hardening

Date: 2026-09-02

## Status

Completed — 2026-09-02

## Outcome

The Gotack backend/host codebase is consolidated on one `main` branch, builds
from a clean checkout, and carries executable guards for the architecture rules
that were previously prose-only. Confirmed dead code and orphaned artifacts
were removed, the Wails binding surface and REST/SSE engine boundary were
closed, approval guidance was reconciled with the interactive-by-default ADR,
and repository validation now covers formatting, vetting, static analysis,
unreachable functions, generated bindings, frontend compatibility, and the
Windows portable package.

## Scope Applied

Reviewed all Go source in the repository root, `internal/**`, and `cmd/**`, plus
Go tests, `README.md`, `AGENTS.md`, architecture decisions, contracts, and
execution plans. No committed source changes were made under `frontend/`,
`build/`, `scripts/`, or `third_party/crush/`; generated frontend bindings were
created only inside CI workspaces for validation.

## Baseline Diagnostics

The review started from `main` commit
`2491cd5638a49aaeaaa41ba174a52585bf53e1a5`.

- The Go CI job could not compile a clean checkout because `main.go` embedded a
  missing generated `frontend/dist` directory.
- The frontend CI job ran before Wails generated `frontend/wailsjs`, so
  `svelte-check` could not resolve the Wails runtime module.
- `go test ./...` exposed host-dependent attachment basename handling for
  Windows paths on Linux.
- `deadcode -test ./...` found one unreachable method:
  `internal/recall.Store.Browse`.
- Three host-only SSE callbacks were exported methods on `App`, unintentionally
  widening the Wails binding surface.
- The UI event forwarder's delay override was assigned by tests but ignored by
  production code.
- Root-level `prompt1.md` and `prompt2.md` were orphaned experimental artifacts.
- Contract and completed-plan narration still referred to removed `DoneSink`,
  `App.RunDone`, and `Tracker.SessionDone` names.

## Hardening Changes

- Split frontend assets by build tag: production builds embed the real bundle;
  non-production Go tooling uses a minimal in-memory filesystem. Clean-checkout
  tests no longer depend on generated UI output.
- Added cross-platform attachment display-name normalization accepting both
  slash conventions, and routed cache, transform, prompt, and replay paths
  through it.
- Replaced exported host-only SSE sink methods with explicit function callbacks
  and added an exact reflective allowlist for all Wails-bound methods. The test
  also rejects exported `App` fields.
- Added a repository test that parses every scoped Go file and rejects imports
  of `third_party/crush/internal/...`.
- Removed the unreachable recall method and orphaned prompt artifacts.
- Made the UI event retry-delay override effective and covered it directly.
- Reconciled Zalo, scheduler, Wails, approval, README, and historical plan text
  with the implementation.
- Expanded CI and release validation with pinned `staticcheck`, pinned
  `deadcode`, `gofmt`, generated-event drift, Wails binding generation, and the
  existing repository/frontend/package checks.

## Architectural Invariants Verified

- 142 Go implementation files were checked; the largest is
  `internal/skillmanage/manager.go` at 529 lines. Every implementation file is
  below the 1,000-line limit.
- Host packages do not import `third_party/crush/internal/...`; the invariant is
  now enforced by `TestHostDoesNotImportCrushInternals`.
- Engine communication remains behind `internal/crushapi` and REST/SSE.
- Wails-bound methods remain in package `main`, and the exact exported surface
  is enforced by `TestWailsBindingSurfaceMatchesContract`.
- Interactive approval remains the default; automatic approval is an explicit
  opt-in, consistent with decision 0002 and its negative tests.

## Validation

The final PR head `89c17dc107c5c652dbb577a396db2bf50b33bce7`
passed GitHub Actions run `33583227648`:

- `go test ./...`;
- `go vet ./...`;
- pinned `staticcheck ./...`;
- pinned `deadcode -test ./...` with no findings;
- `gofmt` and generated UI-event drift checks;
- Wails JavaScript binding generation;
- repository invariant checks;
- `pnpm --dir frontend check`;
- `pnpm --dir frontend test`;
- `pnpm --dir frontend build`;
- Windows Wails application build;
- pinned Crush build;
- `office`, `guard`, `memory`, `skills`, and `recall` helper builds;
- portable-directory assembly and artifact upload.

An additional `go test -race -count=1 ./...` completed successfully in run
`33582827394`. The final guard-only follow-up also ran `go test ./...` before it
was published.

## Repository Consolidation

PR #10 was squash-merged to `main` as
`b2cc47b7c0644c83dfa25ee5783e5ab67ae46291`. All verified auxiliary remote
branches were deleted after the merge, including the review branches. The
repository branch listing then contained only `main`.

## Result

The hardening pass is complete. The merged change touched 37 files with 362
additions and 318 deletions, while preserving the explicitly excluded source
trees. Automated source, race, frontend-compatibility, and Windows packaging
proof is green.

A credentialed, interactive packaged-app smoke path was not executed in this
review environment. That remaining live-app proof belongs to the separate
Hermes learning-loop plan, which stays active rather than being represented as
completed without evidence.
