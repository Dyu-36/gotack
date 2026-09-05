# Plan: Rebrand .crush to .tack and .crush-pin to .tack-pin

## Motivation
Previously, workspaces used `.crush` as the data directory where conversation history (`crush.db`, `crush.json`, locks) was stored, and the engine commit tracking used `.crush-pin`.
To align with Tack branding and user requirements, all `.crush` data directory references were changed to `.tack`, the pin file was renamed to `.tack-pin`, and legacy `.crush` folders are now automatically migrated (renamed) to `.tack` upon workspace opening.

## Scope & Decisions
1. **Pin File (`.crush-pin` -> `.tack-pin`)**:
   - Renamed tracked file `.crush-pin` to `.tack-pin`.
   - Updated `scripts/check-repository-invariants.mjs` to guard `.tack-pin`.
   - Updated `scripts/update-crush.ps1` to read `.tack-pin`.
   - Updated `.github/workflows/ci.yml` and `.github/workflows/release.yml`.
   - Updated documentation (`README.md`, `third_party/README.md`, `scripts/README.md`, and contracts).

2. **Engine Hardening (`scripts/harden-crush-for-tack.ps1`)**:
   - Changed default data directory in `third_party/crush/internal/config/config.go` from `".crush"` to `".tack"`.
   - Added `".tack"` and `".tackignore"` to `internal/fsext/fileutil.go` and `internal/fsext/ls.go`.
   - Updated `commands.go` and `stats.go` paths to support `.tack`.
   - Hardened `third_party/crush` and rebuilt `tack-engine.exe` into `build/bin/resources/`.

3. **Automatic Workspace Migration**:
   - In `internal/workspace/service.go`, added `MigrateLegacyDataDir` which checks if `<workspace>/.crush` exists and `<workspace>/.tack` does not, automatically renaming `.crush` to `.tack`.
   - Added unit tests in `internal/workspace/service_test.go` verifying migration when `.crush` exists, when `.tack` exists, and when neither exists.

4. **Repository Invariants & Git**:
   - Added `/.tack/` and `/.crush/` to `.gitignore`.
   - Migrated local repo's `.crush` folder to `.tack`.
   - Validated with `node scripts/check-repository-invariants.mjs` and `go test ./...`.

## Progress
- [x] 1. Rename `.crush-pin` to `.tack-pin` and update invariant checker, scripts, workflows, and docs
- [x] 2. Update `scripts/harden-crush-for-tack.ps1` and apply to `third_party/crush`
- [x] 3. Implement `migrateLegacyDataDir` in `internal/workspace` with unit tests
- [x] 4. Update `.gitignore` and migrate local repository's `.crush` -> `.tack`
- [x] 5. Run full test suite & validation (`go test ./...`, invariant check, rebuild engine)
- [x] 6. Move plan to completed
