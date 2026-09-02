package main

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/crushapi"
	"github.com/Dyu-36/gotack/internal/officecli"
)

const officeMCPName = "gotack-office"

func officeBinaryName() string {
	if runtime.GOOS == "windows" {
		return "officecli.exe"
	}
	return "officecli"
}

func resolveOfficeCommand() string {
	name := officeBinaryName()
	if executable, err := os.Executable(); err == nil {
		root := filepath.Dir(executable)
		for _, candidate := range []string{
			filepath.Join(root, "resources", name),
			filepath.Join(root, name),
		} {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate
			}
		}
	}
	if found, err := exec.LookPath(name); err == nil {
		return found
	}
	return ""
}

type officeSeeder struct {
	seeder *officecli.Seeder
	log    *slog.Logger
}

func newOfficeSeeder(log *slog.Logger) *officeSeeder {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &officeSeeder{
		seeder: officecli.New(appconfig.Dir(), log),
		log:    log,
	}
}

func (s *officeSeeder) resolveOfficeSourceDir() string {
	if executable, err := os.Executable(); err == nil {
		root := filepath.Dir(executable)
		for _, candidate := range []string{
			filepath.Join(root, "resources"),
			filepath.Join(root, "..", "resources"),
			root,
		} {
			if info, err := os.Stat(filepath.Join(candidate, officeBinaryName())); err == nil && !info.IsDir() {
				return candidate
			}
		}
	}
	return ""
}

func (s *officeSeeder) startup() {
	source := s.resolveOfficeSourceDir()
	if source == "" {
		s.log.Debug("office: bundled resources not found, falling back to system install")
	} else if err := s.seeder.Seed(source); err != nil {
		s.log.Warn("office: failed to seed bundled resources", "err", err)
	}
	s.seeder.InstallPath()
}

func (s *officeSeeder) CrushEnv() map[string]string {
	return s.seeder.CrushEnv()
}

func (s *officeSeeder) SkillsPath() string {
	return s.seeder.SkillsPathArg()
}

func (a *App) ensureOfficeSeed() {
	if a.officeSeeder == nil {
		return
	}
	a.officeSeeder.startup()
}

func (a *App) registerOfficeTools(workspaceID string) {
	if a.officeSeeder == nil {
		return
	}
	svc, err := a.services()
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Second)
	defer cancel()

	command := resolveOfficeCommand()
	if command == "" {
		command = officecli.EnsureOfficeCLIOnPath()
	}
	if command == "" {
		_ = svc.api.RemoveConfigField(ctx, workspaceID, crushapi.ConfigScopeWorkspace, "mcp_servers."+officeMCPName)
	} else {
		entry := map[string]any{"command": command, "type": "stdio", "timeout": 30}
		if err := svc.api.SetConfigField(ctx, workspaceID, crushapi.ConfigScopeWorkspace, "mcp_servers."+officeMCPName, entry); err != nil {
			if a.log != nil {
				a.log.Warn("office tools registration failed", "err", err)
			}
		}
	}

	env := a.officeSeeder.CrushEnv()
	skillsPath := a.officeSeeder.SkillsPath()

	additions := make([]string, 0, 3)
	if skillsPath != "" {
		additions = append(additions, skillsPath)
	}
	additions = append(additions, userSkillsDir())
	if desc, ok := svc.ws.Current(); ok && desc.Path != "" {
		additions = append(additions, projectSkillsDir(desc.Path))
	}
	if len(env) == 0 && len(additions) == 0 {
		return
	}

	current, err := svc.api.GetWorkspaceConfig(ctx, workspaceID)
	if err != nil {
		if a.log != nil {
			a.log.Warn("office runtime config read failed; skipping merge", "err", err)
		}
		return
	}
	fields := make(map[string]any, 2)
	if len(env) > 0 {
		fields["env"] = mergeConfigEnv(current.Env, env)
	}
	if len(additions) > 0 {
		fields["options.skills_paths"] = mergeSkillsPaths(current.SkillsPaths(), additions...)
	}
	if err := svc.api.SetConfigFields(ctx, workspaceID, crushapi.ConfigScopeWorkspace, fields); err != nil && a.log != nil {
		a.log.Warn("office runtime config registration failed", "err", err)
	}
}

func userSkillsDir() string {
	return filepath.Join(appconfig.Dir(), "skills")
}

func projectSkillsDir(workspacePath string) string {
	return filepath.Join(workspacePath, ".agents", "skills")
}

func mergeConfigEnv(existing, additions map[string]string) map[string]string {
	merged := make(map[string]string, len(existing)+len(additions))
	for key, value := range existing {
		merged[key] = value
	}
	for key, value := range additions {
		merged[key] = value
	}
	return merged
}

func mergeSkillsPaths(existing []string, additions ...string) []string {
	merged := existing
	for _, add := range additions {
		if add == "" {
			continue
		}
		present := false
		for _, path := range merged {
			if path == add {
				present = true
				break
			}
		}
		if !present {
			merged = append(merged, add)
		}
	}
	return merged
}
