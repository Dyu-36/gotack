//go:build windows

package fileopen

import (
	"fmt"
	"os/exec"

	"golang.org/x/sys/windows"
)

func Open(path string) error {
	verb, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	file, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	if err := windows.ShellExecute(0, verb, file, nil, nil, windows.SW_SHOWNORMAL); err != nil {
		return fmt.Errorf("open generated file: %w", err)
	}
	return nil
}
func Reveal(path string) error {
	if err := exec.Command("explorer.exe", "/select,", path).Start(); err != nil {
		return fmt.Errorf("reveal generated file: %w", err)
	}
	return nil
}
