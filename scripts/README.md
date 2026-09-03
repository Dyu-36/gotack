# scripts

Developer entry points. The build/resource helpers are PowerShell and Windows
only; `check-repository-invariants.mjs` is cross-platform and is the shared
local/CI owner for deterministic repository policy checks.

`build` stays a thin wrapper around Wails, Go and pnpm. `prepare-resources` is
deliberately not a thin wrapper: it provisions the bundled runtime.

| Script | Role |
| --- | --- |
| build | pnpm build, then wails build for the current platform, then assemble Crush and runtime resources under build/bin/resources |
| prepare-resources | provision resources/bin: fetch or reuse officecli.exe, an embedded Python 3.12.8 runtime and uv, then install `ortools` and `openpyxl`. Reuses a companion Stack checkout via `-StackRoot`, `$env:GOTACK_STACK_ROOT`, or `C:\stack` when present |
| update-crush | refresh third_party/crush to a pinned upstream commit and verify the REST/SSE route markers Gotack relies on |
| check-repository-invariants.mjs | enforce the 1000-line implementation-file limit and the single-owner `.crush-pin` format for the Crush pin |

The release workflow runs `prepare-resources`, so packaged builds include
OfficeCLI, bundled skills and the optional Python libraries available to coding
agents. OfficeCLI is exposed through `PATH`; no Office MCP server is built or
registered.
