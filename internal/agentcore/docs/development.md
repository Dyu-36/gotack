<!-- SPDX-License-Identifier: Apache-2.0 -->
# Development

Use the Go version declared in `go.mod`. With `GOTOOLCHAIN=auto`, Go can download
that toolchain automatically. Task is optional; direct Go commands work without
it. PowerShell 7 is recommended for Windows development.

## Validate changes

```powershell
go test -tags gotacktest -mod=readonly ./...
go vet -tags gotacktest -copylocks=false -mod=readonly ./...
go build -mod=readonly ./...
```

The tagged test harness is excluded from production builds. The scoped vet
setting matches Gotack's product build: the schema alias on a generic map uses
a value receiver for schema discovery, so that build disables `copylocks`.

When a C compiler is available, also run the race suite used by Windows CI:

```powershell
go test -race -tags gotacktest -mod=readonly ./...
```

Format Go changes with `go fmt ./...`, which uses the module's selected toolchain.
If invoking `gofmt` directly, use the binary from that toolchain's `GOROOT` rather
than an older system installation. Format the OAuth callback HTML, CSS and JavaScript
with Prettier, retaining template actions on one line inside HTML tags. Commit
source and lockfiles; keep binaries, generated previews and local state out of
Git.

## Run the engine

```powershell
go build -mod=readonly -trimpath -o tack-engine.exe .
./tack-engine.exe server --debug
```

The default listener is a named pipe on Windows and a Unix socket on other
platforms. For API exploration, use `--host tcp://127.0.0.1:8080` and open
`http://127.0.0.1:8080/v1/docs/`. Engine commands are limited to `server` and its
flags.

With Task installed, `task build` produces the `tack-engine` executable,
`task run` starts its server and `task test` enables the tagged race suite.

## Build with Gotack

1. Commit and push the engine change.
2. Update Gotack's `.tack-pin` to that exact commit after validation.
3. Check out the pinned engine revision in a clean working tree.
4. From the Gotack repository, run:

   ```powershell
   ./scripts/build-product.ps1 -EngineSource <path-to-pinned-tack-engine>
   ```

The product build checks the desktop, frontend, engine, IPC contract and bundled
runtime. Its engine build embeds the commit and source digest used by the host
handshake. A plain `go build` is useful for development but does not replace that
packaging step.

## Generated files and releases

`task swag` regenerates the OpenAPI files from the annotations on `main.go` and
the API handlers. `task sqlc` regenerates database bindings with SQLC. Review and
commit generated changes with the corresponding source changes.

The GoReleaser configuration builds engine archives for Windows, Linux and
macOS. Archives include the README, license terms and notices. Use
`goreleaser check` and `goreleaser release --snapshot --clean` for local validation.
Publishing requires an intentionally created engine version tag and a GitHub
token authorized for `Dyu-36/tack-engine`.

The release configuration publishes to this engine repository. Third-party
package registries and vendor-owned taps are not configured.
