# Gotack Approvals Contract

Gotack enforces its approval posture through Crush's single hook event,
`PreToolUse`, implemented by the `cmd/guard` executable and decided by the
`internal/guard` package. This is an external boundary: Crush spawns `guard`
before every tool call, and the two processes communicate over a pinned JSON
wire contract. The authority is decision 0002 (approval posture) and decision
0003 (memory write protection).

## Hook wire contract

The wire shapes are pinned against the Crush commit recorded in `.crush-pin`.
They mirror `internal/hooks` in the vendored engine (`Payload` on stdin, the
`parseStdout` envelope on stdout). Gotack does not import those packages; the
shapes are redeclared in `internal/guard/wire.go`.

### Input (stdin JSON)

```json
{
  "event": "PreToolUse",
  "session_id": "<crush session id>",
  "cwd": "<session working directory>",
  "tool_name": "<tool>",
  "tool_input": { "command": "...", "file_path": "..." }
}
```

`guard` reads only `tool_input.command` and `tool_input.file_path`; both are
also provided by Crush as `CRUSH_TOOL_INPUT_COMMAND` / `CRUSH_TOOL_INPUT_FILE_PATH`
environment variables. A malformed payload is treated as empty (fail-open), not
an error, because a crashing hook would surface as a non-blocking error and let
the call through.

### Output (stdout JSON)

```json
{ "version": 1, "decision": "allow" | "deny", "halt": false, "reason": "..." }
```

- `decision: "deny"` blocks the tool call. `reason` is shown to the user and,
  per outcome 4, always names the rule that fired.
- `halt: true` additionally stops the whole turn; it is reserved for the
  unrecoverable rules (recursive root delete, disk format/wipe).
- `decision: "allow"` pre-approves the call (skips any interactive prompt).
  The auto tier uses it for safe operations.
- **No opinion** is expressed by writing nothing to stdout and exiting 0. Crush
  then applies its own permission system unchanged; with prompts enabled this
  is the ask tier (the request reaches the UI through `internal/permission`).

`guard` always exits 0 on a decided or pass-through result. A failure to read
stdin exits 1, which Crush treats as a non-blocking hook error (the call is not
blocked). `guard` never prompts and never blocks waiting for input. Every deny
is additionally written to stderr so refusals are logged even when nobody
watches the UI.

## Graduated tiers

`guard` receives per-session options derived by `cmd/guard` from the hook
payload and the Gotack config directory:

- **Write-safe root** = the session's working directory (`cwd` in the hook
  payload), i.e. the workspace path.
- **Context dir** = `<appconfig.Dir()>/context`, the seeded context and memory
  directory owned by the context/memory subsystems.
- **Unattended flag** = membership of the session id in the unattended roster
  (below).

The decision order is fixed (`Evaluate` in `internal/guard/policy.go`):

| Tier | Applies to | Decision |
| --- | --- | --- |
| deny (floor) | any `tool_input.command` matching the blocklist | deny, reason names the rule; never overridable in any posture |
| deny (write boundary) | `write`/`edit`/`multiedit` into `<appconfig.Dir()>/context` (`memory-context-write`) or outside the write-safe root (`write-outside-safe-root`) | deny, reason names the rule |
| auto | read-only tools (`ls`, `glob`, `grep`, `view`, `sourcegraph`) anywhere; writes inside the write-safe root | allow (pre-approved, no prompt) |
| ask | shell commands, network fetches (`download`, `fetch`, `agentic_fetch`), delegation (`agent`), unknown tools | interactive: no opinion → Crush asks the UI; unattended: deny |

The context-dir check runs before the root check because the default assistant
workspace is the drive root, which contains the config directory; the root
check alone would leave the memory cap-bypass hole open (decision 0003).
Unknown tools are never auto-approved: they land in the ask tier.

## Unattended posture (Zalo and scheduled sessions)

An unattended session has no human to answer prompts, so an `ask` there can
never be answered. The posture therefore fails such calls instead of letting
them hang:

- The host records the session id of every Zalo-originated turn in the
  **unattended roster** before the prompt runs (`startZaloTurn` →
  `guard.MarkUnattendedSession`). The scheduler uses the same seam.
  A failed mark fails the turn: running it unmarked could hang on a prompt.
- Roster file: `<appconfig.Dir()>/unattended-sessions.json`, shape
  `{"sessions": ["<session id>", ...]}`, capped at 500 entries (oldest
  trimmed). The spawned guard reads it on every call; a missing or malformed
  roster fails open to the interactive posture (the host re-marks on every
  remote turn).
