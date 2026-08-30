# Execution Plan: Zalo Settings And Timetable Integration

Date: 2026-08-30

## Status

Completed

## Outcome

Gotack users can select and configure Zalo from Settings, validate a bot token through the real desktop boundary, and use the proven timetable skill/runtime from `C:\stack` inside Gotack/Crush to produce a validated school timetable and Excel export.

## Context

- `docs/contracts/wails-bindings.md`: UI/host contract for Zalo and resource installation.
- `docs/plans/completed/release-hardening.md`: accepted Zalo product behavior and credential boundary.
- `frontend/src/components/SettingsModal.svelte`: Settings interaction.
- `frontend/src/platform/desktop.ts`, `bind_zalo.go`, `internal/zalo/`: desktop boundary and Zalo runtime.
- `office_seed.go`, `resources/`, `internal/officecli/`: bundled Crush resource installation.
- `C:\stack\.tack\skills\timetable` and `C:\stack\.tack\agents\timetable-solver.md`: requested timetable authority and implementation source.

## Scope

In scope:

- Reproduce and fix the Zalo tab/control interaction without exposing the token.
- Preserve the existing write-only token contract and validate against the official bot API.
- Integrate the timetable skill, solver, exporter, references, and any required agent/tool dependencies through Gotack's existing resource-install path.
- Add focused automated coverage and verify the user-visible/runtime path.

Out of scope:

- Redesigning Settings or changing Zalo Bot Platform policy.
- Replacing Crush agent execution or creating a second scheduling implementation.
- Modifying the source repository at `C:\stack`.

## Approach

1. Establish the current behavior and separate pre-existing work from missing behavior.
2. Trace Settings event handling through the desktop Zalo manager and add the smallest coherent fix plus regression proof.
3. Trace Gotack's bundled resource installer, import the timetable package without altering its algorithm, and wire required metadata/dependencies.
4. Run focused Go/frontend/resource tests, exercise Settings with `agent-browser`, and execute the timetable solver/exporter on a deterministic fixture.
5. Record evidence, limitations, and move this plan to completed only after all required proof passes.

## Risks And Recovery

- Existing dirty changes overlap Zalo and resource wiring. Mitigation: inspect diffs first and patch only the missing behavior; recovery is reverting only hunks/files introduced by this plan.
- Bot validation reaches an external service. Mitigation: perform only identity/connection checks and never persist or print the supplied secret outside the app's normal credential store.
- Timetable runtime may depend on Python packages unavailable on a clean machine. Mitigation: identify and package/check dependencies explicitly, and report any installer boundary that cannot be proven.
- Copying assets can drift from `C:\stack`. Mitigation: preserve the upstream directory structure and add a manifest/content test at Gotack's installation boundary.

## Progress

- [x] Read repository workflow and identify product/contract authority.
- [x] Reproduce and fix Zalo Settings selection/configuration.
- [x] Integrate timetable skill/runtime/agent dependencies.
- [x] Run focused, integration, UI, and timetable artifact validation.
- [x] Record results and close the plan.

## Decisions

- 2026-08-30: Treat the user's explicit request and `C:\stack` timetable package as authority for feature parity; reuse the existing solver/exporter rather than implementing a new scheduler.
- 2026-08-30: Preserve all pre-existing dirty work in both repositories and make no writes to `C:\stack`.
- 2026-08-30: Keep the Stack timetable package byte-for-byte at version 2.2.0. Gotack's Crush runtime has no Stack-style `task(agent_id=...)` tool, so the skill follows its documented direct solver fallback; no incompatible custom-agent file is installed.
- 2026-08-30: Bundle Python 3.12, `ortools`, and `openpyxl` beside the executable so scheduling does not depend on a user's system Python.
- 2026-08-30: Enable HTTP/2 negotiation for the official Zalo Bot API after the live `getMe` probe proved that the service responds with HTTP/2 frames.

## Validation

- Focused proof: `go test ./...` passes, including Zalo config/JSON contract checks and the timetable solver/exporter smoke test.
- Integration or end-to-end proof: `agent-browser` exercised Settings -> Zalo -> enable -> token -> save -> connected status -> disconnect; the supplied stored token passed the live `getMe` test; `GOTACK_RESOURCE_ROOT=build/bin/resources go test ./internal/officecli -run TestBundledTimetableSolverAndExporter` produced and opened a valid XLSX using the packaged Python runtime.
- Repository-required checks: `go vet ./...`, `pnpm --dir frontend check`, and `scripts/build.ps1` pass. `git diff --check` still reports a pre-existing extra blank line in `internal/crushapi/config_mutation.go`; that unrelated dirty file was not changed for this task.

## Result

Zalo Settings is interactive and keeps its enabled/token state synchronized across save and disconnect. The Zalo HTTP client now negotiates HTTP/2, and the real token identity check succeeds. Gotack packages the Stack timetable 2.2.0 skill, solver, exporter, Python runtime and dependencies; both source and packaged smoke tests generate a conflict-free schedule and valid Excel workbook. The production Wails build completes with the timetable skill at `build/bin/resources/skills/timetable`.
