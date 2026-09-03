package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/bundleseed"
	"github.com/Dyu-36/gotack/internal/crushapi"
	"github.com/Dyu-36/gotack/internal/officecli"
)

const legacyOfficeMCPName = "gotack-office"

func officeBinaryName() string {
	if runtime.GOOS == "windows" {
		return "officecli.exe"
	}
	return "officecli"
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
func (s *officeSeeder) resolveOfficeSkillsSourceDir() string {
	if executable, err := os.Executable(); err == nil {
		root := filepath.Dir(executable)
		for _, candidate := range []string{
			filepath.Join(root, "resources", "skills"),
			filepath.Join(root, "..", "resources", "skills"),
			filepath.Join(root, "..", "..", "resources", "skills"),
		} {
			if info, err := os.Stat(candidate); err == nil && info.IsDir() {
				return filepath.Clean(candidate)
			}
		}
	}
	return ""
}

func (s *officeSeeder) startup() {
	source := s.resolveOfficeSourceDir()
	if source == "" {
		s.log.Debug("office: bundled executable resources not found, falling back to system install")
		if skillsSource := s.resolveOfficeSkillsSourceDir(); skillsSource != "" {
			if err := bundleseed.CopyIfChanged(skillsSource, filepath.Join(appconfig.Dir(), "skills"), bundleseed.Options{ExistingFiles: bundleseed.ManagedFiles}); err != nil {
				s.log.Warn("office: failed to seed bundled skills", "err", err)
			}
		}
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

func (a *App) registerOfficeRuntime(workspaceID string) {
	if a.officeSeeder == nil {
		return
	}
	svc, err := a.services()
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Second)
	defer cancel()

	if err := svc.api.RemoveConfigField(ctx, workspaceID, crushapi.ConfigScopeWorkspace, "mcp_servers."+legacyOfficeMCPName); err != nil && a.log != nil {
		a.log.Warn("legacy office MCP cleanup failed", "err", err)
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

func skillPathKey(path string) string {
	key := filepath.Clean(path)
	if runtime.GOOS == "windows" {
		key = strings.ToLower(key)
	}
	return key
}
func mergeSkillsPaths(existing []string, additions ...string) []string {
	merged := make([]string, 0, len(existing)+len(additions))
	seen := make(map[string]struct{}, len(existing)+len(additions))
	appendPath := func(path string) {
		if path == "" {
			return
		}
		key := skillPathKey(path)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		merged = append(merged, path)
	}
	for _, path := range existing {
		appendPath(path)
	}
	for _, path := range additions {
		appendPath(path)
	}
	return merged
}
