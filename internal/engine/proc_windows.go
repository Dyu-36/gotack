//go:build windows

package engine

import (
	"os/exec"
	"strconv"
	"syscall"
)

// configureProcAttr hides the spawned console window so the user never sees a
// stray terminal pop up when gotack autostarts the engine.
func configureProcAttr(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= 0x00000008 // CREATE_NO_WINDOW
}

// killTree terminates the child process and any descendants it spawned. Uses
// taskkill /T /F which is the supported way to walk a process tree on
// Windows without importing golang.org/x/sys/windows.
func killTree(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	pid := cmd.Process.Pid
	tk := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid))
	tk.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := tk.Run(); err != nil {
		// Last-resort: a direct Kill still terminates the immediate child even
		// if taskkill is missing (e.g. minimal Nano server images).
		return cmd.Process.Kill()
	}
	return nil
}
