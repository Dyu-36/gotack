# Development and product builds

Use PowerShell 7 x64 on Windows for the shipped product. Requirements are the Go
version in `go.mod`, Node.js 24, pnpm 11.20.0, Wails v2.15.0 and Python for packaging.
The shared CI action pins the frontend, Wails and packaging tool versions.
Engine tests require Git Bash with `sh` and `bash` on `PATH`. Race-detector checks
also require cgo and a C compiler available to Go.

## Engine checkout

The engine is a separate repository and is not committed inside Gotack:

```powershell
git clone --no-checkout https://github.com/Dyu-36/tack-engine.git third_party/engine-source
git -C third_party/engine-source checkout --detach (Get-Content .tack-pin -Raw).Trim()
```

Keep the checkout clean. The build verifies its revision and source contents.
Pass `-EngineSource <path>` to use a checkout outside the workspace.

## Desktop development

```powershell
pnpm --dir frontend install --frozen-lockfile
wails generate module
./scripts/build-engine.ps1 -EngineSource third_party/engine-source
wails dev
```

Windows x64 Wails builds also verify and bundle the pinned engine through a
post-build hook. A tested copy is cached in `resources/bin/` so `wails build
-clean` retains it. If no verified copy exists, the hook builds from
`third_party/engine-source` (or the checkout in `GOTACK_ENGINE_SOURCE`). The build
fails with setup instructions if that source is missing. Keep the generated
`resources/` directory beside `gotack.exe`; no engine entry in `PATH` is needed.
Reconnect searches again if the engine was installed after Gotack started.

For a browser-only UI preview, use `pnpm --dir frontend dev`. Conversations use
DEV-only fixtures when the desktop bridge is unavailable. Production requires
the Wails bridge. See [CONTRIBUTING](../CONTRIBUTING.md) for checks.

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
