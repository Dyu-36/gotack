package main

import (
	"path/filepath"

	"github.com/Dyu-36/gotack/internal/appconfig"
	workspaceconfig "github.com/Dyu-36/gotack/internal/workspaceconfig"
)

func (a *App) workspaceRuntimeManager() *workspaceconfig.Manager {
	a.workspaceRuntime = workspaceconfig.NewManager(workspaceconfig.Options{
		Log:             a.log,
		Office:          a.officeSeeder,
		Context:         a.ensureContextRegistrar(),
		UserSkillsDir:   userSkillsDir(),
		RecallIndexRoot: filepath.Join(appconfig.Dir(), "recall"),
		Resolvers: workspaceconfig.Resolvers{
			Memory: func() string { return resolveMemoryCommand() },
			Skills: func() string { return resolveSkillsCommand() },
			Recall: func() string { return resolveRecallCommand() },
			Guard:  func() string { return resolveGuardCommand() },
		},
	})
	return a.workspaceRuntime
}
