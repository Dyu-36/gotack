# Contract: gotack-skills MCP server (`cmd/skills`)

Status: implemented by `internal/skillmanage`, `cmd/skills`, and
`skills_seed.go`.

`gotack-skills` is the write boundary for the per-user procedural-skill root.
Crush remains the canonical owner of skill discovery: it injects the catalog
from `options.skills_paths` and its built-in `view` tool reads skill files.
Gotack does not expose a second catalog or list tool. The server exposes two
tools only:

| Tool | Contract |
| --- | --- |
| `skill_view` | Reads one managed file and, for a background review, records an exact read mark in the same MCP process. It is a safety handshake, not a catalog source. |
| `skill_manage` | Applies a non-empty `operations` array atomically, capped at 20 items. |

## `skill_manage` operations

The advertised action enum is exactly:

`create`, `patch`, `delete`, `write_file`, `remove_file`.

| Action | Required fields | Behavior |
| --- | --- | --- |
| `create` | `name`, complete `content`; optional `category` | Publishes a new skill directory. |
| `patch` | `name`, unique `old_string`, `new_string`; optional `file_path` | Replaces one exact occurrence in `SKILL.md` or a support file. An empty `new_string` deletes the occurrence. |
| `delete` | `name` | Removes the complete managed skill tree; it must be the only operation in the call. |
| `write_file` | `name`, `file_path`, `file_content` | Creates or replaces one support file. |
| `remove_file` | `name`, `file_path` | Removes one support file. |

`old_text`/`new_text` are not skill fields, and there is no advertised `edit`,
`list`, or archive action. Results contain success, applied operation count,
action/name/path, or a bounded error; mutation results never echo skill
content.

## Layout and validation

The server receives only the per-user root:

```text
<appconfig.Dir()>/skills/
  .ownership.json
  [<category>/]<name>/SKILL.md
  [<category>/]<name>/references/**
  [<category>/]<name>/templates/**
  [<category>/]<name>/scripts/**
  [<category>/]<name>/assets/**
```

There is no `learned/` or archive layer. A bare skill name must be unique
across the root and one optional category level. Workspace and bundled skill
roots remain engine-readable but are never passed to this writer.

Validation before publication:

- names and categories use lowercase letters, digits, and single hyphens, with
  a 64-rune maximum;
- `SKILL.md` is valid UTF-8, at most 100,000 runes, starts with closed YAML
  frontmatter, has matching `name`, a non-empty `description`, and a non-empty
  body;
- a newly created skill's description is at most 60 runes; existing skill
  files may retain descriptions up to 1,024 runes;
- optional `compatibility` is at most 500 runes;
- support files are valid UTF-8 and at most 1 MiB;
- support paths stay below `references/`, `templates/`, `scripts/`, or
  `assets/`; absolute paths, traversal, symlinks/junctions, and special files
  are refused.

Create publishes a completed temporary directory with one rename. Patches and
support writes use temporary-file, sync, close, then rename. A batch snapshots
every touched skill and restores all touched trees if an operation, ownership
update, or cancellation fails.

## Background-review safety

The PreToolUse hook overwrites `_session_id` and `_background_review` on every
skills call; these fields are host context, not model authority. Foreground
calls use normal validation. A background review may modify or delete only a
skill it previously created during a background review, recorded in
`.ownership.json`.

Existing installations are migrated without weakening this boundary. The
legacy `.gotack-agent-skills.json` file is read only when `.ownership.json` is
absent; the next ownership-changing write publishes the current filename. If
the current manifest exists but is corrupt, unreadable, redirected, or has an
unsupported version, loading fails closed and never falls back to the legacy
file.

Before changing an existing `SKILL.md` or support file, the same review must
call `skill_view` for that exact file. `skill_view` records the file's digest;
`skill_manage` consumes the mark and rechecks the current digest immediately
before mutation. A changed or missing mark fails closed. New skills and new
support files need no prior read, and mutations created earlier in the same
atomic batch are tracked without a redundant read.

This handshake is intentionally retained even though Crush has a canonical
`view`: the canonical view runs in the Crush process, while `skill_manage`
runs in `skills.exe`; without a same-process mark, the writer would either
accept an unproven background edit or have to reject every edit.

## Registration

`skills_seed.go` registers this workspace-scoped entry on every runtime rebind:

```json
{
  "mcp_servers": {
    "gotack-skills": {
      "command": "<absolute path to skills.exe>",
      "args": ["--root", "<appconfig.Dir()>/skills"],
      "type": "stdio",
      "timeout": 30
    }
  }
}
```

If `skills.exe` cannot be resolved beside the app or on `PATH`, the host
removes `mcp_servers.gotack-skills`. The engine can therefore continue without
a dangling MCP entry. The host also merges the same user root into
`options.skills_paths`; Crush rebuilds its catalog at the existing runtime
boundary, without a watcher or polling loop.
