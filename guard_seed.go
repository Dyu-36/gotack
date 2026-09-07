package main

import workspaceconfig "github.com/Dyu-36/gotack/internal/workspaceconfig"

var resolveGuardCommand = resolveGuardCommandFromDisk

func guardBinaryName() string {
	return workspaceconfig.BinaryName("guard")
}

func resolveGuardCommandFromDisk() string {
	return workspaceconfig.ResolveBinary(guardBinaryName())
}
