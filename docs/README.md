# Documentation Map

Start with the smallest authoritative surface.

## Current Documents

- `../README.md`: product overview, architecture, integrations, stack
  baseline, and repository layout.
- `WORKFLOW.md`: request shape, planning, judgment, operation, validation, and
  completion.
- `contracts/wails-bindings.md`: the UI/host boundary (bound methods and
  events); update it in the same change as the binds.
- `product/`: current product behavior.
- `decisions/`: lasting choices future work must inherit.
- `plans/`: durable working-memory documents; `active/` while in progress,
  `completed/` after validation.
- [`patterns/encoding-invariants.md`](patterns/encoding-invariants.md): turn
  accepted architecture, reliability, security, and quality rules into native
  mechanical validation.
- `templates/`: optional decision, plan, and runbook structures.

Code, tests, CI, and runtime signals are the executable truth for product
behavior; these documents describe intent and boundaries.
