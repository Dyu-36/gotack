package engine

import (
	"os"
	"path/filepath"
	"strings"
)

func bundledPythonEnvironment(env []string, executable string) []string {
	python := filepath.Join(filepath.Dir(executable), "resources", "python", "python.exe")
	info, err := os.Stat(python)
	if err != nil || !info.Mode().IsRegular() {
		return env
	}
	result := make([]string, 0, len(env)+1)
	for _, entry := range env {
		name, _, _ := strings.Cut(entry, "=")
		if !strings.EqualFold(name, "GOTACK_PYTHON") {
			result = append(result, entry)
		}
	}
	return append(result, "GOTACK_PYTHON="+python)
}
