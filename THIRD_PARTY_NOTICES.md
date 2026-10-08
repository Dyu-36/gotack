# Third-party notices

## Inherited engine source

Tack Engine contains a derivative of source originally published in
[Charmbracelet/Crush](https://github.com/charmbracelet/crush).
The inherited license text identifies FSL-1.1-MIT, Copyright 2025-2026
Charmbracelet, Inc., and retains an MIT notice for Kujtim Hoxha.
The full terms and original notices are preserved in [LICENSE.md](LICENSE.md).

FSL's future MIT grant applies separately to each version after two years from
its original availability. It is not a present Apache-2.0 grant for all engine
source. Modifications and derivatives remain subject to the applicable
inherited terms. A repository rename or history reset does not change them.

## Gotack documentation

The independently authored documentation listed in `LICENSE.md` is licensed
under Apache-2.0, Copyright 2026 Dyu-36 and Gotack contributors. This grant does
not replace the license on inherited engine implementation or dependencies.

## Dependencies

The Gotack callback artwork is copied from `frontend/public/tack.png` in the
Apache-2.0 licensed Gotack repository. Its license is included in `LICENSE.md`.

`go.mod` and `go.sum` identify the engine's Go dependencies, including provider
SDKs, MCP, database, terminal-formatting and language-server libraries. Those
libraries retain their upstream licenses and notices. A dependency's vendor
name in an import path is not this product's branding.

## Distribution

Engine archives include `LICENSE.md`, `NOTICE` and this document. When bundling
the engine into Gotack, include its complete license text and notices alongside
the desktop host's Apache-2.0 license.
