# Repository Workflow

Repository product behavior, architecture, decisions, plans, code, tests, CI and runtime signals are the system of record.

## Repository Map

- `AGENTS.md`: entry map and repository rules.
- `README.md`, `docs/product/`, architecture and decisions: current intent and constraints.
- `docs/plans/`: durable work; `docs/templates/`: optional structures.
- Code, tests, CI and runtime signals: executable and observable truth.

Use `docs/README.md` for the complete map.

## Select The Work Shape

### Does The Work Need Durable Memory?

Use an ephemeral plan for bounded work. Create one plan in `docs/plans/active/` when work spans sessions, coordinates contributors, has meaningful dependencies, needs recovery, or cannot safely resume from its diff.

Use `docs/templates/exec-plan.md`. Keep progress and task-local decisions in the same file; avoid parallel task records without an independent audience.

### Does The Work Need Human Judgment?

Before editing, identify authority for new externally observable policy. If materially different choices remain, stop and request the smallest decision. Configurable defaults are not authority.

For example, `Add rate limiting` without a quota, trusted key, enforcement topology or response contract must stop. `Enforce the documented 20 requests per minute per authenticated tenant` may proceed.

Also pause for ambiguous product intent, difficult recovery, weakened validation, security or compatibility, and insufficient authority.

### What Proves The Behavior?

Use focused tests for local rules, integration tests for boundaries, end-to-end interaction for user-visible behavior, recovery rehearsal for dangerous operations, and measurements for reliability or performance.

Plans, checklists and completion messages do not prove product behavior by themselves.

### Does The Work Encode An Invariant?

For architecture, reliability, security or quality boundaries:

1. Find an accepted repository authority that states the required boundary. Conventions, code patterns, tests, defaults and undocumented preferences do not establish policy.
2. Reuse the repository's native validation owner and command. Add the smallest mechanical check that covers the accepted scope and emits a useful diagnostic.
3. Require positive proof that allowed behavior passes and negative proof that the targeted forbidden behavior fails for the intended reason.
4. Report enforcement precisely: a local command is available or passed; CI either invokes the check or does not; external branch protection remains external unless verified.

Use `docs/patterns/encoding-invariants.md` when implementing such a guard.

## Task Flows

### Read-Only Request

Read only what the answer, review, diagnosis, plan or status needs. Do not edit files or mutate repository/runtime state.

### Bounded Change

Restate the outcome, inspect its authority, implementation, patterns and proof, make the smallest coherent change, run focused and required checks, and report the outcome, changes, evidence and limits.

### Durable Planned Change

Create or resume one active plan. Keep outcome, context, approach, risk, recovery, progress, decisions and validation current. Implement in verifiable groups, promote lasting decisions, run focused and repository proof, then record the result and move the plan to `docs/plans/completed/`.

### Operate The Application

When a task requires the real application:

1. Find the consumer-owned runbook and verify prerequisites and ownership.
2. Start only an isolated instance, prove readiness and create known state.
3. Reproduce through the real interface and inspect correlated runtime evidence.
4. Validate through that interface, then stop only resources this run owns.

If no verified runbook exists, inspect current repository authority and report or propose the missing guidance. Do not invent commands, credentials, product policy or cleanup obligations.

## Completion Standard

A change is complete when the outcome exists or its blocker is explicit, repository truth remains current, behavior-appropriate proof passed or its gap is disclosed, any required plan is current, and the report separates facts, limits and unattempted work.
