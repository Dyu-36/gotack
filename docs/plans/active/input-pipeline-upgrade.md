# Input Pipeline Upgrade — execution and audit evidence

Updated: 2026-09-06. Outstanding work is maintained only in
`docs/plans/active/windows-release-completion.md`; this file records authority, implementation evidence,
validation limits and recovery. The milestone is **not release-approved**.

## Record status

This is historical audit evidence, not the current execution checklist. Follow [Windows release completion](windows-release-completion.md) for remaining work. Old root-plan references below refer to the explicit historical Git blob, never to a live file that should be recreated.

## Authority and operating boundary

The owner's current request authorizes auditing the implementation, fixing
existing defects, cleaning the source, reducing ImplementPlan to unfinished
work, and deleting the obsolete WebPlan when appropriate. The recorded
2026-09-05 owner instruction to integrate directly on `main` remains the
repository working policy; no owner worktree/ignored engine checkout is reset
or cleaned. `AGENTS.md`, `docs/WORKFLOW.md`, accepted contracts and decisions
remain binding. Desktop/engine communication stays REST + SSE.

`WebPlan.md` is removed: it duplicated the original requirements, described
already implemented phases as NOT STARTED, and prescribed branch/checkpoint
stops superseded by the newer owner instruction. Its relevant safety boundaries
remain here, in AGENTS/contracts, and in the root backlog. The original detailed
requirements remain retrievable without maintaining another stale plan:

```text
git show 10a9879b745098f03ac0dcf97baa523125732c5e:ImplementPlan.md
git show 10a9879b745098f03ac0dcf97baa523125732c5e:WebPlan.md
```

Do not interpret removing historical checklists as waiving acceptance.
Hybrid/local compaction remains outside the milestone. Do not call paid live
providers without an approved request/cost cap, request secrets in chat, install
an unapproved compiler, change Windows services or branch protection, or test
migration on the owner's real data. Existing provider availability is not a
missing-provider blocker.

## Baseline inspected

Repository: `Dyu-36/gotack`; baseline main:
`10a9879b745098f03ac0dcf97baa523125732c5e`.
Crush pin: `6d14dd93a9e526505f7de54ae5999431bc32a793`.
The manifest contains four compatibility patches followed by hardening and
three input-pipeline patches (core, telemetry, history-anchor). Fantasy
upstream acceptance is separate from the authoring patch.

The baseline execution ledger records implemented PR0/PR1/PR2, host/UI PR4,
Crush-side PR5 and synthetic PR3 infrastructure. It also records the portable
real-app migration matrix and the shipped-manifest slash fix `f865211` as PASS.
Those records are historical evidence, not newly executed checks on this
cleanup candidate. They are not a reason to rebuild already completed features.

The connected GitHub job read for baseline workflow run `34021787121`
confirmed success for `Clean-pin executable REST SSE fixtures`, including
Gotack tests, repository invariants and isolated engine replay/build/E2E.
Its workflow sets `CGO_ENABLED=0`: this was **not Windows race evidence**.
Baseline nested fsext test and csync vet failures, Fantasy authorization,
Windows race and live-acceptance gaps remain explicitly in ImplementPlan.

## Cleanup implementation

Code checkpoint: `19c6ca27cbb760394c5c2c674897d82feb8d3f1a`.

- `scripts/input-pipeline/gate.mjs`: validate event/action shapes, reject arrays,
  malformed typed fields and unknown actions, and reject `build-fail` even when
  a fabricated stream otherwise contains all required RUN/PASS events.
- `scripts/input-pipeline/run.mjs`: one artifact-writing path validates the
  ORIGINAL stream, exports only allowlisted lifecycle fields, never raw Output,
  stderr, arbitrary metadata or dynamic subtest names, and records failure.json
  for exit-zero validation failures as well as subprocess failures/timeouts.
  result.json is emitted only after successful validation.
- `scripts/input-pipeline/gate.test.mjs`: six additional regression tests cover
  malformed records, build failures, diagnostic canaries, the actual success
  artifact sink, exit-zero rejection receipts and process failure/timeout.
