# Execution Plan: Stack Parity For Zalo And Agent Prompts

Date: 2026-08-31

## Status

Completed

## Outcome

Gotack preserves the already-integrated Stack timetable and Office CLI assets,
accepts image payloads sent through the Zalo Bot API in the shapes supported by
`C:\stack`, and runs primary/sub-agent system prompts aligned with Stack's Tack
and Sage behavior while retaining Crush-specific tool and context injection.

## Context

- The user's request makes current `C:\stack` behavior the parity source.
- `docs/contracts/wails-bindings.md` defines the existing Zalo and bundled
  Office/timetable behavior.
- `docs/plans/completed/zalo-timetable-integration.md` records the prior
  timetable and Office integration and its validation.
- `internal/zalo/client.go` parses inbound Bot API media payloads.
- `third_party/crush/internal/agent/` owns Gotack's runtime system prompts.
- `C:\stack\.tack\agents\tack.md` and
  `C:\stack\tackcode\crates\tack_repo\src\agents\sage.md` are the requested
  prompt references.

## Scope

In scope:

- Prove whether timetable and Office CLI integration is already present and
  synchronized with Stack; change it only if a difference is found.
- Accept Stack-supported direct/nested inbound Zalo image URL shapes and retain
  useful image extensions when saving attachments.
- Align Crush's primary and read-only task prompts with Stack's Tack and Sage
  roles without referencing tools Gotack does not expose.
- Add focused regression proof and run repository checks proportional to the
  changed surfaces.

Out of scope:

- Modifying `C:\stack` or replacing Crush with Stack's Rust engine.
- Redesigning the Zalo settings UI or changing the existing Wails binding.
- Reimplementing timetable or Office functionality that is already byte-identical.

## Approach

1. Compare source trees and executable hashes for timetable and Office assets.
2. Port the missing Stack Zalo media payload compatibility into Gotack and add
   focused parser/filename tests.
3. Adapt Stack's Tack and Sage system-prompt intent to the existing Crush Go
   template data and tool model, preserving dynamic skills/context sections.
4. Run focused Zalo and Crush prompt tests, then broader Go/frontend/build proof
   as appropriate; record results and close the plan only after validation.

## Risks And Recovery

- Both Gotack and its vendored Crush worktree contain pre-existing dirty changes.
  Preserve them and patch only the missing behavior; recovery is reverting only
  hunks added under this plan.
- Stack template syntax and tools differ from Crush. Keep semantic parity while
  retaining only executable Gotack instructions, proven by rendering tests.
- Zalo's live API requires a secret and external service. Use deterministic
  payload tests for parser behavior and report live validation separately.

## Progress

- [x] Read repository workflow and relevant contract/previous plan.
- [x] Compare timetable and Office assets with Stack.
- [x] Fix and test inbound Zalo image compatibility.
- [x] Align and test primary/sub-agent prompts.
- [x] Run broader validation and record the result.

## Decisions

- 2026-08-31: Treat byte-identical timetable, Office skill, and Office CLI assets
  as already integrated; avoid unnecessary copies.
- 2026-08-31: Treat current Stack Tack/Sage prompt content as behavioral
  authority, but translate tool-specific wording because Crush exposes a
  different tool catalog and template engine.

## Validation

- Focused proof: `go test ./internal/zalo -run
  'Test(ParseUpdatesAcceptsInboundImageShapes|ParseUpdatesAcceptsDataWrapper|InboundImageReachesAgentTurn|AttachmentFileNamePreservesImageContentType)$'
  -count=1` passes. `go test ./internal/agent ./internal/config` and the
  complete `go test ./...` pass in `third_party/crush`.
- Integration or end-to-end proof: the fake Bot API integration test parses a
  `photo_url`, downloads the image with its WebP extension, and observes the
  downloaded path in the agent turn. `scripts/build.ps1` succeeds; exact Tack
  and Sage prompt markers are present in packaged `crush.exe`. The Stack source
  and packaged Gotack timetable skill hashes match, and the packaged runtime
  smoke test produces a valid timetable workbook. A live Zalo message was not
  sent because that would require and exercise the user's external bot/chat.
- Repository-required checks: root `go test ./...`, `go vet ./...`,
  `pnpm --dir frontend check`, `git diff --check`, and
  `scripts/build.ps1` pass.

## Result

Timetable and all nine Office skill trees plus `officecli.exe` were already
byte-identical to Stack, so no duplicate asset changes were made. Gotack now
accepts Stack's direct and nested inbound Zalo image URL shapes, handles `data`
wrappers, preserves GIF/WebP/BMP attachment extensions, and proves the image is
downloaded into the agent turn. The packaged Crush runtime now uses a
Stack-aligned Tack primary prompt and Sage read-only research prompt while
retaining Crush's dynamic skill/context injection. The full desktop build and
all focused/broad automated checks pass; live external Zalo delivery remains
unattempted.
