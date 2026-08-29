package engine

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// defaultBinary resolves the distribution strategy used by Gotack releases:
// a bundled Crush binary beside the desktop executable is preferred, while an
// externally installed `crush` on PATH remains a development/fallback option.
// An explicit EngineBinary setting bypasses this resolver entirely.
func defaultBinary() string {
	name := "crush"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}

	if executable, err := os.Executable(); err == nil {
		root := filepath.Dir(executable)
		for _, candidate := range []string{
			filepath.Join(root, "resources", name),
			filepath.Join(root, name),
		} {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate
			}
		}
	}

	if found, err := exec.LookPath("crush"); err == nil {
		return found
	}
	// Preserve the useful exec error from Start when neither source exists.
	return "crush"
}
