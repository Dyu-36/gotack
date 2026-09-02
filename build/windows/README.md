# Tack for Windows

Tack is distributed as a **portable ZIP**. An installer is intentionally deferred until the portable build is stable on clean machines.

Artifact layout:

```text
tack-windows-amd64/
  tack.exe
  resources/
    tack-engine.exe
  README.txt
```

Runtime requirements:

- Windows 10/11 x64.
- Microsoft Edge WebView2 Runtime. Current Windows 11 and maintained Windows 10 installations normally already include it; clean/offline images may need the Evergreen runtime installed first.
- No system Go, Node.js, pnpm, or Wails installation is required for the release ZIP. Tack ships the runtime components it needs inside the release bundle.

Release artifacts are built by GitHub Actions from the repository's pinned runtime sources and bundled components to keep builds reproducible.
