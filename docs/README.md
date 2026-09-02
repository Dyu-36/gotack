# Documentation Map

Start with the smallest authoritative surface.

## Current Documents

- `../README.md`: product overview, architecture, integrations, stack
  baseline, and repository layout.
- `WORKFLOW.md`: request shape, planning, judgment, operation, validation, and
  completion.
- `contracts/`: external boundaries and host-owned behavior. It covers Wails,
  Crush REST/SSE, approvals, memory, recall, skills, reflection, scheduling,
  and Zalo. Update the owning contract in the same change as its
  implementation.
- `decisions/`: accepted architecture and safety decisions. Its `README.md`
  indexes the current ADRs and their implementation consequences.
- `plans/`: durable working-memory documents; `active/` while in progress,
  `completed/` after validation.
- [`patterns/encoding-invariants.md`](patterns/encoding-invariants.md): turn
  accepted architecture, reliability, security, and quality rules into native
  mechanical validation.
- `templates/`: optional decision, plan, and runbook structures.

## Frontend placement rule (`frontend/src/`)

New frontend files go into the folder that owns their responsibility; there
is no generic `utils/` or `shared/` catch-all.

| Folder | Owns |
| --- | --- |
| `app/` | app-wide state shared by every view (currently `theme.svelte.ts`) |
| `components/` | reusable presentational and interactive components: chat area, composer, message bubble, sidebar, panels, modals |
| `features/<name>/` | one feature module per slice with its types, state, helpers, and tests; `*.svelte.ts` holds reactive state, plain `.ts` stays pure |
| `lib/` | generic helpers with no feature knowledge (currently `markdown.ts`) |
| `platform/` | the only place that talks to the desktop host: `desktop.ts` (AGENTS.md hard rule 3) plus the generated `events.generated.ts` |

`features/conversations` is the live conversation slice. Verified used, not
dead: five modules import it — `App.svelte` and the components
`ChatArea.svelte`, `Composer.svelte`, `MessageBubble.svelte`, and
`SettingsModal.svelte`. Do not prune it.

Code, tests, CI, and runtime signals are the executable truth for product
behavior; these documents describe intent and boundaries.
