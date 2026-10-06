# Gotack

<img src="frontend/public/tack.png" alt="Gotack" width="96" />

A Windows desktop coding agent built with Go, Wails and Svelte. Gotack provides
persistent conversations, streaming tool output, a bundled timetable skill and
an optional Zalo remote channel. Its engine is built separately from an exact
commit pinned in `.tack-pin`.

## Architecture

```text
Svelte UI → Wails main.App → Go desktop host → local IPC → tack-engine
```

The host owns the desktop experience and integrations. The engine owns model
calls, agent execution, tools and persistence. See [architecture](docs/architecture.md)
for package boundaries and the Go/Wails layout.

The host updates its public provider/model catalog from [Pi.dev](https://pi.dev/api/models).
It caches the catalog for offline use and synchronizes supported models into the
engine, so catalog updates do not require an application release.

## Build and develop

Use Windows x64, PowerShell 7, the Go version in `go.mod`, Node.js 24,
pnpm 11.20.0 and Wails v2.15.0.

```powershell
pnpm --dir frontend install --frozen-lockfile
wails generate module
go test -mod=readonly ./...
pnpm --dir frontend check
pnpm --dir frontend test
pnpm --dir frontend build
```

For the complete portable app, obtain the pinned engine checkout and run:

```powershell
./scripts/build-product.ps1 -EngineSource <path-to-pinned-tack-engine>
```

The product build validates the host, engine, real IPC and offline timetable
runtime, then writes the distribution to `artifacts/`.
[Development guide](docs/development.md) · [Contributing](CONTRIBUTING.md)

## Usage

Configure a provider, select a workspace and start a conversation. Sessions
support cloning, branching and compaction. Project Trust controls loading
project-provided prompts and skills. Enabled tools run with your user privileges.
Closing the Windows window hides the app to the tray; use **Quit Gotack** to stop it.

[User guide](docs/user-guide.md) · [Security policy](SECURITY.md)

## License

Gotack desktop host source is licensed under [Apache-2.0](LICENSE).
The separately pinned engine and dependencies retain their own licenses;
see [third-party notices](THIRD_PARTY_NOTICES.md).