- `internal/contextseed/snapshot_identity.go`: extract key ownership from the
  mixed snapshot module; stage/write/sync/close the complete key and publish via
  a no-replace same-directory hard link. A concurrent loser adopts only a valid
  committed key; read errors/corruption fail closed instead of regenerating or
  overwriting identity. Key encoding and snapshot identity domain are unchanged.
- `internal/contextseed/snapshot_key_test.go`: concurrent goroutines, independent
  publisher processes, reuse, corruption, read errors and staging cleanup.
  The pre-existing snapshot_identity_test.go is preserved, not overwritten.

The key fix does not claim to solve snapshot retention, complete-generation
validation or every Windows ACL/crash case. Those gaps remain IP-02/IP-05.
No engine patch, pin, UI behavior, live credentials or user profile was changed
by this cleanup. Documentation/reference cleanup is separate from code evidence.

The root plan now contains only eight unclosed work items; the obsolete WebPlan
is deleted. third_party/README.md describes the sanitized lifecycle/failure
receipts and original-stream validation. The REST/SSE contract's outdated
size-keyed seeding paragraph is corrected to match bundleseed's SHA-256
comparison and stored size/hash metadata; no seeding behavior is changed by
that documentation correction. Remaining repository-wide reference/unused-code
inventory is explicitly unverified, not silently called clean.

## Checks actually executed during this audit

Local environment: Linux, Node v22.16.0, Go go1.23.2. The runtime had reconstructed
selected source files, not a complete repository checkout. Original gate/run/test
and snapshot files were matched against Git blob SHA before using them. Direct
Git cloning failed with DNS resolution unavailable; no Windows/PowerShell runtime
was available. The checks below therefore have deliberately bounded scope.

| Check | Observed result and limit |
| --- | --- |
| Baseline `node --test scripts/input-pipeline/gate.test.mjs` | PASS: original 12 tests |
| Baseline malformed-event/build-fail/canary probes | Reproduced false acceptance for five malformed/failure inputs and a synthetic canary leak through an arbitrary failing subtest name |
| Expanded gate suite before fixes | Expected FAIL: 12 passed, 6 failed; regression proof, not a passing gate |
| Expanded gate suite after fixes | PASS: 18 tests, zero failures/skips; these are JavaScript harness unit tests, not the 14 Windows executable E2E tests |
| `node --check` on gate.mjs, run.mjs, gate.test.mjs | PASS |
| Original key helper, concurrent publishers, GOMAXPROCS=8 and count=10 | Expected FAIL: returned identities differed from the committed key. An earlier single run passed; concurrency defects are schedule-dependent |
| `GO111MODULE=off GOTOOLCHAIN=local go test -race -count=3 -timeout=60s -v snapshot_identity.go snapshot_key_test.go` | PASS on Linux, including independent publisher processes; only the extracted key implementation/test files, not the complete contextseed package or Windows race gate |
| `GO111MODULE=off GOTOOLCHAIN=local go vet snapshot_identity.go snapshot_key_test.go` | PASS for the same isolated key files |
| `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 ... go test -c` for the same files | PASS cross-compilation only; Windows executable was not run locally |
| gofmt and UTF-8/LF checks on changed local files | PASS |
| Full Gotack/frontend/nested tests and repository invariants on this candidate | NOT RUN locally; require the actual repository/toolchain/CI, not substituted fixture results |
| Windows executable E2E, ACL/portable flow, Windows race and live provider | NOT RUN locally on this cleanup candidate |

Local temporary logs existed under the audit runtime's `gotack-audit/evidence`
directory. They are not a persistent CI dependency or release attestation. The
checked-in regression tests provide the repeatable proof; the full gate must
record safe provenance/artifacts for the integrated HEAD.

## Revalidation and recovery

Run the native validation families from ImplementPlan on a clean committed
checkout, preserving unrelated owner edits. The fixed harness still refuses
dirty/untracked gate inputs, unverified binaries and unexpected skips. Its
Windows entrypoint remains `./scripts/test-input-pipeline-e2e.ps1`; the required
executable-test inventory remains 14, not the stale eight from older handoffs.

Use `git revert` of the relevant cleanup commit(s) to recover. Do not reset the
owner's working tree or ignored `third_party/crush`, and do not delete a real
profile. Reverting documentation does not create new acceptance evidence.
