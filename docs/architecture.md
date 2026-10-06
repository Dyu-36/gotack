# Architecture

Gotack is one Go module containing a Windows desktop application and its private
supporting packages. The agent runtime is a separate repository, built from the
exact commit in `.tack-pin`.

Development skill examples under `agent/skills` belong to the separate module
declared in `agent/go.mod`. Application build and test commands do not compile
these reference examples or require their illustrative dependencies.

```text
Svelte UI
   ↓ Wails bindings and events
main.App — desktop runtime adapter, public methods and JSON DTOs
   ↓
internal/desktop — connection snapshots, attach/reconnect and event streams
   ↓
internal/engine + internal/engineapi — process lifecycle and IPC protocol
   ↓ named pipe on Windows / Unix socket on supported development hosts
tack-engine — agent loop, tools, model calls and persisted sessions
```

## Package responsibilities

| Location | Responsibility |
| --- | --- |
| `main.go`, `app.go`, `bind_*.go` | Wails entrypoint, lifecycle adapters, stable frontend contract |
| `desktop_runtime.go` | Product hooks connecting workspace/settings policy to the coordinator |
| `internal/desktop` | Connection snapshots, service assembly, attach/stop/reconnect, event streams |
| `internal/engine` | Sidecar process lifecycle, handshake and exact revision checks |
| `internal/engineapi` | IPC transport, typed requests and engine responses |
| `internal/session`, `internal/workspace` | Desktop services over the engine API |
| `internal/provider` | Desktop provider configuration, credentials and OAuth integration |
| `internal/modelcatalog` | Pi.dev catalog fetching, HTTP revalidation and offline snapshots |
| `internal/attachments`, `internal/office` | File ingestion, extraction and Office parsing |
| `internal/projecttrust`, `internal/workspaceconfig` | Trust decisions and resource registration |
| `internal/zalo` | Optional remote channel, pairing, delivery and lifecycle |
| `frontend/src/features/settings` | Modal shell and Provider, Agent, Zalo, Appearance panels |
| `resources/skills/timetable` | Bundled skill and canonical workbook templates |
| `scripts`, `.github` | Verified product builds and shared CI toolchain setup |

The coordinator accepts event and product-policy hooks; it does not import Wails
or the root `main` package. The protocol client does not import engine source
internals. Feature packages own their behavior independently of the entrypoint.

## Connection lifecycle

Attach and disconnect share a critical section so a cancelled connection cannot
commit half of its lifecycle state. Readers use snapshots; updates replace the
snapshot instead of changing fields on a published value.

Workspace initialization replaces the handshake scope with an event stream scope.
Final readiness checks use attachment identity so that this planned transition
is not mistaken for a superseded connection. Late attachments after close and
attachments superseded by disconnect are rejected.

## Why the entrypoint stays at the root

The official [Go module layout guide](https://go.dev/doc/modules/layout) supports
a root executable with private packages under `internal/`. `cmd/` is a convention,
especially useful for multiple commands or a module exposing libraries as well
as commands. Gotack currently has one primary desktop entrypoint.

The official [Wails layout](https://wails.io/docs/gettingstarted/firstproject/#project-layout)
places `main.go`, `frontend/`, `build/` and `wails.json` at the root. Gotack embeds
frontend assets, its tray icon and the engine pin. Go
[embed patterns](https://pkg.go.dev/embed#hdr-Directives) are relative to the package
directory and cannot traverse `..`; moving those declarations requires a
different asset layout.

The frontend calls `window.go.main.App`. Wails bindings follow package and struct
names, so `App` remains an explicit façade. Internal refactors keep its public
methods and DTO shapes stable; `bindings_contract_test.go` checks that surface.

## Model catalog

The desktop host owns the public model catalog. `internal/modelcatalog` fetches
`https://pi.dev/api/models`, revalidates with ETag every five minutes, and keeps
an atomic disk cache plus a bundled Pi snapshot for offline startup. The UI
refreshes while connected; hidden windows defer polling until visible again.

`internal/provider` maps Pi protocol identifiers to engine-supported transports
and stable Gotack provider IDs. Unsupported protocols and non-chat models are
excluded. `model_routes` carries each model's protocol, endpoint and header
defaults into the engine. Protocol selection therefore follows catalog metadata
even for new model names that the bundled SDK does not recognize. The engine's
provider response supplies existing transport identities and subscription data,
rather than the public catalog's model list.

Before selecting a Pi model, the host synchronizes the provider's models through
the existing batched engine config API. The engine reloads that configuration,
so newly published models can run without rebuilding or restarting it. A local
ownership manifest tracks synchronized models so later updates can replace or
remove catalog entries while retaining custom model edits. Credential values
and custom endpoints remain owned by the engine configuration.

`catalog_models` marks that synchronized list as authoritative so the engine
does not append older bundled catalog entries after each reload.

Codex keeps its account-scoped OAuth catalog from the engine. Public Pi models
are never merged into that subscription catalog. Custom configured providers
also remain available independently of the public Pi catalog.

File count is not an architectural constraint. Bindings may remain split by
capability. Tests stay beside their Go package, and packages are named for their
responsibility rather than generic `util`, `common` or `types` buckets.
See [Go package naming](https://go.dev/blog/package-names).

## References

[OpenCode](https://github.com/anomalyco/opencode/blob/dev/CONTRIBUTING.md#developing-opencode)
separates core/server code, shared UI and desktop integration.
[Pi](https://github.com/earendil-works/pi#packages) separates provider APIs, agent
runtime and the coding-agent application. Gotack follows these responsibility
boundaries using Go/Wails packages and a separately pinned engine.

## Source and generated files

Keep source, tests, lockfiles, `.tack-pin`, skill templates and build icons/manifests.
`build/bin/`, `frontend/dist/`, `frontend/wailsjs/`, `node_modules/` and `artifacts/`
are generated, ignored local outputs. The engine checkout and history backups
are not part of routine generated-output cleanup.
