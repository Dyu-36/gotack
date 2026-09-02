<div align="center">

# Tack

### A lightweight desktop AI assistant built for speed, low overhead, and real work.

[![CI](https://img.shields.io/github/actions/workflow/status/Dyu-36/gotack/ci.yml?branch=main&style=for-the-badge&label=Build)](https://github.com/Dyu-36/gotack/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/Dyu-36/gotack?style=for-the-badge&sort=semver&label=Release)](https://github.com/Dyu-36/gotack/releases)
[![Go](https://img.shields.io/badge/Go-1.27-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://go.dev/)
[![Wails](https://img.shields.io/badge/Wails-v2.15-DF0000?style=for-the-badge)](https://wails.io/)
[![Svelte](https://img.shields.io/badge/Svelte-5-FF3E00?style=for-the-badge&logo=svelte&logoColor=white)](https://svelte.dev/)
[![Windows](https://img.shields.io/badge/Windows-x64-0078D4?style=for-the-badge&logo=windows11&logoColor=white)](https://github.com/Dyu-36/gotack/releases)
[![License](https://img.shields.io/badge/License-TBD-lightgrey?style=for-the-badge)](#-license)

**Fast · Lightweight · Local-first · Built to assist**

[Download](https://github.com/Dyu-36/gotack/releases) · [Report an issue](https://github.com/Dyu-36/gotack/issues) · [Build from source](#-building-from-source)

</div>

---

<!--
📸 SCREENSHOT / GIF SLOT

Replace the path below once an image is added to the repository, for example:

<p align="center">
  <img src="docs/images/tack-preview.png" alt="Tack desktop assistant" width="960" />
</p>

For an animated demo:

<p align="center">
  <img src="docs/images/tack-demo.gif" alt="Tack demo" width="960" />
</p>
-->

> 📸 **Preview slot:** add a product screenshot or short GIF here when ready. Recommended path: `docs/images/tack-preview.png`.

## ✨ Why Tack?

Tack is a desktop AI assistant designed for files, documents, development, system tasks, research, and everyday automation — without the footprint of a heavyweight desktop runtime.

Built with **Go**, **Wails**, and **Svelte**, Tack uses the operating system's native WebView and keeps the desktop layer deliberately lean. It is designed to start quickly, stay responsive, and remain practical even on lower-resource machines.

| Principle | What it means |
| --- | --- |
| ⚡ **Fast startup** | Minimal desktop overhead and lazy-loaded heavyweight features. |
| 🪶 **Lightweight** | No bundled Chromium or Electron runtime. |
| 🧠 **Assistant-first** | Works with your environment instead of acting as a chat-only UI. |
| 💻 **Built in Go** | Efficient process management, native system integration, and concurrency. |
| 🏠 **Local-first** | Files, tools, sessions, skills, and assistant context stay close to your machine. |
| 🧩 **Modular** | Optional capabilities are kept separate so Tack stays focused and maintainable. |

## 🚀 Features

| Capability | Highlights |
| --- | --- |
| 🤖 **General assistant** | Files, folders, research, system tasks, data transformation, and multi-step automation. |
| 📝 **Office workflows** | Inspect, create, and edit Word (`.docx`), Excel (`.xlsx`), and PowerPoint (`.pptx`) files. |
| 👨‍💻 **Developer workflows** | Source code, repositories, shell commands, diffs, project files, and workspace-aware sessions. |
| 🖥️ **Integrated terminal** | Optional xterm.js terminal, loaded only when needed. |
| 🧠 **Memory** | Bounded assistant context for longer-running workflows. |
| 🧰 **Skills** | Reusable procedural knowledge and task-specific workflows. |
| 🔎 **Recall** | Read-only retrieval from previous sessions. |
| 📱 **Zalo access** | Explicitly paired chats can interact with the desktop assistant remotely. |
| 🛡️ **Permission control** | Tool execution remains user-controlled with guarded sensitive operations. |

## 🪶 Lightweight by Design

Resource usage is a product constraint in Tack, not an afterthought.

| Tack does | Tack avoids |
| --- | --- |
| Uses the **system WebView / WebView2** | Bundling a full Chromium runtime |
| Keeps the desktop host in **Go** | Heavy desktop-process overhead |
| Lazy-loads terminal functionality | Initializing optional components at startup |
| Uses event streams where appropriate | Unnecessary background polling |
| Keeps long-running responsibilities modular | Duplicating state across layers |
| Focuses on assistant workflows | Becoming another heavyweight IDE |

The goal is simple: **small enough to leave running, fast enough to open without thinking, and capable enough to get real work done.**

## 🏗️ Architecture

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

The UI and assistant runtime are isolated so the desktop lifecycle does not need to be tightly coupled to active assistant work.

## 🧱 Technology Stack

| Layer | Technology |
| --- | --- |
| Desktop host | **Go 1.27** |
| Desktop framework | **Wails v2.15** |
| UI | **Svelte 5** |
| Language | **TypeScript** |
| Styling | **Tailwind CSS** |
| Build tooling | **Vite** |
| Desktop runtime | **System WebView / WebView2** |
| Terminal | **xterm.js** |
| Local storage | **SQLite** |
| Communication | **Local IPC, REST & SSE** |
| Current packaged target | **Windows x64** |

## 📦 Installation

### Windows portable release

1. Download the latest ZIP from [GitHub Releases](https://github.com/Dyu-36/gotack/releases).
2. Extract it to a folder of your choice.
3. Launch Tack:

```powershell
.\tack.exe
```

The release bundle includes the Tack runtime and supporting tools. You do **not** need a system installation of Go, Node.js, pnpm, or Wails to run a packaged release.

### Requirements

```text
Windows 10/11 x64
Microsoft Edge WebView2 Runtime
```

WebView2 is already available on most maintained Windows installations.

## 🛠️ Building from Source

### Prerequisites

```text
Go 1.27+
Node.js 24+
pnpm 11+
Wails v2.15+
```

Clone the repository:

```powershell
git clone https://github.com/Dyu-36/gotack.git
cd gotack
```

Install the Wails CLI and frontend dependencies:

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0
pnpm --dir frontend install --frozen-lockfile
```

Run Tack in development mode:

```powershell
wails dev
```

Build a Windows x64 binary:

```powershell
wails build -platform windows/amd64 -clean
```

## 🔐 Security Model

Tack is designed as a single-user local desktop assistant with explicit tool control.

| Protection | Behavior |
| --- | --- |
| 🛡️ Permission prompts | Enabled by default. |
| ⛔ Guard layer | Protects sensitive operations and blocks catastrophic commands. |
| ✅ Auto approval | Must be explicitly enabled; it is never silently assumed. |
| 📱 Zalo pairing | Remote chats require explicit pairing and can be revoked individually. |

> **Powerful local tools should remain under user control.**

## 📂 Project Structure

<details>
<summary><strong>Expand repository layout</strong></summary>

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

</details>

## ✅ Project Status

Tack is under active development.

The desktop assistant, Office integration, local assistant services, terminal workflow, and Zalo connection are implemented. CI covers Go tests and analysis, frontend validation and tests, production builds, repository invariants, and a Windows portable build.

**Current packaged target:** Windows x64.

## 🧭 Design Philosophy

Tack is intentionally not trying to become a browser, an IDE, or an operating system inside an application.

```text
You
 ↓
Tack
 ↓
Files · Documents · Code · Terminal · Tools · Automation
```

The assistant should be available when you need it and stay out of the way when you do not.

## 🤝 Contributing

Issues, bug reports, and contributions are welcome.

When making architectural changes, preserve Tack's core priorities:

**performance · low resource usage · modularity · security · focused assistant experience**

## 📄 License

A project license has **not yet been published** in this repository.

Until a license is added, normal copyright restrictions apply. If you plan to distribute or accept external contributions, add an explicit license first.
