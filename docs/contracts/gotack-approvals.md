# Gotack Approvals Contract

Gotack enforces its approval posture through Crush's single hook event,
`PreToolUse`, implemented by the `cmd/guard` executable and decided by the
`internal/guard` package. This is an external boundary: Crush spawns `guard`
before every tool call, and the two processes communicate over a pinned JSON
wire contract. The authority is decision 0002 (approval posture) and decision
0003 (memory write protection); the phased rollout is Phase 4 of
`docs/plans/active/hermes-parity-harness.md`.

The posture ships in two stages. Stage 1 (this document's initial scope) is a
deny-only destructive-command blocklist with no new prompts. Stage 2 extends the
same hook to the graduated allow/ask/deny tiers; those sections are added in the
stage-2 change.

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
- **No opinion** is expressed by writing nothing to stdout and exiting 0. Crush
  then applies its own permission system unchanged. This is the pass-through.

`guard` always exits 0 on a decided or pass-through result. A failure to read
stdin exits 1, which Crush treats as a non-blocking hook error (the call is not
blocked). `guard` never prompts and never blocks waiting for input.

## Stage 1 — destructive-command blocklist

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
(or just the `gotack-guard` entry to keep user hooks). Nothing else in Phases
1–5 modifies Crush state, so retreat requires no migration.
