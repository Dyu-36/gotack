# Cleanup: dead state, duplicated blocks, documentation drift

Status: applied. Opened 2026-08-31 against `552225ee`; the code half was
executed the same day on branch `chore/cleanup-and-refactor-2026-08-31`.

## Goal

Remove state the host accepts and then discards, collapse blocks that are
duplicated verbatim, and keep the documentation reconcilable with the code.

## Verification method

The first pass was read-only over the files listed as evidence below. It did
**not** run `go build`, `go vet`, `staticcheck`, `gofmt -l` or `go test`, and
GitHub code search had no index for this repository, so every "never read" or
"unused" claim in that pass was a static reading rather than a tool result.

The code half was then executed on a local checkout with the toolchain
available, so the removals below are now tool-verified:

```sh
gofmt -l $(git ls-files '*.go')   # empty
go build ./...                    # ok
go vet ./...                      # ok
go test ./...                     # every package ok
staticcheck ./...                 # only the intentional ST1005 findings remain
deadcode -test ./...              # empty
pnpm --dir frontend check         # 0 errors, 0 warnings
pnpm --dir frontend build         # ok
actionlint                        # clean
```

Two corrections to the first pass, recorded so the earlier numbers are not
quoted again:

- The `gofmt -l .` count of 22 files was an artifact of a CRLF working tree, not
  formatting debt. Re-checked against an LF export, the real set was 9 files.
  `.gitattributes` now pins the tree to LF so the number cannot drift again.
- `bind_zalo.go` was reported as having misaligned struct tags. Its actual
  defect was mixed line endings; the only genuine alignment hunk was in
  `app.go` (`zalo` / `officeSeeder` fields).

Also corrected: `go build` caught one reference to the removed `conn.attachCtx`
field in `bind_engine.go` that the read-only pass had missed, which is exactly
why the gates above are run rather than trusted by inspection.

## A. Dead state and dead configuration

### A1. `conn.attachCtx` is written and never read

Evidence: declared on the `conn` struct in `app.go`; assigned in
`bind_engine.go` `tryConnect` (`c.attachCtx = ctx`) and cleared in
`stopTransport` (`c.attachCtx = nil`). The attach scope actually used is the
local `ctx` variable passed into the connect goroutine, not the field.

Why it matters: a context stored on shared state implies a cancellation
contract that does not exist. The next person to touch reconnection will assume
cancelling `attachCtx` aborts an attach, and it will not.

Action: delete the field and both assignments. Confirm with a repo-wide grep
first.

### A2. `officeSeeder.sourceDir` is written and never read

Evidence: `office_seed.go` assigns `s.sourceDir = source` in `startup()`. The
seeder's consumers (`CrushEnv`, `SkillsPath`, `ensureOfficeSeed`,
`registerOfficeTools`) never read it. Verified by reading `office_seed.go`.

Why it matters: it looks like a cached resolution that later code can reuse, so
a future change will read a value that was only correct at startup.

Action: delete the field, or read it in `ensureOfficeSeed` instead of
re-resolving the source directory.

### A3. `SettingsInfo.SmallModel` is accepted and discarded

Evidence: `bind_config.go` `SaveSettings` writes
`a.cfg.SmallModel = strings.TrimSpace(s.Model)` — it stores `s.Model`, not
`s.SmallModel`. `settings_crush.go` `applyCrushSettings` never reads
`s.SmallModel`; it calls `setModel("large", modelID, ...)` and
`setModel("small", modelID, ...)` with the same `modelID`. Verified by reading
both files.

Why it matters: the field is on the Wails boundary and in `desktop.ts`, and the
contract document claimed it selected the small-task model. Anything built on
it silently does nothing.

Action: either implement a real second selector, or remove `small_model` from
`SettingsInfo`, `desktop.ts` and the contract together. The contract now states
the real behavior; hard rule 8 forbids leaving it in the middle state.

