package main

import (
	"path/filepath"

	"github.com/Dyu-36/gotack/internal/appconfig"
	workspaceconfig "github.com/Dyu-36/gotack/internal/workspaceconfig"
)

func userSkillsDir() string {
	return filepath.Join(appconfig.Dir(), "skills")
}

func (a *App) workspaceRuntimeManager() *workspaceconfig.Manager {
	a.workspaceRuntimeOnce.Do(func() {
		a.workspaceRuntime = workspaceconfig.NewManager(workspaceconfig.Options{
			Log:           a.log,
			UserSkillsDir: userSkillsDir(),
			ManagedRoot:   appconfig.Dir(),
		})
	})
	return a.workspaceRuntime
}
