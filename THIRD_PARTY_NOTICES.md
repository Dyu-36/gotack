# Third-party components

The Apache-2.0 license in this repository applies to the Gotack desktop host.
It does not replace the licenses of the imported engine or dependencies.

## Agent engine

Gotack imports the engine under `internal/agentcore` from historical
Dyu-36/tack-engine revision `00205e5f73fa9bac13c46e055a7a076b3bb51d50`.
Its Git history is preserved in this repository. It contains a derivative of code
originally published by Charmbracelet.
The engine's `internal/agentcore/LICENSE.md` preserves FSL-1.1-MIT and MIT terms on inherited source,
and scopes Apache-2.0 to its independently authored Gotack documentation and
artwork. The engine as a whole is not offered under Apache-2.0 alone. The product
build copies its complete license into `licenses/tack-engine-LICENSE` and includes
its `NOTICE` and `THIRD_PARTY_NOTICES.md` when present; the engine's own terms
remain authoritative.

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
