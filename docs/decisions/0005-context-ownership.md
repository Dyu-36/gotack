# ADR: Context Ownership Model

**Status:** Accepted
**Date:** 2026-09-05
**Authority:** ImplementPlan.md section 6, AGENTS.md

## Context

The original `resources/context/TACK.md` served as both product identity and
user customization surface. This created problems:
- Gotack releases could not safely upgrade the file when users modified it
- Generic coding rules were duplicated between TACK.md and the engine template
- No clear ownership boundary between product-managed and user-owned content

## Decision

Adopt a two-layer context ownership model:

### Layer 1: TACK_CORE.md (Product-Managed)

Contains Gotack-specific identity and capability definitions:
- Core principles and operating behavior
- Windows/Office/Zalo capabilities
- Guard, memory, recall, and skills integration
- File system and shell operation guidance
- Hard prohibitions

Updated atomically by Gotack releases. Never directly edited by users.

### Layer 2: USER.md (User-Owned)

Contains user customizations:
- Personal preferences
- Project-specific rules
- Custom response formatting
- Team workflows

Preserved across Gotack updates. User edits are always respected.

## Migration from Legacy TACK.md

### Stock Legacy (Hash Match)

When the existing `TACK.md` hash matches the shipped stock:
1. Create staging directory
2. Write TACK_CORE.md and USER.md to staging
3. Atomic rename staging contents to context directory
4. Backup legacy TACK.md to `.migration-backup/<timestamp>/`
5. Remove legacy TACK.md
6. Write migration manifest with status `migrated`
7. Rebuild prompt snapshot

Rollback: restore TACK.md from backup, remove managed files, update manifest.

### Modified Legacy (Hash Mismatch)

When the existing `TACK.md` has been modified:
1. Set status to `migration_pending`
2. Generate diff/preview for user review
3. Wait for explicit user approval
4. On approval: 3-way merge with base version, user resolves conflicts
5. On cancel: status remains `migration_pending`, legacy file untouched

Never auto-migrate modified legacy content.

## Invariants

- Snapshot contains exactly one policy owner: managed core OR legacy, never both
- Custom USER.md bytes are always preserved
- Backup and rollback tokens kept for one release
- Atomic rename prevents half-written prompt state
- Stock hash manifest is versioned for future upgrades

## Consequences

- New installs get TACK_CORE.md + empty/guidance USER.md
- Existing unmodified installs migrate automatically
- Modified installs require user action (safe, explicit)
- Engine template remains the source for generic coding rules
- Context ownership is clear and auditable
