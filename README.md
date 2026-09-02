# Tack

**A lightweight desktop AI assistant built for speed, low overhead, and real work.**

Tack is a desktop AI assistant designed to help with files, documents, development, system tasks, research, and everyday automation without the footprint of a heavyweight desktop runtime.

Built with **Go**, **Wails**, and **Svelte**, Tack uses the operating system's native WebView and keeps its desktop layer deliberately lean. The result is an assistant that starts quickly, stays responsive, and remains practical even on lower-resource machines.

> **Fast. Lightweight. Local-first. Built to assist.**

## Why Tack?

Desktop AI tools should not need to consume a large portion of your system just to stay open.

Tack is built around a few core principles:

- **Lightweight by design** — no bundled Chromium or Electron runtime.
- **Built in Go** — efficient process management, native system integration, and strong concurrency primitives.
- **Fast startup** — minimal desktop overhead with heavyweight features loaded only when needed.
- **Low memory usage** — designed to coexist comfortably with browsers, IDEs, terminals, Office applications, and development tools.
- **Local-first** — files, tools, sessions, skills, and assistant context stay close to your machine.
- **More than chat** — Tack is an assistant that can work with your environment, not just answer questions.

## What Tack Can Do

### General Assistant

Use Tack for day-to-day work such as:

- working with local files and folders;
- researching and organizing information;
- automating repetitive tasks;
- inspecting and transforming data;
- assisting with system operations;
- creating and modifying documents;
- helping with software development;
- executing multi-step workflows with local tools.

### Office Work

Tack includes integrated tooling for common Office formats:

- **Word** — `.docx`
- **Excel** — `.xlsx`
- **PowerPoint** — `.pptx`

The assistant can inspect, create, and edit Office documents directly without requiring a separate Office automation setup.

### Developer Workflows

Tack is designed to fit naturally into development workflows. It can work with:

- source code and repositories;
- shell commands;
- project files;
- diffs and changed files;
- development tools;
- terminals;
- workspace-aware sessions.

The optional terminal is lazy-loaded so it does not add unnecessary cost to the default application footprint.

### Memory, Skills & Recall

Tack includes local assistant infrastructure for longer-running workflows:

- **Memory** stores bounded assistant context.
- **Skills** provide reusable procedural knowledge and task-specific workflows.
- **Recall** enables read-only retrieval from previous sessions.

These capabilities are intentionally modular so long-term context remains controlled and predictable.

### Remote Access with Zalo

Tack can connect to an official Zalo Bot, allowing explicitly paired chats to interact with the desktop assistant remotely.

Each paired chat receives its own reusable assistant session. Pairing is explicit and revocable, so remote access remains under user control.

## Lightweight Architecture

Tack deliberately avoids the architecture used by many heavyweight desktop applications.

```text
┌────────────────────────────────────────────┐
│                    Tack                    │
│                                            │
│  Svelte + TypeScript                       │
│  ├── Chat                                  │
│  ├── Sessions                              │
│  ├── Workspaces                            │
│  ├── Files & diffs                         │
│  ├── Tool activity                         │
│  ├── Permissions                           │
│  └── Terminal                              │
│                    │                       │
│              Wails bridge                  │
│                    │                       │
│                 Go host                    │
└────────────────────┬───────────────────────┘
                     │
              Local IPC / REST / SSE
                     │
                     ▼
┌────────────────────────────────────────────┐
│             Local Agent Runtime            │
│                                            │
│  ├── Assistant sessions                    │
│  ├── Tools                                 │
│  ├── Permissions                           │
│  ├── Shell                                 │
│  ├── MCP services                          │
│  ├── Memory                                │
│  ├── Skills                                │
│  └── Recall                                │
└────────────────────────────────────────────┘
```

The UI and assistant runtime are isolated from each other so UI lifecycle and active assistant work do not need to be tightly coupled.

## Why It Stays Lightweight

Resource usage is a product constraint in Tack, not an afterthought.

Tack keeps overhead low by:

- using the **system WebView** instead of shipping a full browser engine;
- keeping the desktop host in **Go**;
- lazy-loading terminal functionality;
- avoiding heavyweight editor runtimes unless they are actually required;
- avoiding duplicated long-running state between UI and backend;
- using event streams instead of unnecessary polling;
- keeping optional capabilities modular.

The goal is not to turn Tack into another full IDE. It is to provide a capable assistant that can comfortably live beside the tools you already use.

## Technology

| Layer | Technology |
| --- | --- |
| Desktop host | Go |
| Desktop framework | Wails v2 |
| UI | Svelte 5 |
| Language | TypeScript |
| Styling | Tailwind CSS |
| Build tooling | Vite |
| Desktop runtime | System WebView / WebView2 |
| Terminal | xterm.js |
| Local storage | SQLite |
| Communication | Local IPC, REST & SSE |

## Security Model

Tack is designed as a single-user local desktop assistant with explicit tool control.

Permission prompts are enabled by default. A dedicated guard layer protects sensitive operations and blocks catastrophic commands. Automatic approval must be explicitly enabled rather than silently assumed.

Remote Zalo access also requires explicit pairing and can be revoked per chat.

> Powerful local tools should remain under user control.

## Installation

### Windows

Tack is currently distributed as a **portable Windows x64 ZIP**.

Download the latest release, extract it, and run:

```text
tack.exe
```

The release bundle contains the required Tack runtime and supporting tools.

You do **not** need to install Go, Node.js, pnpm, or Wails to run a release build.

### Requirements

- Windows 10 or Windows 11, x64
- Microsoft Edge WebView2 Runtime

WebView2 is already present on most maintained Windows installations.

## Building from Source

Requirements:

```text
Go 1.27+
Node.js 24+
pnpm 11+
Wails v2
```

Install frontend dependencies:

```bash
pnpm --dir frontend install
```

Run the development application:

```bash
wails dev
```

Build Tack:

```bash
wails build
```

## Project Structure

```text
.
├── cmd/                    # Bundled local services and tools
│   ├── guard/
│   ├── memory/
│   ├── office/
│   ├── recall/
│   └── skills/
├── internal/               # Go application implementation
├── frontend/               # Svelte desktop UI
├── resources/              # Bundled skills, context and runtime assets
├── scripts/                # Build and repository tooling
├── docs/                   # Architecture and engineering documentation
└── .github/workflows/      # CI and release pipelines
```

## Project Status

Tack is under active development.

The desktop assistant, Office integration, local assistant services, terminal workflow, and Zalo connection are implemented. CI validates the Go and frontend codebases through formatting, static analysis, unit tests, type checking, frontend tests, production builds, repository invariants, and Windows application builds.

Packaged releases currently target **Windows x64**.

## Design Philosophy

Tack is intentionally not trying to become a browser, an IDE, or an operating system inside an application.

It focuses on the layer between **you** and **your computer**:

```text
You
 ↓
Tack
 ↓
Files · Documents · Code · Terminal · Tools · Automation
```

The assistant should be available when you need it and stay out of the way when you do not.

That means keeping Tack:

**small enough to leave running,  
fast enough to open without thinking,  
and capable enough to actually get work done.**

## Contributing

Issues, bug reports, and contributions are welcome.

When making architectural changes, preserve Tack's core priorities: **performance, low resource usage, modularity, security, and a focused assistant experience.**

## License

See the repository license for details.
