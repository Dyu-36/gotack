//go:build !windows

package engine

import "os/exec"

func configureProcAttr(*exec.Cmd) {}

func killTree(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}