### A4. `SettingsInfo.AutostartEngine` cannot be turned off

Evidence: `bind_config.go` `GetSettings` returns a hardcoded
`AutostartEngine: true`; `SaveSettings` writes a hardcoded
`a.cfg.AutostartEngine = true`. `reapplySavedWorkspaceSettings` still passes
`a.cfg.AutostartEngine` through, but nothing consumes it as a decision.

Why it matters: two commits (`bb35a09d`, `26ca7559`) exist specifically to
preserve this preference through the settings modal. That work is now moot, and
the stored config key implies a user choice that no longer exists.

Action: remove the field from the boundary, or restore a real toggle. Delete
the now-pointless preservation logic either way.

### A5. The `conn` comment in `app.go` describes fields that do not exist

Evidence: the comment lists `zaloChats`, `zaloCancel` and `zalo` as dynamic
fields "swapped atomically through a `*conn` pointer", and notes "the previous
`sync.RWMutex` is gone". The `conn` struct has no `zaloChats` or `zaloCancel`
field, and `zalo` lives on `App`.

Why it matters: this is the explanatory comment for the concurrency model. It
is the first thing read before changing reconnection, and it is wrong.

Action: rewrite it against the current struct and drop the historical note
about the removed mutex.

### A6. `bind_changes.go` points at a file that does not exist

Evidence: the header comment says "Full editor features stay out of scope, see
`docs/roadmap.md`." There is no `docs/roadmap.md`.

Action: point at `../README.md` ("Initial scope") or delete the reference.

### A7. Office source resolution is Windows-only while its sibling is not

Evidence: `office_seed.go` `resolveOfficeCommand()` appends `.exe` only when
`runtime.GOOS == "windows"`, but `resolveOfficeSourceDir()` hardcodes
`filepath.Join(candidate, "officecli.exe")`.

Why it matters: on macOS and Linux the seed path can never match, so the whole
seeding branch is unreachable there. The asymmetry between two adjacent helpers
reads as an oversight rather than a decision, so someone will "fix" one half.

Action: make both helpers use the same platform-aware binary name, or state
explicitly in the code that seeding is Windows-only. The contract and README
now document the current limit.

### A8. `cfg.Zalo.Token` and `cfg.Zalo.AllowedChats` are migration-only

Evidence: `app.go` passes them once into `a.zalo.ImportLegacy(...)`;
`bind_zalo.go` `RemoveZaloToken` clears them. Live state is
`<configDir>/zalo.json`. Nothing else writes them.

Why it matters: `AllowedChats` looks like the access-control list but is not —
the real mechanism is pairing. README described the old model until this change.

Action: keep the import for one release, mark both fields deprecated in
`internal/appconfig`, and record a removal target.

## B. Duplicated blocks

### B1. Catalog-workspace bootstrap duplicated in `bind_config.go`

Evidence: `ListProviders()` and `RevealProviderAPIKey()` both contain the same
sequence: `filepath.Join(appconfig.Dir(), "catalog-workspace")`,
`os.MkdirAll(..., 0o755)`, `svc.api.CreateWorkspace(ctx, catalogPath, false)`,
and the same error wrapping.

Why it matters: two copies of a filesystem-plus-remote-call sequence. A fix to
one (permissions, error text, reuse of an existing workspace) will not reach
the other.

Action: extract `func (a *App) catalogWorkspace(ctx) (string, error)`.

### B2. Stream swap duplicated in `bind_workspace.go`

Evidence: `activateWorkspace()` and `activateAssistantWorkspace()` both run
`swapConn` → capture `cancel`/`streamCtx` → `context.WithCancel(a.ctx)` →
`cancel()` → `startStream(...)` → `a.resetZaloSessions()` →
`a.registerOfficeTools(desc.WorkspaceID)`. They differ only in `Open` versus
`OpenWithDataDir`. `bind_engine.go` already has `replaceWorkspaceStream()`
covering most of it.

