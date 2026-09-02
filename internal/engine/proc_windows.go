//go:build windows

package engine

import (
	"os/exec"
	"strconv"
	"syscall"
)

const (
	detachedProcess  = 0x00000008
	createNewConsole = 0x00000010
	createNoWindow   = 0x08000000
)

func configureProcAttr(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}

	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= createNewConsole
}

func killTree(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	pid := cmd.Process.Pid
	tk := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid))

	tk.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
	if err := tk.Run(); err != nil {

		return cmd.Process.Kill()
	}
	return nil
}
