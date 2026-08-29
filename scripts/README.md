# scripts

Developer entry points. Keep them thin wrappers around the Wails and pnpm CLIs.

| Script | Role |
| --- | --- |
| build | pnpm build, then wails build for the current platform |
| build-office | build cmd/office as a Windows GUI-subsystem binary into build/bin/resources/office.exe |
| update-crush | refresh third_party/crush to a pinned upstream commit |
