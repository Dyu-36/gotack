package workspaceconfig

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Dyu-36/gotack/internal/engineapi"
	"github.com/Dyu-36/gotack/internal/workspace"
	"github.com/Dyu-36/gotack/resources"
)

type Options struct {
	Log           *slog.Logger
	UserSkillsDir string
	ManagedRoot   string
}

type Manager struct {
	log           *slog.Logger
	userSkillsDir string
	managedRoot   string
}

func NewManager(options Options) *Manager {
	return &Manager{
		log:           options.Log,
		userSkillsDir: options.UserSkillsDir,
		managedRoot:   options.ManagedRoot,
	}
}

func (m *Manager) Apply(ctx context.Context, api *engineapi.Client, desc workspace.Descriptor) error {
	if m == nil || api == nil || desc.WorkspaceID == "" {
		return nil
	}
	if err := removeLegacyTools(ctx, api, desc.WorkspaceID, m.managedRoot); err != nil {
		m.warn("workspace runtime: legacy tool cleanup failed", "err", err)
		return fmt.Errorf("workspace runtime: legacy tool cleanup: %w", err)
	}
	bundledDir, err := resources.InstallSkills(m.managedRoot)
	if err != nil {
		return fmt.Errorf("workspace runtime: install bundled skills: %w", err)
	}
	if err := RegisterSkillsPaths(ctx, api, desc.WorkspaceID, desc, m.userSkillsDir, bundledDir); err != nil {
		m.warn("workspace runtime: skills registration failed", "err", err)
		return fmt.Errorf("workspace runtime: skills registration: %w", err)
	}
	return nil
}

func (m *Manager) warn(message string, args ...any) {
	if m != nil && m.log != nil {
		m.log.Warn(message, args...)
	}
}
