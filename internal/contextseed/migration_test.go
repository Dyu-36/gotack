package contextseed

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeContextFixture(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func layeredSource(t *testing.T, stockText string) string {
	t.Helper()
	dir := t.TempDir()
	writeContextFixture(t, filepath.Join(dir, managedCoreName), "managed-core")
	writeContextFixture(t, filepath.Join(dir, userContextName), "default-user")
	basePath := filepath.Join("legacy", "TACK-v1.md")
	writeContextFixture(t, filepath.Join(dir, basePath), stockText)
	manifest := stockManifest{Version: 1, LegacyStocks: []legacyStock{{SHA256: bytesSHA256([]byte(stockText)), Path: basePath}}}
	data, err := json.Marshal(manifest)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, stockManifestName), data, 0o644))
	return dir
}

func TestLayeredSeedFreshInstallIsDurableAndIdempotent(t *testing.T) {
	dataDir := t.TempDir()
	seeder := New(dataDir, nil)
	source := layeredSource(t, "legacy-stock")
	require.NoError(t, seeder.Seed(source))
	first := seeder.MigrationStatus()
	require.Equal(t, MigrationCommitted, first.Mode)
	require.NotZero(t, first.Generation)
	require.Equal(t, "managed", seeder.SnapshotOwner())
	require.NoFileExists(t, filepath.Join(seeder.ContextDir(), legacyContextName))
	require.NoError(t, New(dataDir, nil).Seed(source))
	require.Equal(t, first.Generation, New(dataDir, nil).MigrationStatus().Generation)
}

func TestStockLegacyAutoMigratesAndRollbackSurvivesRestart(t *testing.T) {
	dataDir := t.TempDir()
	seeder := New(dataDir, nil)
	require.NoError(t, os.MkdirAll(seeder.ContextDir(), 0o755))
	writeContextFixture(t, filepath.Join(seeder.ContextDir(), legacyContextName), "legacy-stock")
	source := layeredSource(t, "legacy-stock")
	require.NoError(t, seeder.Seed(source))
	status := seeder.MigrationStatus()
	require.Equal(t, MigrationCommitted, status.Mode)
	require.NotEmpty(t, status.BackupToken)
	rolled, err := seeder.RollbackMigration(RollbackMigrationRequest{ExpectedGeneration: status.Generation, Token: status.BackupToken})
	require.NoError(t, err)
	require.Equal(t, MigrationRolledBack, rolled.Mode)
	require.Equal(t, "legacy-stock", readSeeded(t, seeder, legacyContextName))
	restarted := New(dataDir, nil)
	require.NoError(t, restarted.Seed(source))
	require.Equal(t, MigrationRolledBack, restarted.MigrationStatus().Mode)
	require.Equal(t, "legacy", restarted.SnapshotOwner())
}

func TestModifiedUnknownLegacyRequiresManualCAS(t *testing.T) {
	seeder := New(t.TempDir(), nil)
	require.NoError(t, os.MkdirAll(seeder.ContextDir(), 0o755))
	legacyPath := filepath.Join(seeder.ContextDir(), legacyContextName)
	writeContextFixture(t, legacyPath, "unknown custom legacy")
	require.NoError(t, seeder.Seed(layeredSource(t, "known stock")))
	preview, err := seeder.PreviewMigration()
	require.NoError(t, err)
	require.Equal(t, MigrationPending, preview.Status.Mode)
	require.False(t, preview.BaseKnown)
	require.True(t, preview.RequiresResolution)
	writeContextFixture(t, legacyPath, "concurrent edit")
	_, err = seeder.AcceptMigration(AcceptMigrationRequest{ExpectedGeneration: preview.Status.Generation, ExpectedLegacyHash: preview.Status.LegacyHash, ExpectedUserHash: preview.Status.UserHash, ExpectedCoreHash: preview.Status.CoreHash, ResolvedUser: "reviewed"})
	require.ErrorIs(t, err, ErrMigrationConflict)
	require.Equal(t, "concurrent edit", readSeeded(t, seeder, legacyContextName))
}

