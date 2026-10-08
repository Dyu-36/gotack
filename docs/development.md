# Development

Clone only `github.com/Dyu-36/gotack`. The main branch contains all engine source,
terminal code, desktop code, licenses and release tooling. One `go.mod` builds
both interfaces; no engine checkout, download, pin file or source override is needed.

Use Windows x64, PowerShell 7, Go from `go.mod`, Node.js 24, pnpm 11.20.0
and Wails v2.15.0 for the complete desktop product.

```powershell
pnpm --dir frontend install --frozen-lockfile
wails generate module
go test -tags gotacktest -mod=readonly ./...
go vet -tags gotacktest -copylocks=false -mod=readonly ./...
pnpm --dir frontend check
pnpm --dir frontend test
pnpm --dir frontend build
go build -o build/bin/gotack.exe ./cmd/gotack
./build/bin/gotack.exe chat --workspace .
wails build -clean
```

The inherited engine's cross-package test helpers and tests use `gotacktest`;
production builds omit that tag. The JSON schema alias uses a value receiver,
so engine vet disables only the copylocks analyzer.

`gotack chat` reads prompts interactively and streams responses. `gotack run`
accepts a prompt after its flags. `--session ID` resumes persisted messages.
Both use the shared IPC client and process supervisor. `gotack serve` starts the
same server directly and accepts `--host`, `--data-dir`, and `--debug`.
The former `server` command remains an alias for custom binary configurations.

Configuration paths, `.tack` workspace databases, desktop settings, global
provider credentials, trust decisions, and the IPC endpoint remain compatible.
The terminal reuses the desktop's configured engine override, workspace models,
and trust store. New terminal workspaces request tool permissions by default;
`--yolo` is an explicit opt-in. Loading project prompts/skills requires an existing
trust decision made through the desktop when protected resources are present.

For validated builds and the complete portable distribution:

```powershell
./scripts/build-engine.ps1
$env:GOTACK_TEST_ENGINE = (Resolve-Path build/bin/gotack.exe).Path
$env:GOTACK_REQUIRE_ENGINE = '1'
go test -tags gotacktest -count=1 -run '^TestBridge' -v .
./scripts/build-product.ps1
```

All product components build from the same Gotack commit. The build manifest
records the source SHA256 digest, commit, protocol, executable checksum, and
validation results. Both binaries embed the same digest and reject stale engines.
Direct development builds always enforce protocol compatibility and check the
commit when Go embeds it. The portable output contains `gotack.exe`,
`gotack-desktop.exe`, Python timetable resources, notices and documentation.
Release automation inventories and attests that exact archive.

Build output belongs in `build/bin`; do not pass a directory to Wails `-o`.
The post-build hook builds the terminal/server next to the desktop. The pre-build
hook stops only processes running from this repository's output directory.
Generated frontend bindings, distributions, runtime downloads and artifacts are
ignored. Use `scripts/clean.ps1` for generated output cleanup.

Engine history was merged into Gotack before importing source from revision
`00205e5f73fa9bac13c46e055a7a076b3bb51d50`. Its authoritative license and notices
are preserved in `internal/agentcore/`. The historical engine repository is no
longer an input to any build, test, or release command.