- In the unattended posture, ask-tier operations are denied with rule
  `unattended-approval` and a legible reason naming the tool; reads and
  writes inside the safe root still succeed. `guard` never blocks waiting for
  input, so an unattended run either proceeds or fails — never hangs.

## Permissions-skip reconciliation

`activateWorkspace` and `activateAssistantWorkspace` call
`SetPermissionsSkip` with `permissionsSkip()`. A missing config, a missing
`auto_approve` key, and `"auto_approve": false` all keep Crush's interactive
permission prompts enabled. Only explicit `"auto_approve": true` enables the
auto posture (equivalent to `--yolo` / `--dangerously-skip-permissions`), as
the escape hatch authorized by decision 0002. The guard's deny rules still
apply in that posture because the hook decides before Crush's permission
system runs.

## Destructive-command blocklist

`guard` hard-denies any `tool_input.command` matching the blocklist and passes
every other tool call through untouched. The blocklist is built in
`internal/guard/blocklist.go`; every rule carries a stable name that appears in
the refusal reason.

| Rule name | Denies | Halt |
| --- | --- | --- |
| `recursive-force-delete` | recursive force delete of `/`, `/*`, `~`, `$HOME`, `--no-preserve-root`, drive-root `rd/rmdir/del/erase /s /q`, `Remove-Item <drive>: -Recurse -Force` | yes |
| `disk-format-wipe` | `format <drive>:`, `mkfs*`, `diskpart`, `dd ... of=/dev/*`, `shred`, `wipefs`, `sgdisk --zap-all` | yes |
| `mass-permission-change` | `chmod/chown` 777/000 on `/`, recursive `icacls <drive>:` `/t`, `takeown /r` | no |
| `shutdown-reboot` | `shutdown`, `reboot`, `poweroff`, `halt`, `init 0/6`, `Restart-Computer`, `Stop-Computer` | no |
| `history-credential-destruction` | `history -c/--clear`, `Clear-History`, deleting shell-history files, deleting `~/.ssh` or `~/.gnupg`, `git filter-branch`, `cmdkey /delete:*` | no |
| `credential-exfiltration` | `curl/wget` uploading credential material, piping `id_rsa`/`.env`/`.netrc`/`.aws/credentials`/`.ssh/` into `curl/wget/nc/Invoke-WebRequest/Invoke-RestMethod` | no |

The patterns are high precision, not high recall: a false deny is disruptive,
so each rule targets the canonical catastrophic form. Ordinary development
commands (`rm -rf node_modules`, `git reset --hard`, `go build`, …) are not
matched. The allow/deny matrix is pinned by `internal/guard/blocklist_test.go`.

## Registration and removal

The host registers the hook when a workspace attaches (`registerGuardHook`,
called from `rebindWorkspaceRuntime` in `bind_workspace.go`).

- Config key written: `hooks.PreToolUse` (workspace scope), an array of Crush
  `HookConfig` objects. The gotack entry is:
  `{ "name": "gotack-guard", "matcher": "", "command": "<abs path to guard>", "timeout": 10 }`.
  An empty `matcher` matches every tool.
- The registered `command` is an **absolute path** (`resolveGuardCommand`):
  bundled `resources/guard(.exe)` next to the desktop executable, else PATH.
  Crush resolves a relative command against the session working directory, so
  the path must be absolute.
- **Merge, do not clobber.** The host reads the current `hooks.PreToolUse` list
  (via `GetWorkspaceConfig`), strips any prior `gotack-guard` entry, and writes
  the rest back with the new entry appended, so user-defined hooks on the same
  event survive.
- **No dangling config.** When the guard binary cannot be resolved, the
  `gotack-guard` entry is removed (rewriting the remaining user hooks, or
  removing the whole key when none remain). A missing binary never leaves a hook
  that errors on every tool call. This mirrors the `office_seed.go` precedent.

## Removing everything

To detach the approval posture entirely, remove the hook key for a workspace:
`RemoveConfigField(workspaceID, ConfigScopeWorkspace, "hooks.PreToolUse")`
(or just the `gotack-guard` entry to keep user hooks). To restore prompts-less
behaviour without detaching the floor, set `"auto_approve": true` in
`config.json`. The unattended roster (`unattended-sessions.json`) can be
deleted freely; it is recreated on the next remote turn. No other host state
requires migration when the hook is removed.
