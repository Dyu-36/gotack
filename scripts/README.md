# scripts

Developer entry points. The build/resource helpers are PowerShell and Windows
only; `check-repository-invariants.mjs` is cross-platform and is the shared
local/CI owner for deterministic repository policy checks.

Keep `build` and `build-office` thin wrappers around the Wails, Go and pnpm
CLIs. `prepare-resources` is deliberately not a thin wrapper: it provisions a
bundled runtime.

| Script | Role |
| --- | --- |
| build | pnpm build, then wails build for the current platform, then assemble Crush/Office runtime resources under build/bin/resources |
| build-office | build cmd/office as a Windows GUI-subsystem binary into build/bin/resources/office.exe |
| prepare-resources | provision resources/bin: fetch or reuse officecli.exe, an embedded Python 3.12.8 runtime and uv, then install `ortools` and `openpyxl`. Reuses a companion Stack checkout via `-StackRoot`, `$env:GOTACK_STACK_ROOT`, or `C:\stack` when present |
| update-crush | refresh third_party/crush to a pinned upstream commit and verify the REST/SSE route markers Gotack relies on |
| check-repository-invariants.mjs | enforce the 1000-line implementation-file limit and the single-owner `.crush-pin` format for the Crush pin |

## Known gaps

- No workflow runs `prepare-resources`, and `release.yml` copies only
  `officecli.exe` and `resources/skills` — not `resources/bin/`. The bundled
  Python timetable runtime is therefore absent from the released ZIP and is
  only available in a locally prepared checkout.
- `build-office` and the release workflow build `cmd/office` independently, so
  their flags can drift.
