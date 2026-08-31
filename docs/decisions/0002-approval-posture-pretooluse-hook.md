# 0002 Graduated approval posture via the PreToolUse hook

Date: 2026-08-31

## Status

Accepted

## Context

Today every workspace attaches with Crush permission prompts skipped, the
default assistant workspace is the drive root, and the Zalo bridge reaches
the agent with a six-digit pairing code. Anyone who obtains that code reaches
an auto-approving agent scoped to the whole drive. The Hermes parity goal
requires the opposite: a destructive-command floor, a write-safe root, and
graduated approval modes. Reversing blanket auto-approval is a real UX
regression, and unattended (scheduled, remote) sessions cannot answer
interactive prompts.

Crush supports exactly one hook event, `PreToolUse`, whose `deny` decision is
affirmative enough to block a tool call with a legible reason, while silence
falls through to the existing permission relay (`internal/permission`). The
user delegated this decision on 2026-08-31.

## Decision

Implement the approval posture as one `PreToolUse` hook (`cmd/guard`), with
no new Crush change and no new UI:

- **deny** — an unrecoverable-command blocklist (recursive force delete,
  disk format, mass permission changes, shutdown, history rewrite, credential
  exfiltration to network sinks), never overridable, with the refusal reason
  naming the rule. Ship this first, before any mode change, so the security
  floor arrives without new prompts.
- **write-safe root** — file writes outside the root are denied; writes into
  `<appconfig.Dir()>/context/` are always denied, closing the memory
  cap-bypass hole (see decision 0003).
- **ask** — operations outside the auto-approved set route through the
  existing permission relay to the UI. No second prompt path is invented.
- **auto** — ordinary local reads and edits inside the safe root, keeping an
  explicit opt-in escape hatch for today's fully-automatic behaviour.
- Remote-originated (Zalo) and scheduled sessions default to the stricter
  posture regardless of the desktop setting; an unattended session that would
  need an answer fails the run with a legible reason instead of blocking.

Known residual hole, accepted with mitigation: `PreToolUse` does not fire for
sub-agent tool calls, so a delegation through the `agent` tool runs outside
the guard. Mitigation: in the strict posture, restrict the `agent` tool
itself. Fully closing the hole requires the Phase 7 fork.

## Alternatives Considered

1. Keep the status quo (blanket skip). Rejected: the remote entry point makes
   it an unrecoverable-impact attack surface.
2. Build approval modes inside Crush (fork). Rejected for Phases 1–6: the
   hook covers the same matrix without fork maintenance cost.

## Consequences

Positive:

- A security floor exists in release builds without modifying Crush.
- The posture is one config key (`hooks.PreToolUse`), removable with
  `RemoveConfigField`.

Tradeoffs:

- Users who relied on never being asked will be asked (mitigated by the
  opt-in escape hatch and by shipping deny-only first).
- The sub-agent hole remains until a fork exists.

## Follow-Up

- `docs/plans/active/hermes-parity-harness.md` Phase 4 implements this.
- Revisit the sub-agent hole if the Phase 7 fork happens.
