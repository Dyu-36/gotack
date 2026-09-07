package workspaceconfig

import (
	"context"
	"log/slog"

	"github.com/Dyu-36/gotack/internal/contextseed"
	"github.com/Dyu-36/gotack/internal/crushapi"
	"github.com/Dyu-36/gotack/internal/workspace"
)

type Resolvers struct {
	Memory func() string
	Skills func() string
	Recall func() string
	Guard  func() string
}

type Options struct {
	Log             *slog.Logger
	Office          OfficeRuntime
	Context         *contextseed.Registrar
	UserSkillsDir   string
	RecallIndexRoot string
	Resolvers       Resolvers
}

type Manager struct {
	log             *slog.Logger
	office          OfficeRuntime
	context         *contextseed.Registrar
	userSkillsDir   string
	recallIndexRoot string
	resolvers       Resolvers
}

func NewManager(options Options) *Manager {
	return &Manager{
		log:             options.Log,
		office:          options.Office,
		context:         options.Context,
		userSkillsDir:   options.UserSkillsDir,
		recallIndexRoot: options.RecallIndexRoot,
		resolvers:       options.Resolvers,
	}
}

func (m *Manager) Apply(ctx context.Context, api *crushapi.Client, desc workspace.Descriptor) {
	if m == nil || api == nil || desc.WorkspaceID == "" {
		return
	}

	if err := RegisterOffice(ctx, api, desc.WorkspaceID, desc, m.office, m.userSkillsDir); err != nil {
		m.warn("workspace runtime: office registration failed", "err", err)
	}
	if err := RegisterMemory(ctx, api, desc.WorkspaceID, resolve(m.resolvers.Memory)); err != nil {
		m.warn("workspace runtime: memory registration failed", "err", err)
	}
	if err := RegisterSkills(ctx, api, desc.WorkspaceID, resolve(m.resolvers.Skills), m.userSkillsDir); err != nil {
		m.warn("workspace runtime: skills registration failed", "err", err)
	}
	if err := RegisterRecall(ctx, api, desc.WorkspaceID, desc, resolve(m.resolvers.Recall), m.recallIndexRoot); err != nil {
		m.warn("workspace runtime: recall registration failed", "err", err)
	}
	if m.context != nil {
		m.context.Register(ctx, api, desc.WorkspaceID)
	}
	if err := RegisterGuard(ctx, api, desc.WorkspaceID, resolve(m.resolvers.Guard)); err != nil {
		m.warn("workspace runtime: guard registration failed", "err", err)
	}
}

func resolve(resolver func() string) string {
	if resolver == nil {
		return ""
	}
	return resolver()
}

func (m *Manager) warn(message string, args ...any) {
	if m != nil && m.log != nil {
		m.log.Warn(message, args...)
	}
}
