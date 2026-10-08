<!-- SPDX-License-Identifier: Apache-2.0 -->
# Contributing to Tack Engine

Tack Engine provides Gotack's local agent runtime. Changes to the desktop UI,
Wails bindings and host integrations belong in
[Gotack](https://github.com/Dyu-36/gotack).

Before submitting an engine change:

1. Explain the behavior and any API or data compatibility implications.
2. Follow the existing package boundaries and format modified source.
3. Run the checks in the [development guide](docs/development.md).
4. Add a regression test when a behavior change needs one.
5. Keep generated API and database files consistent with source changes.

Do not commit provider credentials, local configuration, binaries or test
artifacts. Keep required source attribution and license notices. See
[LICENSE.md](LICENSE.md) for the inherited source terms and the Apache-2.0 scope
of independently authored documentation.
