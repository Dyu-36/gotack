package main

import workspaceconfig "github.com/Dyu-36/gotack/internal/workspaceconfig"

const skillsMCPName = workspaceconfig.SkillsMCPName

var resolveSkillsCommand = resolveSkillsCommandFromDisk

func skillsBinaryName() string {
	return workspaceconfig.BinaryName("skills")
}

func resolveSkillsCommandFromDisk() string {
	return workspaceconfig.ResolveBinary(skillsBinaryName())
}

func skillsEntry(command, root string) map[string]any {
	return workspaceconfig.SkillsEntry(command, root)
}

func (a *App) registerSkillsTools(workspaceID string) {
	svc, err := a.services()
	if err != nil {
		return
	}
	if err := workspaceconfig.RegisterSkills(a.ctx, svc.api, workspaceID, resolveSkillsCommand(), userSkillsDir()); err != nil && a.log != nil {
		a.log.Warn("skills registration failed", "err", err)
	}
}
