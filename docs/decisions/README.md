# Decisions

Decision records preserve lasting product, architecture, data ownership,
security, compatibility, and validation choices that future work must inherit.

Use `docs/templates/decision.md`. Task-local implementation choices remain in
the active execution plan and do not require a separate decision.

An installed consumer begins with no fabricated decisions. Add local decision
documents here as real choices are accepted, then index them in this file.

## Index

- [`0001-narrow-no-agent-logic-rule.md`](0001-narrow-no-agent-logic-rule.md)
  — narrow `AGENTS.md` hard rule 4 to named responsibilities so desktop-layer
  memory, recall, scheduling, and reflection are permissible over REST + SSE.
- [`0002-approval-posture-pretooluse-hook.md`](0002-approval-posture-pretooluse-hook.md)
  — graduated approvals (deny blocklist, write-safe root, ask/auto modes) via
  one `PreToolUse` hook; remote and scheduled sessions default stricter.
- [`0003-memory-writes-constrained-by-construction.md`](0003-memory-writes-constrained-by-construction.md)
  — memory writes need no interactive approval; safety comes from caps,
  atomic writes, provenance, and denial of every other write path.
