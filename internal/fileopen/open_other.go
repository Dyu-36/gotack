//go:build !windows

package fileopen

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
)

func Open(path string) error {
	command := "xdg-open"
	if runtime.GOOS == "darwin" {
		command = "open"
	}
	if err := exec.Command(command, path).Start(); err != nil {
		return fmt.Errorf("open generated file: %w", err)
	}
	return nil
}

func Reveal(path string) error {
	if runtime.GOOS == "darwin" {
		return exec.Command("open", "-R", path).Start()
	}
	return exec.Command("xdg-open", filepath.Dir(path)).Start()
}
