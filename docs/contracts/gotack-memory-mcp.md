# Contract: gotack-memory MCP server (`cmd/memory`)

Status: implemented by `internal/memory`, `cmd/memory`, and `memory_seed.go`.

`gotack-memory` is the only sanctioned agent writer for the two bounded
memory files injected through `options.global_context_paths`. It uses MCP over
stdio and exposes exactly one tool, `memory`. There is no read action because
the files are already present in the next system prompt.

## Tool shape

`target` selects `memory` (`MEMORY.md`) or `user` (`USER.md`). A request uses
either the single-operation fields or an `operations` array:

```json
{
  "target": "memory",
  "operations": [
    {"action": "replace", "old_text": "old unique text", "content": "new entry"},
    {"action": "add", "content": "another entry"}
  ]
}
```

The single-operation compatibility shape is
`{target, action, old_text?, content?}`; `new_text` is accepted as an alias for
`content`. `operations` is preferred for consolidation. When it is present, the
single-operation fields are ignored.

| Action | Contract |
| --- | --- |
| `add` | Append one non-empty entry. An exact duplicate succeeds without writing another copy. |
| `replace` | Find the one entry containing `old_text` and replace that whole entry with `content`. |
| `remove` | Find the one entry containing `old_text` and remove that whole entry. |

`old_text` must identify exactly one entry. Zero or multiple matches are
errors; the tool never guesses. Unknown targets/actions, empty batches,
missing fields, invalid UTF-8 already on disk, and blocked content are also
refused.

## Bounded atomic behavior

The character budgets are the Hermes limits, measured as Unicode runes over
the serialized entry body:

| Target | File | Cap |
| --- | --- | ---: |
| `memory` | `MEMORY.md` | 2,200 characters |
| `user` | `USER.md` | 1,375 characters |

A batch is evaluated against a copy and committed only if every operation and
the final state are valid. Intermediate overflow is allowed so one batch can
consolidate old entries before adding a replacement. Final overflow refuses
the entire batch and leaves the file unchanged; the store never evicts an
entry. This is the policy in decisions
[`0003`](../decisions/0003-memory-writes-constrained-by-construction.md) and
[`0004`](../decisions/0004-memory-refuses-instead-of-evicting.md).

All candidate `add`/`replace` content is scanned before filesystem access for
instruction override, exfiltration instructions, and hidden Unicode. Each
target has a cross-process lock. Persistence is temporary-file, sync, close,
then atomic replacement, so readers see either the old or complete new file.

Successful results are deliberately compact and never echo stored text:

```json
{
  "success": true,
  "done": true,
  "target": "memory",
  "usage": "14% — 312/2,200 chars",
  "entry_count": 3,
  "message": "Applied 2 operation(s).",
  "note": "Write saved. This update is complete — do not repeat it."
}
```

An over-cap refusal, or a recoverable `old_text` locator refusal, includes the
unchanged `current_entries` and `usage` so the model can correct one bounded
batch without a read tool. Other failures do not echo memory content. Domain
failures are JSON tool results with `success: false`; MCP transport remains
healthy.

## File representation

```text
<appconfig.Dir()>/context/memory/
  MEMORY.md
  USER.md
```

Entries are raw UTF-8 text separated only by the exact delimiter `\n§\n`.
Non-empty files receive a compact fullness wrapper:

```text
══════════════════════════════════════════════
MEMORY (your personal notes) [14% — 312/2,200 chars]
══════════════════════════════════════════════
first entry
§
second entry
```

`USER.md` uses `USER PROFILE (who the user is)` in the same wrapper. Empty
memory remains an empty file and consumes no prompt tokens. The parser also
migrates the earlier marker/provenance representation without carrying its
generated wrappers forward; new writes do not add provenance metadata.

## Registration and removal

On every workspace rebind, `memory_seed.go` writes:

```json
{
  "mcp_servers": {
    "gotack-memory": {
      "command": "<absolute path to memory.exe>",
      "type": "stdio",
      "timeout": 30
    }
  }
}
```

The binary defaults to `<appconfig.Dir()>/context/memory` and accepts `-dir`
for an explicit root. If the executable cannot be resolved beside the app or
on `PATH`, the host removes `mcp_servers.gotack-memory` rather than leaving a
dangling entry.

The PreToolUse guard denies generic file-writing tools aimed at the context
directory, including background reviews. Background reviews may mutate these
files only through `memory`, where the caps, scan, lock, and atomic batch are
unavoidable. Manual editing or deleting the plain-text files remains the
recovery path.
