//go:build windows

package engine

import (
	"os/exec"
	"testing"
)

func TestConfigureProcAttrGivesEngineAHiddenConsole(t *testing.T) {
	cmd := exec.Command("crush", "server")
	configureProcAttr(cmd)

	if cmd.SysProcAttr == nil {
		t.Fatal("configureProcAttr left SysProcAttr nil")
	}
	if !cmd.SysProcAttr.HideWindow {
		t.Error("HideWindow must be true so the engine console stays invisible")
	}
	if cmd.SysProcAttr.CreationFlags&createNewConsole == 0 {
		t.Errorf("CreationFlags = %#x, want the CREATE_NEW_CONSOLE bit set", cmd.SysProcAttr.CreationFlags)
	}
	if cmd.SysProcAttr.CreationFlags&detachedProcess != 0 {
		t.Errorf("CreationFlags = %#x: DETACHED_PROCESS makes every grandchild pop a console", cmd.SysProcAttr.CreationFlags)
	}
}