Why it matters: this is the reconnection-critical path. Three near-copies of
stream ownership is exactly where a leaked goroutine or a double-cancel hides.

Action: have both call one helper that takes the already-opened workspace
descriptor.

### B3. The Crush pin exists in three files

Evidence: `CRUSH_COMMIT: 6d14dd93...` in `.github/workflows/ci.yml` and
`.github/workflows/release.yml`, plus the same SHA in `third_party/README.md`,
which instructs updating "this file and the CI/release workflow in the same PR".

Why it matters: a hand-synced constant across three files. A release built from
a different Crush commit than CI verified is a silent correctness problem.

Action: move the SHA to one tracked file (for example `.crush-pin`) and have
both workflows read it.

## C. Structure

### C1. Merge the two single-method bind files

Evidence: `bind_bridge.go` is 399 bytes and holds only
`BackendReady() bool { return true }`. `bind_dialog.go` is 381 bytes and holds
only `SelectWorkspace()`. `bind_dialog.go` is also the only bind file without
the `// bind_*.go -- role:` header every sibling has.

Why it matters: file-per-binding-group is a good rule, but a file per method
stops paying for itself. Thirteen root files make the "thin desktop layer"
look larger than it is.

Action: fold both into one `bind_shell.go` (bridge readiness plus native
dialogs), and give it the standard role header.

### C2. Move the connection state machine out of `bind_engine.go`

Evidence: `bind_engine.go` is the largest root file (10,621 bytes). It holds
four bound methods (`EngineStatus`, `StartEngine`, `StopEngine`,
`ReconnectEngine`) plus roughly ten unbound helpers: `tryConnect`, `connect`,
`commitAttach`, `attachStream`, `replaceWorkspaceStream`, `stopTransport`,
`transportLost`, `failConnect`, `services`, `bridgeServices`.

Why it matters: hard rule 1 only requires *bound methods* to stay in
`package main`. The supervisor logic is the most concurrency-sensitive code in
the repo and today it cannot be unit-tested without the Wails app object —
which is why `bridge_smoke_test.go` has to reach through `App`.

Action: extract the supervisor into `internal/enginelink` with an explicit
interface, leaving the four bound methods as thin delegations. This also makes
A1 and B2 straightforward.

### C3. Name the two non-bind files in `package main` for what they are

Evidence: `office_seed.go` and `settings_crush.go` are implementation, not
bindings, and were absent from the AGENTS.md layout table until this change.

Why it matters: the table is declared authoritative, so anything missing from
it has no stated owner and accumulates unrelated helpers.

Action: they are now listed. Longer term, `settings_crush.go` belongs next to
the Crush config client and `office_seed.go` next to `internal/officecli`;
move them when C2 lands.

### C4. User-facing Vietnamese strings are scattered across Go files

Evidence: `bind_dialog.go` has `Title: "Chọn thư mục làm việc"`;
`bind_session.go` composes `"Hãy xem và xử lý tệp/các tệp đính kèm sau:"`;
`app.go` returns the error `"chưa chọn mô hình"`.

Why it matters: three files, three languages of error reporting (Go errors are
otherwise English), and no place to change wording or add a second locale.

Action: collect user-visible strings into one `strings.go` in `package main`,
or return typed errors and let the UI supply the wording.

### C5. `frontend/src/features/` holds one feature

Evidence: `frontend/src/` has `app/`, `components/`, `features/`, `lib/`,
`platform/`; `features/` contains only `conversations/`.

Why it matters: four competing organizing schemes for a small UI means no rule
decides where a new file goes, so placement becomes arbitrary.

Action: either move the rest of the UI into `features/`, or drop the directory
and keep `components/` plus `lib/`. Write the chosen rule into `docs/README.md`.

### C6. `resources/bin/` is prepared but never shipped

