# third_party

Upstream engine source is used for contract inspection and release builds. It
is ignored by the parent repository and is not part of the Gotack Go module.
Desktop integration is REST + SSE through `internal/crushapi`; never import
`third_party/crush/internal/...` into Gotack.

## Pin and ordered recipe

The single tracked Crush pin owner is the repository-root `.tack-pin`.
The upstream is `https://github.com/charmbracelet/crush`.

`third_party/patches/manifest.json` declares two ordered arrays. Every tracked
`.patch` in this directory must occur exactly once; missing, duplicate,
unlisted, nested, or escaping patch paths are errors. The order is:

1. Clean pinned Crush source.
2. `compatibility` patches, in manifest order.
3. `scripts/harden-crush-for-tack.ps1`.
4. `input_pipeline` patches, in manifest order.
5. Contract checks, then build/test.

The compatibility inventory is ChatGPT subscription OAuth, Hermes skill
refresh, proactive auto-compaction, and prompt context refresh. The current
`input_pipeline` list is empty. There is no accepted
`zz-input-pipeline-windows.patch`; earlier prose describing one was a prototype
claim, not a replayable artifact. Alphabetical names do not define phase order.

Hardening removes the Question agent tool/routes and applies Tack's
model-visible identity. It preserves upstream module paths, legacy executable
fallback names, and the `crush://` compatibility URI. The proactive context
patch and existing compaction thresholds remain unchanged by Phase 0A.

`apply-crush-patches.ps1 -SkipInputPipeline` actually omits only the final
phase, and reports that omission. Such a build is not accepted by the required
input-pipeline gate. Missing manifest/patch/hardening inputs fail closed.

## Isolated replay and E2E

Use PowerShell 7 from a clean, committed Gotack checkout:

```powershell
node --test scripts/input-pipeline/gate.test.mjs
./scripts/test-input-pipeline-e2e.ps1
```

The script resolves the repository root itself. It creates a unique directory
under `RUNNER_TEMP` (or the OS temp directory), fetches the exact pin into a new
upstream repository, and never uses the owner's ignored `third_party/crush`.
No Gotack branch or worktree is created. The command checks each native exit
status, bounds subprocess duration, and builds the engine from root `.` using
`go build -mod=readonly -trimpath -o <absolute-binary> .`.

The provenance receipt contains the Gotack commit, Crush pin, ordered patch
SHA256s, manifest and replay-script SHA256s (including hardening), Fantasy
version/checksum read from both the module graph and binary, build settings,
build command, absolute binary path, and binary SHA256. It is a reproducibility
receipt, not a cryptographic attestation by an independent build service.

A two-step invocation is supported:

```powershell
./scripts/test-input-pipeline-e2e.ps1 -BuildOnly
# Use the two exact absolute paths printed by the successful build:
./scripts/test-input-pipeline-e2e.ps1 -SkipBuild `
  -EngineBinary '<absolute-binary>' -Provenance '<absolute-provenance.json>'
```

`SkipBuild` never searches PATH. It verifies the receipt against the current
commit, current recipe, and the actual binary metadata/hash. An old receipt,
missing binary, uncommitted gate input, skipped phase, or Fantasy replacement
is an error. It does not mutate the owner's global Go settings/toolchain.

The required Windows tests drive a real executable via a unique named pipe,
create workspace/session IDs through REST, attach SSE before prompting, and
wait for the matching terminal event. The local fake Responses server handles
text, 429/retry, a real MCP tool loop, replay after restart using the same
isolated database, and malformed-stream rejection. Primary and title model
calls are distinguished explicitly. No live provider credentials are inherited.
The child profile/config/cache/temp paths are isolated; provider discovery and
metrics are disabled, and the fake endpoint never proxies external requests.
This is fixture isolation, not a system-wide firewall or arbitrary-code sandbox.

Only `provenance.json`, `tests.jsonl`, and `result.json` are evidence artifacts.
Captured requests remain in memory. Child stdout/stderr and engine profile
contents are not exported. The MCP audit contains only initialize/call counts.
Do not upload engine logs, profiles, request bodies, or environment dumps.
The unique upstream build directory/receipt remain in OS temp for inspection;
remove only that exact generated directory when it is no longer needed.

The JSON post-check requires every named acceptance test to RUN and PASS, a
passing package, and zero unexpected skips. Build-only success, unit fixtures,
scaffold tests, and a workflow definition are not E2E acceptance evidence.
`.github/workflows/input-pipeline.yml` invokes the Windows gate; its actual run
status and any branch-protection requirements must be verified separately.

## Release and patch maintenance

Product binary lookup is unchanged: a bundled `resources/tack-engine.exe`
next to `tack.exe` is preferred; the existing explicit override and developer
PATH fallback remain product behavior. The E2E harness has a stricter policy.

The existing `scripts/update-crush.ps1` is the deliberate developer refresh
entrypoint, not the isolated E2E entrypoint. Do not run it over the owner's
known-dirty ignored engine tree during this milestone. Do not advance the pin
without rebasing affected patches and rerunning contract/build/tests.

Generate future phase patches from an isolated clean-pin replay after the
compatibility and hardening steps. Include reviewed staged additions in the
diff so new source/tests are not lost. Produce UTF-8/LF, not PowerShell 5.1
redirection that may create UTF-16. Register each patch in the correct manifest
phase. A scratch checkout or untracked diff is never a release artifact.

Fantasy changes follow their separately approved upstream/pin workflow. A
local `replace` is not a release solution. Only the optional cache experiment
is default-off; do not label all correctness fixes as optional/default-off.
