# scripts

Developer entry points. All four are PowerShell and Windows only, so a
non-Windows checkout currently has no scripted build path and must drive
`wails`, `go` and `pnpm` directly.

Keep `build` and `build-office` thin wrappers around the Wails, Go and pnpm
CLIs. `prepare-resources` is deliberately not a thin wrapper: it provisions a
bundled runtime.

| Script | Role |
| --- | --- |
| build | pnpm build, then wails build for the current platform, then assemble Crush/Office runtime resources under build/bin/resources |
| build-office | build cmd/office as a Windows GUI-subsystem binary into build/bin/resources/office.exe |
| prepare-resources | provision resources/bin: fetch or reuse officecli.exe, an embedded Python 3.12.8 runtime and uv, then install `ortools` and `openpyxl`. Reuses a companion Stack checkout via `-StackRoot`, `$env:GOTACK_STACK_ROOT`, or `C:\stack` when present |
| update-crush | refresh third_party/crush to a pinned upstream commit and verify the REST/SSE route markers Gotack relies on |

## Known gaps

- No workflow runs `prepare-resources`, and `release.yml` copies only
  `officecli.exe` and `resources/skills` — not `resources/bin/`. The bundled
  Python timetable runtime is therefore absent from the released ZIP and is
  only available in a locally prepared checkout.
- The pinned Crush commit is duplicated in `third_party/README.md`,
  `.github/workflows/ci.yml` and `.github/workflows/release.yml`. All three
  must be updated in the same change.
- `build-office` and the release workflow build `cmd/office` independently, so
  their flags can drift.
