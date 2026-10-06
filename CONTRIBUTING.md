# Contributing to Gotack

Gotack ships a Windows x64 desktop app. Use PowerShell 7 x64, the Go version in
`go.mod`, Node.js 24, pnpm 11.20.0 and Wails v2.15.0.

## Local checks

From the repository root:

```powershell
pnpm --dir frontend install --frozen-lockfile
wails generate module
go test -mod=readonly ./...
go vet ./...
pnpm --dir frontend check
pnpm --dir frontend test
pnpm --dir frontend build
```

Run `gofmt` on changed Go files. When changing event names, run
`go run ./internal/uievents/gen/main.go` and include the generated TypeScript change.
Wails bindings in `frontend/wailsjs/` are generated locally and are not committed.

## Architecture and changes

Read [the architecture guide](docs/architecture.md) before moving code between
the host and engine. Keep `main.App`'s binding names, argument shapes and JSON
fields stable during internal refactors. Keep tests beside their Go package.
Prefer focused packages in `internal/` and small changes with a clear purpose.

The Settings modal owns navigation and provider/theme drafts. Individual settings
panels own their operations and remain mounted across tab changes.

For a pull request, explain the resulting behavior and the checks you ran.
Include screenshots for visible UI changes. Report vulnerabilities through
[the security policy](SECURITY.md).

## Engine and product validation

The engine is a separate repository. Its checkout must match `.tack-pin` and be
clean before packaging. Follow [the development guide](docs/development.md) to
run required real IPC tests or build the complete Windows distribution.

CI checks tests, vet, staticcheck, deadcode, formatting, generated events, frontend
checks and product integration. The shared toolchain setup lives in
`.github/actions/setup-desktop/action.yml`.

## Contributions and licensing

Gotack host source uses Apache-2.0; see [LICENSE](LICENSE). Keep existing notices
and distinguish host changes from the separately licensed engine and dependencies.
