//go:build windows

package zalo

import (
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

// hideConsoleWindow keeps helper processes like the PowerShell screenshot
// capture from flashing a console window over the user's desktop.
func hideConsoleWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
}
