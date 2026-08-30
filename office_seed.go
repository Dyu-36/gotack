package main

// office_seed.go -- role: bootstrap the bundled command-line runtimes and the
// matching Crush skill files so the agent gains Office and timetable tools
// without any user setup.

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

// resolveOfficeCommand prefers the bundled officecli binary, falling back to
// any system PATH install. An empty result tells the host to skip MCP
// registration so the user is not misled by a half-configured workspace.
func resolveOfficeCommand() string {
	name := "officecli"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
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

// officeSeeder wraps the officecli seeder and locates the bundled resources
// shipped next to the desktop executable.
type officeSeeder struct {
	seeder    *officecli.Seeder
	sourceDir string
	log       *slog.Logger
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

// resolveOfficeSourceDir locates the bundled resources next to the running
// executable. An empty result disables seeding; the desktop still benefits
// from any system-installed officecli.
func (s *officeSeeder) resolveOfficeSourceDir() string {
	if executable, err := os.Executable(); err == nil {
		root := filepath.Dir(executable)
		for _, candidate := range []string{
			filepath.Join(root, "resources"),
			filepath.Join(root, "..", "resources"),
			root,
		} {
			if info, err := os.Stat(filepath.Join(candidate, "officecli.exe")); err == nil && !info.IsDir() {
				return candidate
			}
		}
	}
	return ""
}

// startup copies the bundled resources into the per-user data dir and makes
// officecli and the timetable Python runtime available on PATH for Crush.
func (s *officeSeeder) startup() {
	source := s.resolveOfficeSourceDir()
	if source == "" {
		s.log.Debug("office: bundled resources not found, falling back to system install")
	} else if err := s.seeder.Seed(source); err != nil {
		s.log.Warn("office: failed to seed bundled resources", "err", err)
	} else {
		s.sourceDir = source
	}
	s.seeder.InstallPath()
}

// CrushEnv returns the env map that must be applied to every workspace so the
// agent's shell resolves the bundled officecli binary.
func (s *officeSeeder) CrushEnv() map[string]string {
	return s.seeder.CrushEnv()
}

// SkillsPath returns the path Crush should add to options.skills_paths.
func (s *officeSeeder) SkillsPath() string {
	return s.seeder.SkillsPathArg()
}

// ensureOfficeSeed is called once during startup.
func (a *App) ensureOfficeSeed() {
	if a.officeSeeder == nil {
		return
	}
	a.officeSeeder.startup()
}

// registerOfficeTools wires the bundled office MCP server, the shared skills
// path and the PATH override into the active workspace configuration. The
// timetable skill uses the Python runtime from the same PATH.
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
	if len(env) > 0 {
		if err := svc.api.SetConfigField(ctx, workspaceID, crushapi.ConfigScopeWorkspace, "env", env); err != nil && a.log != nil {
			a.log.Warn("office env registration failed", "err", err)
		}
	}

	skillsPath := a.officeSeeder.SkillsPath()
	if skillsPath == "" {
		return
	}
	merged := []string{skillsPath}
	if err := svc.api.SetConfigField(ctx, workspaceID, crushapi.ConfigScopeWorkspace, "options.skills_paths", merged); err != nil && a.log != nil {
		a.log.Warn("office skills path registration failed", "err", err)
	}
}
