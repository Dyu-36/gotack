package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBundledPythonEnvironment(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "gotack.exe")
	original := []string{"PATH=project-python", "GOTACK_PYTHON=old"}
	absent := bundledPythonEnvironment(original, executable)
	if strings.Join(absent, "|") != strings.Join(original, "|") {
		t.Fatal("development environment changed")
	}
	python := filepath.Join(root, "resources", "python", "python.exe")
	if err := os.MkdirAll(filepath.Dir(python), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(python, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	got := bundledPythonEnvironment(original, executable)
	if len(got) != 2 || got[0] != "PATH=project-python" || got[1] != "GOTACK_PYTHON="+python {
		t.Fatalf("unexpected environment: %v", got)
	}
	if original[1] != "GOTACK_PYTHON=old" {
		t.Fatal("input environment mutated")
	}
}
