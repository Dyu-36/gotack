# Gotack for Windows

MVP distribution is a **portable ZIP**. An installer is intentionally deferred until the portable build is stable on clean machines.

Artifact layout:

```text
gotack-windows-amd64/
  tack.exe
  resources/
    tack-engine.exe
  README.txt
```

Runtime requirements:

- Windows 10/11 x64.
- Microsoft Edge WebView2 Runtime. Current Windows 11 and maintained Windows 10 installations normally already include it; clean/offline images may need the Evergreen runtime installed first.
- No system Go, Node.js, pnpm, Wails or Crush installation is required for the release ZIP. Gotack prefers the bundled `resources/tack-engine.exe` and falls back to `tack-engine` or `crush` on PATH when the bundle is absent.

The release artifact must be built by GitHub Actions from the pinned Crush commit recorded in `third_party/README.md`; do not copy an arbitrary local Crush executable into a release.
