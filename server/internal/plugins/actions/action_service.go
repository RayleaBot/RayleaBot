package actions

import (
	"context"
	"log/slog"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

type Deps struct {
	CurrentConfig     func() config.Config
	Logger            *slog.Logger
	RedactText        func(string) string
	Permissions       PermissionView
	Plugins           plugins.CatalogView
	PluginConfig      PluginConfigRepository
	PluginFiles       FileStore
	PluginKV          KVRepository
	Secrets           SecretReader
	ThirdParty        ThirdPartyAccountReader
	AccountValidation ThirdPartyAccountValidationRequester
	ThirdPartyResolve ThirdPartyResolver
	Scheduler         SchedulerCreateFunc
	Dispatcher        ConfigChangeDispatcher
	MessageSender     MessageSendFunc
	Renderer          Renderer
	Adapter           OneBotAdapter
	PluginLogLimiter  *PluginLogLimiter
	Governance        any
	RefreshCommands   func(context.Context, string, map[string]any)
	Registrars        []Registrar
	ActionRegistry    *Registry
}

type Service struct {
	actionRegistry *Registry
	runtimeHooks   *runtimeHooks
}

type runtimeHooks struct {
	refreshCommands func(context.Context, string, map[string]any)
}

func New(deps Deps) *Service {
	hooks := &runtimeHooks{
		refreshCommands: deps.RefreshCommands,
	}
	service := &Service{
		runtimeHooks: hooks,
	}
	if deps.ActionRegistry != nil {
		service.actionRegistry = deps.ActionRegistry
	} else {
		if len(deps.Registrars) == 0 {
			deps.Registrars = DefaultRegistrars()
		}
		deps.RefreshCommands = func(ctx context.Context, pluginID string, settings map[string]any) {
			if hooks.refreshCommands != nil {
				hooks.refreshCommands(ctx, pluginID, settings)
			}
		}
		service.actionRegistry = NewRegistryWithRegistrars(deps, deps.Registrars...)
	}
	return service
}

func (s *Service) SetRefreshPluginCommands(refresh func(context.Context, string, map[string]any)) {
	if s.runtimeHooks == nil {
		s.runtimeHooks = &runtimeHooks{}
	}
	s.runtimeHooks.refreshCommands = refresh
}
