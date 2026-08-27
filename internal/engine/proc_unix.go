//go:build !windows

package engine

import "os/exec"

// configureProcAttr applies OS-specific process attributes. On non-Windows
// there is nothing to set; it exists only so the Windows helper has a
// same-signature counterpart.
func configureProcAttr(*exec.Cmd) {}

// killTree terminates a child process. On non-Windows this is a direct Kill:
// process-group kill needs pgid + syscall.Kill glue that is intentionally left
// out to keep the host small, so descendants are not reaped here.
func killTree(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}