Evidence: `scripts/prepare-resources.ps1` provisions `resources/bin/` with
`officecli.exe`, an embedded Python 3.12.8, `uv`, `ortools` and `openpyxl`.
`release.yml` copies only `build/windows/resources/officecli.exe` and
`resources/skills`; nothing copies `resources/bin/`. No workflow runs the
script.

Why it matters: the contract document advertises a bundled timetable
solver/exporter usable "without separate user setup". In a released ZIP that
runtime is absent, so the feature works only on a developer machine that ran
the script.

Action: either bundle `resources/bin/` in `release.yml` and run
`prepare-resources` in the release job, or scope the claim down. `scripts/README.md`
and `README.md` now record the gap.

## D. Formatting is not enforced

Evidence: `bind_zalo.go` `ZaloConfigInfo` has extra spaces before the struct
tags on `Enabled`, `PairedChats`, `PairingCode`, `HasToken`, `BotName`,
`TokenSuffix` and `Running`; `app.go` has the same on the `zalo` and
`officeSeeder` fields. `gofmt` would rewrite both. `ci.yml` runs `go test` and
`go vet` but never `gofmt -l`.

Why it matters: unformatted code in `main` proves the check is missing, and
every later diff carries unrelated whitespace noise.

Action: add a `gofmt -l .` gate that fails on non-empty output, then run
`gofmt -w .` once.

## E. Documentation corrected in this change

- `README.md`: Zalo pairing instead of allow-list; one model selector; the
  15 `internal/` packages; `cmd/`, `resources/`, `.agents/`, `.harness-core/`
  and `.github/` added to the layout; `docs/` subdirectories corrected (there
  is no `architecture/` or `guides/`); baseline gained the installed rows it
  omitted and the Go direct dependencies; the caret-range deviation restated
  (five ranges, two of them runtime, not "most dev dependencies"); project
  status now separates what CI proves from what is manual only.
- `AGENTS.md`: layout table corrected; hard rule 8 added.
- `docs/contracts/wails-bindings.md`: `RevealProviderAPIKey` and
  `DeleteProvider` documented; `small_model` and `autostart_engine` recorded as
  inert; the permission-skip default and `C:\` workspace stated; generated
  events file called out; rule 2 restated to name the secret-reveal exception.
- `docs/README.md`: `product/` and `decisions/` marked as empty placeholders;
  noted that the Crush and Zalo boundaries have no contract document.
- `scripts/README.md`: "thin wrappers" claim corrected, Windows-only stated,
  the `resources/bin/` packaging gap recorded.

## F. Documentation still open

- `internal/crushapi` and `internal/zalo` are the two highest-churn external
  boundaries and have no `docs/contracts/` document, even though hard rule 7
  requires contract docs for external boundaries. Add
  `contracts/crush-rest-sse.md` and `contracts/zalo-bot.md`.
- `docs/product/` and `docs/decisions/` are upstream harness placeholders. For
  a release candidate with an engine supervisor, a Zalo bridge and a bundled
  Office runtime, at least the permission-skip trust model and the
  single-model-selector choice deserve real decision records.

## Sequencing

1. D (`gofmt` gate) — independent, and keeps later diffs clean.
2. A5, A6 (comment fixes) — no behavior change.
3. A1, A2 (dead fields) — after a grep confirms no reader.
4. B1, C1 — mechanical, no behavior change.
5. A3, A4 — boundary change; touches Go, `desktop.ts` and the contract
   together, so it needs one decision on whether a second model selector is
   wanted.
6. C2, then B2 — the largest change; do it alone.
7. B3, C6 — release-pipeline work.
8. A7, A8, C4, C5, F — as capacity allows.

## Validation

Each step: `gofmt -l .` empty, `go vet ./...` clean, `go test ./...` green,
`pnpm --dir frontend check` green. For A3/A4 additionally confirm the settings
modal still round-trips provider, model, thinking and credential. For C2/B2
additionally exercise engine restart, workspace switch and transport loss, since
`bridge_smoke_test.go` is the only automated cover for that path.
