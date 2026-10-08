<!-- SPDX-License-Identifier: Apache-2.0 -->
# Tack Engine

<img src="internal/oauth/callback/gotack.png" alt="Gotack" width="96" />

The agent runtime for [Gotack](https://github.com/Dyu-36/gotack), a desktop coding
agent built with Go, Wails and Svelte. Tack Engine runs as a separate local
process and provides the API used by the desktop application.

The engine handles model requests, agent execution, tools, workspace state and
persistent sessions. Gotack provides the desktop interface, application
lifecycle, provider/model catalog and desktop integrations.

## Architecture

```text
Gotack desktop UI → desktop host → local IPC → Tack Engine
```

The desktop host pins an exact engine revision in its `.tack-pin` file. The two
repositories build independently and communicate over a Windows named pipe or
Unix socket. TCP is also available for development.

The engine supports streaming agent events, session branching and compaction,
provider configuration and OAuth, tool permissions, MCP, LSP, hooks and skills.
See [architecture](docs/architecture.md) for the package boundaries.

## Build and run

Use Git and the Go toolchain specified in [go.mod](go.mod). Go's automatic
toolchain selection can obtain the required version when `GOTOOLCHAIN=auto`.

```powershell
git clone https://github.com/Dyu-36/tack-engine.git
cd tack-engine
go build -mod=readonly -trimpath -o tack-engine.exe .
./tack-engine.exe server
```

On Linux or macOS, build with `-o tack-engine` and run `./tack-engine server`.
Use `server --help` to inspect the available flags. The default transport is
local IPC. For development, an explicit loopback TCP endpoint can be used:

```powershell
./tack-engine.exe server --host tcp://127.0.0.1:8080 --debug
```

The engine is a server process; conversations are started through Gotack or the
API. The API uses `/v1`, with OpenAPI documentation at `/v1/docs/` on a TCP server.
Tools execute with the permissions of the user running the engine; keep TCP
listeners on trusted local interfaces.

## Develop and integrate

```powershell
go test -tags gotacktest -mod=readonly ./...
go vet -tags gotacktest -copylocks=false -mod=readonly ./...
```

The `gotacktest` tag enables the cross-package test harness without shipping it
in the engine binary. Gotack's product build additionally checks the engine
revision, source digest and IPC contract.

[Development guide](docs/development.md) · [Contributing](CONTRIBUTING.md)

## License

Gotack's desktop host is Apache-2.0 licensed. This engine contains inherited
source under FSL-1.1-MIT, with retained MIT notices. Its new documentation listed
in [LICENSE.md](LICENSE.md) is licensed under Apache-2.0.

The engine as a whole is **not offered under Apache-2.0 alone**. Rebranding and
repository history changes do not replace the terms of inherited source.
See [LICENSE.md](LICENSE.md), [NOTICE](NOTICE) and
[third-party notices](THIRD_PARTY_NOTICES.md) for the applicable terms.
