# Gotack Agent Instructions

## Project layout

- Go/Wails desktop host: repository root and `internal/`
- Svelte frontend: `frontend/src/`
- Build and release tooling: `scripts/`, `.github/workflows/`, and `build/`
- Pinned local agent engine revision: `.tack-pin`

## Setup and validation

Use the versions declared by the repository and CI.

```powershell
pnpm --dir frontend install --frozen-lockfile
wails generate module
go test ./...
go vet ./...
pnpm --dir frontend check
pnpm --dir frontend test
pnpm --dir frontend build
```

When UI event names change, regenerate and keep the committed generated event file in sync:

```powershell
go run ./internal/uievents/gen/main.go
```

## Generated and local files

Do not commit:

- `frontend/wailsjs/`
- `frontend/dist/`
- `build/bin/`
- `third_party/engine-source/`
- local runtime, cache, editor, or agent configuration

`frontend/src/platform/events.generated.ts` is intentionally committed and must remain in sync with the Go event definitions.

Do not change `.tack-pin` unless intentionally updating the pinned engine revision.

## Editing rule

Do not add comments when editing source code unless explicitly requested.
