# third_party

Vendored upstream code is used only for contract inspection and release builds. It is not part of the Gotack Go module and remains ignored by the parent repository.

## Crush pin

- Upstream: `https://github.com/charmbracelet/crush`
- Pinned commit: the single tracked owner is `.crush-pin` at the repository
  root. The workflows, `scripts/update-crush.ps1`, and this document all read
  or reference that file.
- Gotack-specific engine compatibility patches live in `third_party/patches/*.patch`
  and are applied, in filename order, on top of the pin before Crush is tested or built.
- The proactive context patch preserves Crush's existing reserve for smaller
  models and caps the live conversation at 128,000 prompt-plus-completion tokens
  on larger contexts before the normal summarization/requeue path runs.
- Contract checked: REST v1 sessions, agent/cancel, agent/refresh-prompt, config/set, config/set-batch, config/remove, config/models, config/provider-key, config/refresh-oauth, permissions, question batches, workspace SSE (`message`, `run_complete`, `file`, permission/question events).
- Desktop integration: REST + SSE only through `internal/crushapi`; Gotack never imports `third_party/crush/internal/...`.

## Distribution policy

Release builds prefer a bundled Crush executable at `resources/crush.exe` next to `gotack.exe` on Windows (or `resources/crush` on Unix). If the bundle is absent, Gotack falls back to `crush` on `PATH`. A non-empty `engine_binary` setting is an explicit override and wins over both.

The release job must build Crush from the exact pinned commit plus the tracked patch set above and place it in the Gotack artifact. This keeps releases deterministic while retaining the PATH fallback for developer machines.

## Refresh procedure

Run `scripts/update-crush.ps1 -Commit <sha>` from the repository root on Windows/PowerShell (without `-Commit` the script reads `.crush-pin`). The script refreshes the ignored `third_party/crush` checkout, applies `third_party/patches/*.patch`, verifies the REST/SSE route markers Gotack relies on, and builds the bundled executable. After deliberately accepting an upstream contract change, rebase/refresh every affected patch and update `.crush-pin` in the same PR.

## Rules

- Upstream source is read-only for desktop needs; desktop-only behavior belongs in `internal/`.
- Gotack talks to Crush over REST + SSE only.
- Go forbids importing `third_party/crush/internal/...` from another module, so the wire contract is re-declared in `internal/crushapi`.
- Never advance the pin without re-running the route/contract checks and the Gotack smoke suite.
