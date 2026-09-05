# Fantasy reasoning continuity patch

This directory records the Fantasy-side implementation needed by Gotack PR5.
The patch applies to the signed `v0.41.3` tag at commit
`f06034c7824ffddc4394d4cefa5ed5132a186b1b` and has focused tests for ordered,
encrypted-only, duplicate-ID, and `store=false` replay behavior.

The patch is an authoring and upstream-review artifact. It is deliberately not
wired through a local Go `replace`, module-cache mutation, or private fork.
`manifest.json` therefore marks it `release_eligible: false`. A release build
must pin a publicly fetchable Fantasy commit containing the accepted upstream
change, then remove this authoring-only patch or update its provenance. Creating
that upstream PR changes a remote repository and requires separate owner
authorization.

To verify the patch in an isolated checkout:

```powershell
git init --quiet --template= <fantasy-dir>
git -C <fantasy-dir> fetch --depth=1 https://github.com/charmbracelet/fantasy.git refs/tags/v0.41.3
git -C <fantasy-dir> checkout --detach FETCH_HEAD
git -C <fantasy-dir> apply --check <repo>\third_party\fantasy-patches\responses-reasoning-continuity.patch
git -C <fantasy-dir> apply <repo>\third_party\fantasy-patches\responses-reasoning-continuity.patch
go test ./providers/openai -run 'TestResponsesPromptReplaysEncryptedReasoning|TestValidateReasoningReplay|TestResponsesPromptKeepsEncryptedOnlyAssistantMessage' -count=1
```
