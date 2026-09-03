//go:build !windows

package zalo

import "os/exec"

func hideConsoleWindow(cmd *exec.Cmd) {}
