# Gotack user guide

## Agent and providers

The default tools are `read`, `powershell`, `edit`, `write`, `grep` and `glob`.
Settings > Agent can disable tools globally; an empty disabled list means full
capability. The engine constructs the system prompt from the same enabled tool
registry exposed to the model.

Providers and chat models come from Pi.dev and refresh every five minutes while
the app is connected and visible. A local cache and bundled snapshot keep the
catalog available offline. Models using protocols the engine supports appear
in Settings; the chat model picker shows providers with usable credentials.
ChatGPT/Codex uses the account's own OAuth model list. Custom providers and
endpoints remain supported. Credentials can be replaced or removed; previously
stored keys are not returned to JavaScript as plaintext.

## Sessions

Sessions support create, rename, delete, switch, full-transcript cloning, forking
from a persisted user message, parent-child trees, manual compaction, automatic
summarization, cancellation and live streaming. The engine owns persistence.

## System resources, skills and Project Trust

Global `SYSTEM.md` and `APPEND_SYSTEM.md` are supported. Projects may provide
those files under `.pi/` or `.tack/`, plus skills under `.agents/skills/`.
Project-provided dynamic resources load only after Project Trust approves the
workspace. Global resources remain available regardless of project trust.

**Open untrusted** excludes project dynamic resources while keeping enabled tool
execution. **Trust project** loads those resources. Decisions persist in
`project-trust.json` and may inherit from trusted or denied parent directories.
Project Trust does not change tool or OS privileges.

Gotack progressively discovers its bundled timetable skill, user skills and
trusted project skills. Only `resources/skills/timetable` is bundled, with
`mau-thoi-khoa-bieu.xlsx` and `phan-cong-chuan-hoa.xlsx`. The Windows product also
ships an offline Python/OR-Tools runtime for workbook generation and scheduling.

## Zalo and desktop lifecycle

Zalo is an optional remote channel into the same runtime, with the same enabled
tools and model. Enable it in Settings, save a bot token and pair the intended
chat with the displayed pairing code.

Codes expire, pairing attempts are throttled and only paired chats are allowed.
The integration redacts tokens in errors, keeps channel state outside the
frontend, validates attachment download destinations/redirects and size limits,
and controls outbound file sharing. It supports stop/new/status/model commands
and continues operating while the Windows window is hidden.

Closing the window hides Gotack to the tray. **Quit Gotack** stops the channel,
transport and owned engine. Auto-start can launch it hidden after Windows login.

## Local data

On Windows, host configuration, engine state, skills, trust decisions and Zalo
state live under the user's application-data `gotack` directory. Logs use the
cache directory. The engine receives isolated config/data/cache paths owned by
Gotack. Workspace cleanup does not remove user data.
