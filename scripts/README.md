# scripts

Developer entry points. Keep them thin wrappers around the Wails and pnpm CLIs.

| Script | Role |
| --- | --- |
| dev | start the engine and run wails dev with the Vite watcher |
| build | pnpm build, then wails build for the current platform |
| update-crush | refresh third_party/crush to a pinned upstream commit |
