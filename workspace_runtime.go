package main

import (
	"path/filepath"

	"github.com/Dyu-36/gotack/internal/appconfig"
	workspaceconfig "github.com/Dyu-36/gotack/internal/workspaceconfig"
)

func (a *App) workspaceRuntimeManager() *workspaceconfig.Manager {
	a.workspaceRuntimeOnce.Do(func() {
		a.workspaceRuntime = workspaceconfig.NewManager(workspaceconfig.Options{
			Log:             a.log,
			Office:          a.officeSeeder,
			Context:         a.ensureContextRegistrar(),
			UserSkillsDir:   userSkillsDir(),
			RecallIndexRoot: filepath.Join(appconfig.Dir(), "recall"),
			Resolvers: workspaceconfig.Resolvers{
				Memory: resolveMemoryCommand,
				Skills: resolveSkillsCommand,
				Recall: resolveRecallCommand,
				Guard:  resolveGuardCommand,
			},
		})
	})
	return a.workspaceRuntime
}
