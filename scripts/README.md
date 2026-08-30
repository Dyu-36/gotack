# scripts

Developer entry points. Keep them thin wrappers around the Wails and pnpm CLIs.

| Script | Role |
| --- | --- |
| build | pnpm build, then wails build for the current platform, then assemble Crush/Office runtime resources under build/bin/resources |
| build-office | build cmd/office as a Windows GUI-subsystem binary into build/bin/resources/office.exe |
| prepare-resources | prepare the bundled Python 3.12 runtime with `ortools` and `openpyxl`, reusing `C:\stack` when available |
| update-crush | refresh third_party/crush to a pinned upstream commit |
