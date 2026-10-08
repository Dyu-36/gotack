# Gotack

<img src="frontend/public/tack.png" alt="Gotack" width="96" />

A coding agent with an interactive terminal and a Windows desktop built with Go, Wails and Svelte. Gotack provides
persistent conversations, streaming tool output, a bundled timetable skill and
an optional Zalo remote channel. Both interfaces share the engine in `internal/agentcore`,
built from this repository and its single Go module.

## Architecture

```text
Terminal / Svelte UI → shared engine client and supervisor → local IPC → agentcore
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
go test -tags gotacktest -mod=readonly ./...
pnpm --dir frontend check
pnpm --dir frontend test
pnpm --dir frontend build
```

For the complete portable app, run from this checkout:

```powershell
./scripts/build-product.ps1
```

The product build validates the host, engine, real IPC and offline timetable
runtime, then writes the distribution to `artifacts/`.
[Development guide](docs/development.md) · [Contributing](CONTRIBUTING.md)

## Usage

```powershell
go build -o build/bin/gotack.exe ./cmd/gotack
./build/bin/gotack.exe chat --workspace .
./build/bin/gotack.exe run --workspace . "Explain this project"
./build/bin/gotack.exe serve
```

Use `--provider ID --model ID` to select a configured model, or reuse the
workspace's desktop configuration. Use `--session ID` to resume a conversation.
Tool permission prompts offer one-time approval, session approval, or denial;
`--yolo` explicitly enables automatic approval. During a run, `/cancel` stops it;
Ctrl+C cancels and exits. Chat also provides `/new`, `/sessions`, and `/quit`.
The portable distribution contains `gotack.exe` and `gotack-desktop.exe`.


Configure a provider, select a workspace and start a conversation. Sessions
support cloning, branching and compaction. Project Trust controls loading
project-provided prompts and skills. Enabled tools run with your user privileges.
Closing the Windows window hides the app to the tray; use **Quit Gotack** to stop it.

[User guide](docs/user-guide.md) · [Security policy](SECURITY.md)

## License

Gotack desktop host source is licensed under [Apache-2.0](LICENSE).
The imported engine and dependencies retain their own licenses;
see [third-party notices](THIRD_PARTY_NOTICES.md).
