<!-- SPDX-License-Identifier: Apache-2.0 -->
# Architecture

Tack Engine is the local agent server used by Gotack. It is one Go module with
a root executable and private implementation packages under `internal/`.

```text
Gotack desktop host
    ↓ named pipe / Unix socket / development TCP
main.go → internal/server → internal/backend → internal/app
                                              ↓
                             agent execution, tools and providers
                                              ↓
                                sessions, messages and database
```

## Responsibilities

| Location | Responsibility |
| --- | --- |
| `main.go` | Parse the `server` command, load configuration and manage shutdown |
| `internal/server` | HTTP API, transport listeners and event delivery |
| `internal/backend` | Workspace lifecycle and API operations |
| `internal/app` | Compose workspace services and agent execution |
| `internal/agent` | Model calls, streaming responses, agent loops and tools |
| `internal/config` | Provider, model, workspace and engine configuration |
| `internal/oauth` | Provider authorization and the Gotack callback page |
| `internal/session`, `internal/message`, `internal/history` | Persistent conversation and file state |
| `internal/db` | Database access and migrations |
| `internal/lsp`, `internal/mcp` | Language server and MCP integration |
| `internal/skills`, `internal/hooks`, `internal/permission` | Agent extensions and permission decisions |
| `internal/proto`, `internal/swagger` | API types and OpenAPI documentation |
| `internal/version` | Version and build provenance used by the desktop handshake |

The server is the shipped entrypoint. Some retained implementation packages
contain presentation helpers, but there is no interactive terminal command in
the executable.

## Desktop boundary

The engine owns agent execution and persistence. The Gotack host owns the Wails
bindings, Svelte interface, process lifecycle, public model catalog and desktop
integrations. The host talks to the engine API instead of importing this
repository's `internal` packages.

The host's `.tack-pin` records the exact engine commit. A packaged build validates
the checkout, runs engine checks, embeds build provenance and exercises real IPC.
Changes to the API must remain compatible with the host or update both sides
together.

## Configuration and compatibility

Engine settings use `tack.json` and the `TACK_` environment namespace.
`TACK_ENGINE_GLOBAL_CONFIG` selects the global configuration directory;
`TACK_GLOBAL_DATA` selects the global data directory. `server --data-dir` provides
a custom engine data path. Each workspace also has its own configuration and
session services.

Existing database migrations remain part of the engine, including migrations
needed by earlier installations. Product branding is separate from stored-data
compatibility and third-party attribution.
