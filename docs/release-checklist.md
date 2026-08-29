# Gotack MVP release checklist

## Version and pin

- [ ] Decide the next SemVer and update `wails.json` `Info.productVersion`.
- [ ] Confirm `third_party/README.md` and both workflows use the same pinned Crush commit.
- [ ] Confirm `package.json` / `frontend/package.json` dependency versions and `pnpm-lock.yaml` are in sync.

## Automated gates

- [ ] `go test ./...`
- [ ] `go vet ./...`
- [ ] `pnpm install --frozen-lockfile`
- [ ] `pnpm --filter gotack-frontend check`
- [ ] `pnpm --filter gotack-frontend build`
- [ ] Windows Wails CI artifact is green.

## Windows regression

Use a Windows 10/11 x64 machine representative of the 6 GB RAM target.

- [ ] Start Gotack from the portable directory without Go/Node/pnpm/Wails/Crush installed.
- [ ] If WebView2 is missing, verify the configured Wails WebView2 download path produces a useful install/retry flow.
- [ ] Verify bundled `resources/crush.exe` is selected; separately verify PATH fallback by temporarily removing the bundle.
- [ ] Open a workspace containing Unicode and long paths.
- [ ] Create, switch, rename and delete sessions; restart Gotack and verify workspace/session restoration.
- [ ] Send a prompt and observe SSE text/tool activity; cancel an active prompt.
- [ ] Trigger a real permission request and exercise allow / allow-for-session / deny.
- [ ] Trigger real yes/no, single-choice, multi-choice and free-text questions.
- [ ] Verify Changed Files refreshes after file events and large diffs remain usable.
- [ ] Open terminal; verify resize, Unicode, Ctrl+C, shell exit, workspace switch and app shutdown cleanup.
- [ ] Kill Crush during a session and verify error UI plus reconnect/backoff recovery.
- [ ] Repeat workspace/session switches and reconnects while watching RAM/handles for leaks.

## Release

1. Merge a green release candidate into `main`.
2. Create and push the exact tag `v<Info.productVersion>`; for example `v0.1.0`.
3. `.github/workflows/release.yml` reruns source gates, builds Gotack and the pinned Crush binary, assembles `gotack-vX.Y.Z-windows-amd64.zip`, and publishes the GitHub release.
4. Download that ZIP from the release and run one final clean-machine launch check before announcing it.

MVP packaging strategy is **portable ZIP**. Do not introduce an installer into the MVP gate; an NSIS installer can be added after portable clean-machine behavior is stable.