func TestKnownBasePreviewAcceptRestartAndRollback(t *testing.T) {
	dataDir := t.TempDir()
	seeder := New(dataDir, nil)
	require.NoError(t, os.MkdirAll(seeder.ContextDir(), 0o755))
	base := "stock base"
	writeContextFixture(t, filepath.Join(seeder.ContextDir(), legacyContextName), base+"\nuser edit")
	report := fmt.Sprintf(`{"files":{"TACK.md":%d},"hashes":{"TACK.md":%q}}`, len(base), bytesSHA256([]byte(base)))
	writeContextFixture(t, filepath.Join(seeder.ContextDir(), ".seed-report.json"), report)
	source := layeredSource(t, base)
	require.NoError(t, seeder.Seed(source))
	preview, err := seeder.PreviewMigration()
	require.NoError(t, err)
	require.True(t, preview.BaseKnown)
	require.True(t, preview.HasConflicts)
	_, err = seeder.AcceptMigration(AcceptMigrationRequest{ExpectedGeneration: preview.Status.Generation, ExpectedLegacyHash: preview.Status.LegacyHash, ExpectedUserHash: preview.Status.UserHash, ExpectedCoreHash: preview.Status.CoreHash, ResolvedUser: preview.CandidateUser})
	require.ErrorContains(t, err, "resolve all")
	accepted, err := seeder.AcceptMigration(AcceptMigrationRequest{ExpectedGeneration: preview.Status.Generation, ExpectedLegacyHash: preview.Status.LegacyHash, ExpectedUserHash: preview.Status.UserHash, ExpectedCoreHash: preview.Status.CoreHash, ResolvedUser: "reviewed user rules"})
	require.NoError(t, err)
	require.Equal(t, MigrationCommitted, accepted.Mode)
	restarted := New(dataDir, nil)
	require.NoError(t, restarted.Seed(source))
	require.Equal(t, "reviewed user rules", readSeeded(t, restarted, userContextName))
	snapshot, err := restarted.BuildPromptSnapshot()
	require.NoError(t, err)
	require.NoFileExists(t, filepath.Join(snapshot, legacyContextName))
	require.FileExists(t, filepath.Join(snapshot, managedCoreName))
	rolled, err := restarted.RollbackMigration(RollbackMigrationRequest{ExpectedGeneration: restarted.MigrationStatus().Generation, Token: accepted.BackupToken})
	require.NoError(t, err)
	require.Equal(t, MigrationRolledBack, rolled.Mode)
}

func TestInterruptedStagedCommitRecoversOnRestart(t *testing.T) {
	dataDir := t.TempDir()
	seeder := New(dataDir, nil)
	require.NoError(t, os.MkdirAll(seeder.ContextDir(), 0o755))
	legacy := []byte("legacy-stock")
	writeContextFixture(t, filepath.Join(seeder.ContextDir(), legacyContextName), string(legacy))
	stage := migrationStage{Token: "migration-recovery", PreviousMode: MigrationLegacy, ExpectedLegacyHash: bytesSHA256(legacy), TargetCoreHash: bytesSHA256([]byte("managed-core")), TargetUserHash: bytesSHA256([]byte("default-user"))}
	require.NoError(t, os.MkdirAll(seeder.stageDir(stage.Token), 0o700))
	writeContextFixture(t, filepath.Join(seeder.stageDir(stage.Token), managedCoreName), "managed-core")
	writeContextFixture(t, filepath.Join(seeder.stageDir(stage.Token), userContextName), "default-user")
	require.NoError(t, seeder.saveMigrationStatus(MigrationStatus{Mode: MigrationStaged, Version: 1, Generation: 2, LegacyHash: stage.ExpectedLegacyHash, Stage: &stage}))
	restarted := New(dataDir, nil)
	require.NoError(t, restarted.Seed(layeredSource(t, string(legacy))))
	status := restarted.MigrationStatus()
	require.Equal(t, MigrationCommitted, status.Mode)
	require.FileExists(t, filepath.Join(restarted.backupDir(), status.BackupToken, legacyContextName))
	require.NoFileExists(t, filepath.Join(restarted.ContextDir(), legacyContextName))
}

func TestRollbackRejectsUnissuedToken(t *testing.T) {
	seeder := New(t.TempDir(), nil)
	_, err := seeder.RollbackMigration(RollbackMigrationRequest{Token: "some-file", ExpectedGeneration: 0})
	require.True(t, errors.Is(err, ErrMigrationConflict))
}

func TestManagedCoreUpdatesButUserContextIsPreserved(t *testing.T) {
	seeder := New(t.TempDir(), nil)
	source := layeredSource(t, "stock")
	require.NoError(t, seeder.Seed(source))
	writeContextFixture(t, filepath.Join(seeder.ContextDir(), userContextName), "my preferences")
	writeContextFixture(t, filepath.Join(source, managedCoreName), "managed-core-v2")
	require.NoError(t, seeder.Seed(source))
	require.Equal(t, "my preferences", readSeeded(t, seeder, userContextName))
	require.Equal(t, "managed-core-v2", readSeeded(t, seeder, managedCoreName))
}
