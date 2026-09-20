# Gotack

Gotack is a production Windows desktop coding agent written in Go with a Svelte/Wails UI. Its interaction model follows Pi: a small explicit tool set, configurable system resources and skills, persistent sessions with branching/compaction, project trust for project-provided resources, and direct model/provider access. Gotack adds a native Zalo remote channel and ships one built-in timetable skill.

## Product guarantees

- **Full local agent capability by default.** The six core tools are enabled unless the user explicitly disables one: `read`, `powershell`, `edit`, `write`, `grep`, and `glob`. Gotack does not insert per-command permission prompts.
- **Project Trust is not a permission system.** It only decides whether project-local dynamic resources such as `.pi/SYSTEM.md`, `.pi/APPEND_SYSTEM.md`, and `.agents/skills` may be loaded. Tool and OS privileges are unchanged.
- **Prompt/tool parity.** The engine constructs the system prompt from the same enabled tool registry that is exposed to the model.
- **Reproducible engine.** `.tack-pin` pins an exact `Dyu-36/tack-engine` commit. The desktop rejects an unexpected bundled engine revision.
- **No stored-secret reveal API.** Provider credentials can be replaced or removed but are not returned to the WebView as plaintext.
- **Single built-in skill.** Only `resources/skills/timetable` is bundled. User skills remain supported from the Gotack skills directory; trusted projects may additionally provide `.agents/skills`.

## Architecture

```text
Svelte UI
   |
Wails bindings
   |
Go desktop host
   |  local named pipe / Unix socket
   v
tack-engine (exact commit in .tack-pin)
   |
model providers + 6 core tools + skills
```

The host owns desktop lifecycle, workspace selection, attachments, generated files, settings, provider UX, Zalo, bundled resources and release packaging. The engine owns the agent loop, prompt construction, tool execution, provider calls, messages, sessions, summarization and persistence.

## Agent behavior

### Core tools

The default enabled tools are:

- `read`
- `powershell`
- `edit`
- `write`
- `grep`
- `glob`

Settings > Agent can disable tools globally. An empty disabled list means full capability.

### System resources

The engine supports Pi-style prompt resources:

- global `SYSTEM.md`
- global `APPEND_SYSTEM.md`
- project `.pi/SYSTEM.md`
- project `.pi/APPEND_SYSTEM.md`
- project `.tack/SYSTEM.md`
- project `.tack/APPEND_SYSTEM.md`

Project-local system resources are loaded only after Project Trust approves the workspace. Global resources remain available regardless of project trust.

### Skills

Gotack uses progressive skill discovery. The desktop registers:

1. the bundled timetable skill,
2. the user skills directory,
3. trusted project `.agents/skills`.

The bundled timetable skill includes the canonical workbook templates:

- `mau-thoi-khoa-bieu.xlsx`
- `phan-cong-chuan-hoa.xlsx`

Windows production packages also include an offline Python/OR-Tools runtime used by the timetable workflow. Packaging fails unless CP-SAT and both Excel templates pass runtime validation.

### Sessions

Sessions are persisted by the engine and support:

- create / rename / delete / switch,
- clone complete transcript,
- fork from a persisted user message,
- parent-child relationships rendered as a tree,
- manual context compaction,
- automatic engine summarization,
- cancel and live streaming.

## Project Trust

When a workspace contains protected dynamic resources and has no trust decision, Gotack asks before loading them. Protected project resources include Pi/Tack system prompt files, Pi resource directories and project skills.

Choosing **Open untrusted** keeps normal full tool execution but excludes project-provided dynamic resources. Choosing **Trust project** loads them. Trust decisions are persisted locally in `project-trust.json` and may inherit from a trusted or denied parent directory.

## Providers

Provider and model discovery comes from the pinned engine. Gotack additionally manages the desktop integration for:

- ChatGPT/Codex OAuth,
- OpenAI-compatible API keys/endpoints,
- Mistral bootstrap,
- model reasoning level,
- vision capability,
- provider usage where supported.

Credentials are stored in the engine configuration. The desktop does not expose a method to read a previously stored API key back into JavaScript.

## Zalo

Zalo is an optional remote channel into the same agent runtime. A paired Zalo chat therefore has the same enabled tools and model as the desktop session.

Production safeguards include:

- explicit enable/disable state,
- secure random pairing codes with expiration,
- pairing attempt throttling,
- paired-chat allow-list,
- token redaction from errors,
- token/channel state stored outside the frontend,
- SSRF validation for downloaded attachments,
- download size limits and redirect validation,
- controlled outbound file sharing,
- stop/new/status/model commands,
- clean background lifecycle while the window is hidden.

Closing the window hides Gotack to the tray. **Quit Gotack** shuts down the channel, transport and owned engine.

## Data locations

On Windows, Gotack uses the user's application-data directory under `gotack` for host configuration, engine state, skills, project trust and Zalo channel state. Logs are stored under the user's cache directory.

The engine is launched with isolated Crush-compatible config/data/cache environment paths owned by Gotack.

## Development

Requirements:

- Go version from `go.mod`
- Node.js 24
- pnpm 11.20.0
- Wails 2.15.0

Desktop verification:

```powershell
go test ./...
go vet ./...
pnpm --dir frontend install --frozen-lockfile
pnpm --dir frontend check
pnpm --dir frontend test
pnpm --dir frontend build
```

The normal CI additionally runs staticcheck, dead-code analysis, gofmt verification and generated-event consistency checks.

## Complete product build

A production Windows x64 package must be built through:

```powershell
./scripts/build-product.ps1 -EngineSource <path-to-exact-pinned-tack-engine>
```

The build pipeline:

1. verifies desktop tests/vet/frontend,
2. verifies and builds the exact pinned engine,
3. builds the Wails desktop,
4. builds and validates the offline timetable runtime,
5. runs a real host-engine IPC contract test,
6. packages `gotack.exe`, `tack-engine.exe`, Python runtime and licenses,
7. writes a product manifest and SHA-256 checksums.

GitHub release automation additionally produces an SBOM and build attestations.

## Source layout

```text
.
├── frontend/                 Svelte desktop UI
├── internal/
│   ├── appconfig/            desktop configuration
│   ├── attachments/          attachment ingestion/extraction
│   ├── engine/               sidecar lifecycle and revision checks
│   ├── engineapi/            local engine protocol client
│   ├── projecttrust/         persisted Project Trust decisions
│   ├── provider/             provider integration
│   ├── session/              desktop session service
│   ├── workspace/            workspace service
│   ├── workspaceconfig/      skills/runtime workspace registration
│   └── zalo/                 Zalo channel
├── resources/
│   └── skills/timetable/     the only bundled skill
├── scripts/
│   ├── build-engine.ps1
│   ├── build-product.ps1
│   └── build-timetable-runtime.py
└── .tack-pin                 exact engine commit
```

## Production validation

Pull requests and main pushes run both host CI and product integration. Product integration checks the exact pinned engine on Linux, exercises real IPC, and builds the complete Windows distribution. A release is considered valid only when these gates pass for the exact host and engine revisions distributed together.
