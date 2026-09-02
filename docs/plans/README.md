# Execution Plans

This directory keeps the implementation plan for each non-trivial, cross-cutting change.
A useful plan records the problem, scope, sequence, compatibility policy, validation steps,
and rollback path before code starts moving. It is the handoff between design, implementation,
and review.

## Lifecycle

1. Create a plan in `active/` before editing behavior that spans packages or changes contracts.
2. Update the plan while implementing when the constraints or sequence change.
3. Validate the final implementation against the plan, tests, builds, and user-facing behavior.
4. Move the plan to `completed/` when the work is done; do not rewrite history after completion.

## Active Plans

No active plans.

## Recently Completed

- [`hermes-learning-loop.md`](completed/hermes-learning-loop.md)
  — completed the bounded Hermes-derived memory, skills, reflection, guard, and recall loop; added strict detached-session context bounds, ownership migration, clean-checkout CI, and inspected Windows packaging.
- [`hermes-parity-harness.md`](completed/hermes-parity-harness.md)
  — locked the durable Hermes memory, skill-management, recall, and background-review behavior to the current audited contracts and regression suites.
- [`tray-window-behavior.md`](completed/tray-window-behavior.md)
  — restored hidden startup, explicit window activation, and hide-to-tray lifecycle.
- [`external-chat-zalo.md`](completed/external-chat-zalo.md)
  — added the production Zalo integration and reusable chat-bridge subsystem.
- [`hermes-agent-parity.md`](completed/hermes-agent-parity.md)
  — completed the Gotack/Crush implementation of Hermes-style memory, skills,
  recall, and background reflection.
- [`crush-codex-parity.md`](completed/crush-codex-parity.md)
  — documented and completed the ChatGPT/Codex transport port.
- [`ui-overhaul.md`](completed/ui-overhaul.md)
  — delivered the original application shell overhaul.
- [`ui-finish.md`](completed/ui-finish.md)
  — completed the product-grade UI finish, Composer attachment pipeline,
  accessibility pass, and frontend verification.
