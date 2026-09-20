package runmetrics

import (
	"os"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnsureKeyIsStableAndPrivate(t *testing.T) {
	dir := t.TempDir()
	first, err := EnsureKey(dir)
	require.NoError(t, err)
	second, err := EnsureKey(dir)
	require.NoError(t, err)
	require.Equal(t, first, second)
	content, err := os.ReadFile(first)
	require.NoError(t, err)
	require.Len(t, content, 32)
	info, err := os.Stat(first)
	require.NoError(t, err)
	if runtime.GOOS != "windows" {
		require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	}
}

func TestEnsureKeyEmptyDir(t *testing.T) {
	_, err := EnsureKey("")
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty")
}
