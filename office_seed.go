package main

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/Dyu-36/gotack/internal/appconfig"
	"github.com/Dyu-36/gotack/internal/bundleseed"
	"github.com/Dyu-36/gotack/internal/officecli"
	workspaceconfig "github.com/Dyu-36/gotack/internal/workspaceconfig"
)

func officeBinaryName() string {
	return workspaceconfig.BinaryName("officecli")
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

func (s *officeSeeder) EngineEnv() map[string]string {
	return s.seeder.EngineEnv()
}

func (s *officeSeeder) SkillsPath() string {
	return s.seeder.SkillsPathArg()
}

func (a *App) ensureOfficeSeed() {
	if a.officeSeeder != nil {
		a.officeSeeder.startup()
	}
}

func userSkillsDir() string {
	return filepath.Join(appconfig.Dir(), "skills")
}
