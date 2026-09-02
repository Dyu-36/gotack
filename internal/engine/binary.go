package engine

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func defaultBinary() string {
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}

	primary := "tack-engine" + ext
	fallback := "crush" + ext

	if executable, err := os.Executable(); err == nil {
		root := filepath.Dir(executable)
		for _, candidate := range []string{
			filepath.Join(root, "resources", primary),
			filepath.Join(root, primary),
			filepath.Join(root, "resources", fallback),
			filepath.Join(root, fallback),
		} {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate
			}
		}
	}

	for _, name := range []string{primary, "tack-engine", fallback, "crush"} {
		if found, err := exec.LookPath(name); err == nil {
			return found
		}
	}

	return primary
}
