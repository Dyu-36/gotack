# Development and product builds

Use PowerShell 7 x64 on Windows for the shipped product. Requirements are the Go
version in `go.mod`, Node.js 24, pnpm 11.20.0, Wails CLI v2.15.0 and Python for packaging.
The shared CI action pins the frontend, Wails and packaging tool versions.
Engine tests require Git Bash with `sh` and `bash` on `PATH`. Race-detector checks
also require cgo and a C compiler available to Go.

## Engine checkout

The engine is maintained in its own Go module and repository, not nested as a
host-repository submodule:

```powershell
git clone --no-checkout https://github.com/Dyu-36/tack-engine.git tack-engine-source
git -C tack-engine-source checkout --detach (Get-Content .tack-pin -Raw).Trim()
```

The committed `.tack-pin` identifies the exact engine source revision. Keep the
checkout clean: the build checks its revision and source contents before
building. The engine repository retains its required license notices. Pass
`-EngineSource <path>` to use a checkout outside the workspace.

## Desktop development

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0
pnpm --dir frontend install --frozen-lockfile
wails generate module
./scripts/build-engine.ps1 -EngineSource tack-engine-source
wails dev
```

The CLI stays at v2.15.0 for Go 1.27 binding generation. `go.mod` deliberately
replaces the Wails application module with v2.12.0: v2.13–v2.16 bundle a Svelte 5
reconnect overlay but instantiate it with `new Overlay(...)`, causing
`Cannot read properties of null (reading 'nodes')` in `/wails/ipc.js`. The v2.12.0
bundle uses the compatible constructor API; no module-cache patches are needed.
This pins the whole Wails runtime, not only its overlay. Keep the replacement
until an upstream release passes the browser reconnect smoke check:

1. Open the Wails DevServer URL printed by `wails dev`, not Vite's port 5173.
2. Stop the backend. The reconnect overlay must appear without a JavaScript error.
3. Restart `wails dev` on the same address. The overlay must disappear, and
   `window.go.main.App.ListProviders()` must resolve again without reloading the tab.

Windows x64 Wails builds also verify and bundle the pinned engine through a
post-build hook. A tested copy is cached in `resources/bin/` so `wails build
-clean` retains it. If no verified copy exists, the hook builds from
`tack-engine-source` (or the checkout in `GOTACK_ENGINE_SOURCE`). The build
fails with setup instructions if that source is missing. Keep the generated
`resources/` directory beside `gotack.exe`; no engine entry in `PATH` is needed.
Reconnect searches again if the engine was installed after Gotack started.

For a browser-only UI preview, use `pnpm --dir frontend dev`. Conversations use
DEV-only fixtures when the desktop bridge is unavailable. Production requires
the Wails bridge. See [CONTRIBUTING](../CONTRIBUTING.md) for checks.

## Build outputs

Wails v2 fixes the desktop output directory at `build/bin/<outputfilename>`, so
`wails.json` cannot relocate it. Keep one output per purpose:

- `build/bin/` — Wails output: `gotack.exe` plus the bundled `resources/tack-engine.exe`.
- `resources/bin/tack-engine.exe` — tested engine cache that survives `wails build -clean`.
- `artifacts/` — the distribution written by `scripts/build-product.ps1`.

Always build with `wails build -clean`: it drops stale entries such as a locked
`gotack.exe~`. Never pass a directory to `-o`: Wails joins that value onto
`build/bin`, so `-o pi-update/gotack.exe` leaves a duplicate host and a second
engine copy behind. `scripts/verify-build-output.ps1` checks the layout and
`scripts/ensure-engine.ps1` refuses to bundle an engine outside `build/bin`; the
product build packages only an output that passes both.

Windows cannot replace a running host binary, so the Wails pre-build hook and
`scripts/build-product.ps1` run `scripts/stop-app.ps1` first. It stops
`gotack.exe` and `tack-engine.exe` started from `build/bin`, leaves same-named
processes started elsewhere alone, and fails if the output files stay locked.

## Real host-engine contract tests

```powershell
$env:GOTACK_TEST_ENGINE = (Resolve-Path build/bin/resources/tack-engine.exe).Path
$env:GOTACK_REQUIRE_ENGINE = '1'
go test -mod=readonly -count=1 -run '^TestBridge' -v .
```

These variables make tests use the actual pinned sidecar. Without an engine path,
ordinary `go test ./...` skips those integration tests.

## Complete Windows distribution

```powershell
./scripts/build-product.ps1 -EngineSource <path-to-exact-pinned-tack-engine>
```

The build runs desktop tests/vet and frontend checks/tests/build, verifies and
builds the pinned engine, builds Wails, prepares the offline Python/OR-Tools
runtime and exercises real IPC. Timetable validation requires CP-SAT and round
trips of both canonical Excel templates.

The portable archive contains `gotack.exe`, `resources/tack-engine.exe`, the
Python runtime, host and engine licenses, a product manifest and SHA-256 checksums.
Product CI checks the complete distribution and concurrency behavior; release
automation additionally writes an SBOM and build attestations.

## Local cleanup

Preview cleanup before removing generated local files:

```powershell
./scripts/clean.ps1 -WhatIf
./scripts/clean.ps1
./scripts/clean.ps1 -BuildOutputs -Dependencies -WhatIf
./scripts/clean.ps1 -BuildOutputs -Dependencies
```

The default removes `artifacts/` and the empty legacy `internal/changes/` directory.
`-BuildOutputs` also removes `build/bin/`, `frontend/dist/` and `frontend/wailsjs/`.
`-Dependencies` also removes the two Node dependency directories. Restore them
with the install, bindings and build commands above before developing again.
Build icons, manifests, source, lockfiles, engine checkout and local history
backups are retained. History archives in `.history-backup/` are ignored.
