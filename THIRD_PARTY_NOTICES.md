# Third-party components

The Apache-2.0 license in this repository applies to the Gotack desktop host.
It does not replace the licenses of the separately built engine or dependencies.

## Agent engine

Gotack communicates over local IPC with [Dyu-36/tack-engine](https://github.com/Dyu-36/tack-engine),
pinned to the exact commit in `.tack-pin`. That repository derives from
[Charmbracelet Crush](https://github.com/charmbracelet/crush).
The current pinned engine's `LICENSE.md` identifies FSL-1.1-MIT and credits
Charmbracelet, Inc. The product build copies that exact file into
`licenses/tack-engine-LICENSE`; the engine's own license remains authoritative.

## Go, frontend and timetable dependencies

`go.mod` / `go.sum`, `pnpm-lock.yaml` and `scripts/timetable-requirements.lock`
identify the dependencies used by the host, frontend and timetable runtime.
Their upstream license and attribution files remain applicable.
The product build includes Python runtime notices, and release automation
produces an SBOM of the complete distribution.

## Pi model catalog

Gotack fetches public provider/model metadata from [Pi.dev](https://pi.dev/api/models)
and bundles an offline snapshot in `internal/modelcatalog/snapshot.json`.
The [Pi repository](https://github.com/earendil-works/pi) is MIT-licensed,
Copyright (c) 2025 Mario Zechner. The full upstream license accompanies the
snapshot in `internal/modelcatalog/LICENSE.pi` and packaged releases include it
as `licenses/pi-LICENSE`. Snapshot provenance is recorded in
`internal/modelcatalog/README.md`.

OpenCode and Pi are also architectural references. Gotack is an independent project.
