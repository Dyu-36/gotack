# third_party

Vendored upstream code is used only for contract inspection and release builds. It is not part of the Gotack Go module and remains ignored by the parent repository.

## Crush pin

- Upstream: `https://github.com/charmbracelet/crush`
- Pinned commit: `6d14dd93a9e526505f7de54ae5999431bc32a793`
- Contract checked: REST v1 sessions, agent/cancel, config/model, config/set, config/provider-key, permissions, question batches, workspace SSE (`message`, `run_complete`, `file`, permission/question events).
- Desktop integration: REST + SSE only through `internal/crushapi`; Gotack never imports `third_party/crush/internal/...`.

## Distribution policy

Release builds prefer a bundled Crush executable at `resources/crush.exe` next to `gotack.exe` on Windows (or `resources/crush` on Unix). If the bundle is absent, Gotack falls back to `crush` on `PATH`. A non-empty `engine_binary` setting is an explicit override and wins over both.

The release job must build Crush from the exact pinned commit above and place it in the Gotack artifact. This makes a release deterministic while retaining the PATH fallback for developer machines.

## Refresh procedure

Run `scripts/update-crush.ps1 -Commit <sha>` from the repository root on Windows/PowerShell. The script refreshes the ignored `third_party/crush` checkout, verifies the REST/SSE route markers Gotack relies on, and builds the bundled executable. After deliberately accepting an upstream contract change, update the pinned commit in this file and the CI/release workflow in the same PR.

## Rules

- Upstream source is read-only for desktop needs; desktop-only behavior belongs in `internal/`.
- Gotack talks to Crush over REST + SSE only.
- Go forbids importing `third_party/crush/internal/...` from another module, so the wire contract is re-declared in `internal/crushapi`.
- Never advance the pin without re-running the route/contract checks and the Gotack smoke suite.
