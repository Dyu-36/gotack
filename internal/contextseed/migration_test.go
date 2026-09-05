package contextseed

import (
	"crypto/sha256"
	"encoding/hex"
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

func layeredSource(t *testing.T, legacyStock string) string {
	t.Helper()
	dir := t.TempDir()
	writeContextFixture(t, filepath.Join(dir, managedCoreName), "managed-core")
	writeContextFixture(t, filepath.Join(dir, userContextName), "default-user")
	digest := sha256.Sum256([]byte(legacyStock))
	manifest := fmt.Sprintf(`{"version":1,"legacy_stock_sha256":[%q]}`, hex.EncodeToString(digest[:]))
	writeContextFixture(t, filepath.Join(dir, stockManifestName), manifest)
	return dir
}
func TestLayeredSeedFreshInstall(t *testing.T) {
	seeder := New(t.TempDir(), nil)
	require.NoError(t, seeder.Seed(layeredSource(t, "legacy-stock")))
	require.Equal(t, MigrationLayered, seeder.MigrationStatus().Mode)
	require.Equal(t, "managed", seeder.SnapshotOwner())
	require.FileExists(t, filepath.Join(seeder.ContextDir(), managedCoreName))
	require.FileExists(t, filepath.Join(seeder.ContextDir(), userContextName))
	_, err := os.Stat(filepath.Join(seeder.ContextDir(), legacyContextName))
	require.True(t, os.IsNotExist(err))
}

func TestStockLegacyAutoMigratesAndRollsBack(t *testing.T) {
	seeder := New(t.TempDir(), nil)
	require.NoError(t, os.MkdirAll(seeder.ContextDir(), 0o755))
	legacy := "legacy-stock"
	writeContextFixture(t, filepath.Join(seeder.ContextDir(), legacyContextName), legacy)

	require.NoError(t, seeder.Seed(layeredSource(t, legacy)))
	status := seeder.MigrationStatus()
	require.Equal(t, MigrationLayered, status.Mode)
	require.NotEmpty(t, status.BackupToken)
	_, err := os.Stat(filepath.Join(seeder.ContextDir(), legacyContextName))
	require.True(t, os.IsNotExist(err))

	rolledBack, err := seeder.RollbackMigration(status.BackupToken)
	require.NoError(t, err)
	require.Equal(t, MigrationLegacy, rolledBack.Mode)
	content, err := os.ReadFile(filepath.Join(seeder.ContextDir(), legacyContextName))
	require.NoError(t, err)
	require.Equal(t, legacy, string(content))
}
func TestModifiedLegacyRemainsSingleOwnerUntilExplicitAccept(t *testing.T) {
	seeder := New(t.TempDir(), nil)
	require.NoError(t, os.MkdirAll(seeder.ContextDir(), 0o755))
	writeContextFixture(t, filepath.Join(seeder.ContextDir(), legacyContextName), "custom legacy")

	require.NoError(t, seeder.Seed(layeredSource(t, "different stock")))
	require.Equal(t, MigrationPending, seeder.MigrationStatus().Mode)
	require.Equal(t, "legacy", seeder.SnapshotOwner())

	legacySnapshot, err := seeder.BuildPromptSnapshot()
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(legacySnapshot, legacyContextName))
	require.NoFileExists(t, filepath.Join(legacySnapshot, managedCoreName))
	require.NoFileExists(t, filepath.Join(legacySnapshot, userContextName))

	accepted, err := seeder.AcceptMigration("reviewed user rules")
	require.NoError(t, err)
	require.Equal(t, MigrationLayered, accepted.Mode)
	layeredSnapshot, err := seeder.BuildPromptSnapshot()
	require.NoError(t, err)
	require.NoFileExists(t, filepath.Join(layeredSnapshot, legacyContextName))
	require.FileExists(t, filepath.Join(layeredSnapshot, managedCoreName))
	userBytes, err := os.ReadFile(filepath.Join(layeredSnapshot, userContextName))
	require.NoError(t, err)
	require.Equal(t, "reviewed user rules", string(userBytes))

	preview, err := seeder.PreviewMigration()
	require.NoError(t, err)
	require.Empty(t, preview.Legacy)
	require.Equal(t, "reviewed user rules", preview.UserContext)
}
