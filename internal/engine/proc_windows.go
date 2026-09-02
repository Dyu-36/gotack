//go:build windows

package engine

import (
	"os/exec"
	"strconv"
	"syscall"
)

// Windows process creation flags.
//
// DETACHED_PROCESS gives the engine no console, which makes console
// descendants allocate their own windows. CREATE_NEW_CONSOLE combined with
// HideWindow gives the engine one invisible console that descendants inherit.
const (
	detachedProcess  = 0x00000008 // never use: no console -> children allocate their own
	createNewConsole = 0x00000010 // CREATE_NEW_CONSOLE
	createNoWindow   = 0x08000000 // CREATE_NO_WINDOW
)

// configureProcAttr gives the spawned engine a hidden console so the user
// never sees a stray terminal, and so nothing it spawns has to create one.
func configureProcAttr(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	// STARTF_USESHOWWINDOW + SW_HIDE
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= createNewConsole
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
	// taskkill is itself a console program: without CREATE_NO_WINDOW it pops
	// its own window every time the engine is stopped.
	tk.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
	if err := tk.Run(); err != nil {
		// Last-resort: a direct Kill still terminates the immediate child even
		// if taskkill is missing (e.g. minimal Nano server images).
		return cmd.Process.Kill()
	}
	return nil
}
