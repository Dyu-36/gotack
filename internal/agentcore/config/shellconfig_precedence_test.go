//go:build gotacktest

package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Dyu-36/gotack/internal/agentcore/config"
	"github.com/stretchr/testify/require"
)

// TestShellConfigDotTackrcTakesPrecedence verifies that a project-local
// .tackrc overrides tackrc in the same directory on conflicting settings.
func TestShellConfigDotTackrcTakesPrecedence(t *testing.T) {
	isolated := t.TempDir()
	t.Setenv("HOME", isolated)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(isolated, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(isolated, ".local", "share"))

	workDir := t.TempDir()
	dataDir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(workDir, "tackrc"),
		[]byte("option notifications bell\n"), 0o644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(workDir, ".tackrc"),
		[]byte("option notifications osc\n"), 0o644,
	))

	store, err := config.Load(workDir, dataDir, false)
	require.NoError(t, err)
	require.Equal(t, "osc", store.Config().Options.Notifications,
		".tackrc should win over tackrc")
}
